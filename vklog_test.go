package strive

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	backend "github.com/vektra/go-dockerclient"
	"github.com/vektra/log"
	"github.com/vektra/neko"
)

type vkLogSink struct {
	mu       sync.Mutex
	Messages []*log.Message
}

func (v *vkLogSink) Write(m *log.Message) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.Messages = append(v.Messages, m)

	return nil
}

func (v *vkLogSink) Close() error {
	return nil
}

func TestVKLogger(t *testing.T) {
	n := neko.Start(t)

	var vk *VKLogger
	var sink *vkLogSink

	var mdc MockDockerBackend
	var mws MockWorkSetup

	n.CheckMock(&mws.Mock)
	n.CheckMock(&mdc.Mock)

	img := &backend.Image{
		ID: "ubuntu",
	}

	task := &Task{
		Id: "task1",
		Description: &TaskDescription{
			Command: "echo 'hello'",
		},
	}

	n.Setup(func() {
		sink = &vkLogSink{}
		vk = &VKLogger{
			logger: sink,
		}
	})

	n.It("generates messages from the input stream", func() {
		out, err := vk.SetupStream("output", task)
		require.NoError(t, err)

		line := []byte("this is a line\n")

		n, err := out.Write(line)
		require.NoError(t, err)

		assert.Equal(t, len(line), n)

		tid, ok := sink.Messages[0].GetString("task")
		require.True(t, ok)

		assert.Equal(t, "task1", tid)

		msg, ok := sink.Messages[0].GetString("output")
		require.True(t, ok)

		assert.Equal(t, "this is a line", msg)
	})

	n.It("buffers and reads up to a line", func() {
		out, err := vk.SetupStream("output", task)
		require.NoError(t, err)

		_, err = out.Write([]byte("this "))
		require.NoError(t, err)

		line := []byte("is a line\n")

		n, err := out.Write(line)
		require.NoError(t, err)

		assert.Equal(t, len(line), n)

		tid, ok := sink.Messages[0].GetString("task")
		require.True(t, ok)

		assert.Equal(t, "task1", tid)

		msg, ok := sink.Messages[0].GetString("output")
		require.True(t, ok)

		assert.Equal(t, "this is a line", msg)
	})

	n.It("injects a stream to manage native messages", func() {
		task := &Task{
			Description: &TaskDescription{
				Command: `echo '> greeting="hello"' > /dev/fd/3`,
			},
		}

		se := &ShellExecutor{
			Logger:    vk,
			WorkSetup: &mws,
		}

		mws.On("TaskDir", task).Return("/tmp", nil)

		th, err := se.Run(task)
		require.NoError(t, err)

		err = th.Wait()
		require.NoError(t, err)

		greeting, ok := sink.Messages[0].GetString("greeting")
		require.True(t, ok)

		assert.Equal(t, greeting, "hello")
	})

	n.It("injects a vklog socket to a docker container", func() {
		de := &DockerExecutor{
			Logger:    vk,
			Client:    &mdc,
			WorkSetup: &mws,
		}

		task := &Task{
			Id: "task1",
			Description: &TaskDescription{
				Command: "echo 'hello'",
				Container: &ContainerDetails{
					Image: "ubuntu",
				},
			},
		}

		mws.On("TaskDir", task).Return("/tmp", nil)

		mdc.On("InspectImage", "ubuntu").Return(img, nil)

		cco := backend.CreateContainerOptions{
			Name: "strive-task1",
			Config: &backend.Config{
				Image:        "ubuntu",
				Hostname:     "strive-task1",
				Cmd:          []string{"/bin/bash", "-c", "echo 'hello'"},
				Env:          []string{"STRIVE_TASKID=" + task.Id, "LOG_PATH=/tmp/vklog.sock"},
				WorkingDir:   "/tmp/strive-sandbox",
				AttachStdout: true,
				AttachStderr: true,
			},
		}

		cont := &backend.Container{
			ID:    "xxyyzz",
			Image: "ubuntu",
		}

		mdc.On("CreateContainer", cco).Return(cont, nil)

		out, err := vk.SetupStream("output", task)
		require.NoError(t, err)

		errstr, err := vk.SetupStream("error", task)
		require.NoError(t, err)

		attach := backend.AttachToContainerOptions{
			Container:    cont.ID,
			OutputStream: out,
			ErrorStream:  errstr,

			Stream: true,
			Stdout: true,
			Stderr: true,
		}

		mdc.On("AttachToContainer", attach).Return(nil)

		hostCfg := &backend.HostConfig{
			Binds: []string{"/tmp:/tmp/strive-sandbox", vk.logpath + ":/tmp/vklog.sock"},
		}

		mdc.On("StartContainer", cont.ID, hostCfg).Return(nil)

		th, err := de.Run(task)
		require.NoError(t, err)

		mdc.On("WaitContainer", cont.ID).Return(0, nil)
		mdc.On("RemoveContainer", backend.RemoveContainerOptions{ID: cont.ID}).Return(nil)

		err = th.Wait()
		require.NoError(t, err)
	})

	n.It("does not inject a vklog socket if config says not to", func() {
		de := &DockerExecutor{
			Logger:    vk,
			Client:    &mdc,
			WorkSetup: &mws,
		}

		task := &Task{
			Id: "task1",
			Description: &TaskDescription{
				Command: "echo 'hello'",
				Container: &ContainerDetails{
					Image: "ubuntu",
				},
				Config: map[string]interface{}{
					"vklog.disable": true,
				},
			},
		}

		mws.On("TaskDir", task).Return("/tmp", nil)

		mdc.On("InspectImage", "ubuntu").Return(img, nil)

		cco := backend.CreateContainerOptions{
			Name: "strive-task1",
			Config: &backend.Config{
				Image:        "ubuntu",
				Hostname:     "strive-task1",
				Cmd:          []string{"/bin/bash", "-c", "echo 'hello'"},
				Env:          []string{"STRIVE_TASKID=" + task.Id},
				WorkingDir:   "/tmp/strive-sandbox",
				AttachStdout: true,
				AttachStderr: true,
			},
		}

		cont := &backend.Container{
			ID:    "xxyyzz",
			Image: "ubuntu",
		}

		mdc.On("CreateContainer", cco).Return(cont, nil)

		out, err := vk.SetupStream("output", task)
		require.NoError(t, err)

		errstr, err := vk.SetupStream("error", task)
		require.NoError(t, err)

		attach := backend.AttachToContainerOptions{
			Container:    cont.ID,
			OutputStream: out,
			ErrorStream:  errstr,

			Stream: true,
			Stdout: true,
			Stderr: true,
		}

		mdc.On("AttachToContainer", attach).Return(nil)

		hostCfg := &backend.HostConfig{
			Binds: []string{"/tmp:/tmp/strive-sandbox"},
		}

		mdc.On("StartContainer", cont.ID, hostCfg).Return(nil)

		th, err := de.Run(task)
		require.NoError(t, err)

		mdc.On("WaitContainer", cont.ID).Return(0, nil)
		mdc.On("RemoveContainer", backend.RemoveContainerOptions{ID: cont.ID}).Return(nil)

		err = th.Wait()
		require.NoError(t, err)
	})

	n.Meow()
}
