package strive

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"unicode"

	backend "github.com/vektra/go-dockerclient"
	"github.com/vektra/log"
)

type VKLogger struct {
	logger  log.Logger
	logpath string
}

var _ = Logger(&VKLogger{})

func NewVKLogger() *VKLogger {
	return &VKLogger{
		logger:  log.Connect(),
		logpath: log.LogPath(),
	}
}

type vkStream struct {
	logger log.Logger
	name   string
	taskId string

	buf bytes.Buffer
}

var _ = io.WriteCloser(&vkStream{})

var NL = []byte{'\n'}

func (s *vkStream) Write(line []byte) (int, error) {
	total := len(line)

	for {
		idx := bytes.Index(line, NL)

		if idx == -1 {
			s.buf.Write(line)
			return total, nil
		}

		portion := string(line[0 : idx+1])

		var full string

		if s.buf.Len() == 0 {
			full = portion
		} else {
			full = s.buf.String() + portion
			s.buf.Reset()
		}

		full = strings.TrimRightFunc(full, unicode.IsSpace)

		if len(full) > 0 {
			m := log.Log()
			m.Add("task", s.taskId)
			m.Add(s.name, full)

			err := s.logger.Write(m)
			if err != nil {
				return 0, err
			}
		}

		line = line[idx+1:]
	}

	return len(line), nil
}

func (s *vkStream) Close() error {
	return nil
}

func (v *VKLogger) SetupStream(name string, task *Task) (io.WriteCloser, error) {
	return &vkStream{logger: v.logger, name: name, taskId: task.Id}, nil
}

type structuredStream struct {
	name   string
	read   *os.File
	logger log.Logger
}

func (ss *structuredStream) process() {
	kvStream := &log.KVStream{
		Src:    ss.read,
		Out:    ss,
		Bare:   false,
		Source: ss.name,
	}

	fmt.Println("processing..")

	kvStream.Parse()
}

func (ss *structuredStream) Read(m *log.Message) error {
	return ss.logger.Write(m)
}

func (v *VKLogger) InjectLog(cmd *exec.Cmd, task *Task) error {
	r, w, err := os.Pipe()
	if err != nil {
		return err
	}

	ss := &structuredStream{"logs", r, v.logger}
	go ss.process()

	cmd.ExtraFiles = []*os.File{w}
	cmd.Env = append(cmd.Env, "LOG_FD=3")

	return nil
}

const cInContainerPath = "/tmp/vklog.sock"

func (v *VKLogger) InjectDocker(
	cfg *backend.Config,
	hc *backend.HostConfig,
	task *Task,
) error {
	cfg.Env = append(cfg.Env, "LOG_PATH="+cInContainerPath)
	hc.Binds = append(hc.Binds, v.logpath+":"+cInContainerPath)
	return nil
}
