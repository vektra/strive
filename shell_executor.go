package strive

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

type ShellExecutor struct {
	Logger    Logger
	WorkSetup WorkSetup
}

type shellHandle struct {
	*exec.Cmd

	log io.WriteCloser
}

func (sh *shellHandle) Stop(force bool) error {
	return sh.Process.Kill()
}

func (sh *shellHandle) Wait() error {
	err := sh.Cmd.Wait()
	sh.log.Close()
	return err
}

func (se *ShellExecutor) Run(task *Task) (TaskHandle, error) {
	out, err := se.Logger.SetupStream("output", task)
	if err != nil {
		return nil, err
	}

	errstr, err := se.Logger.SetupStream("error", task)
	if err != nil {
		return nil, err
	}

	dir, err := se.WorkSetup.TaskDir(task)
	if err != nil {
		return nil, err
	}

	for _, url := range task.Description.URLs {
		err = se.WorkSetup.DownloadURL(url, dir)
		if err != nil {
			return nil, err
		}
	}

	f, err := os.Create(filepath.Join(dir, "metadata.json"))
	if err != nil {
		return nil, err
	}

	if task.Description.MetaData == nil {
		f.Write([]byte("{}\n"))
	} else {
		err = json.NewEncoder(f).Encode(task.Description.MetaData)
	}

	f.Close()

	if err != nil {
		return nil, err
	}

	var cmd *exec.Cmd

	if len(task.Description.Exec) > 0 {
		args := task.Description.Exec

		cmd = exec.Command(args[0], args[1:]...)
	} else {
		cmd = exec.Command("bash", "-c", task.Description.Command)
	}

	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = errstr

	if injShell, ok := se.Logger.(ShellLogInjector); ok {
		err = injShell.InjectLog(cmd, task)
		if err != nil {
			return nil, err
		}
	}

	env := []string{"STRIVE_TASKID=" + task.Id}

	for k, v := range task.Description.Env {
		env = append(env, k+"="+v)
	}

	cmd.Env = env

	cmd.Start()

	return &shellHandle{cmd, out}, nil
}
