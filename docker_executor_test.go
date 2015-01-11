package strive

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	backend "github.com/vektra/go-dockerclient"
	"github.com/vektra/neko"
)

func TestDockerExecutor(t *testing.T) {
	n := neko.Start(t)

	var mdc MockDockerBackend
	var mws MockWorkSetup

	n.CheckMock(&mdc.Mock)
	n.CheckMock(&mws.Mock)

	task := &Task{
		Id: "task1",
		Description: &TaskDescription{
			Command: "echo 'hello'",
			Container: &ContainerDetails{
				Image: "ubuntu",
			},
		},
	}

	img := &backend.Image{
		ID: "ubuntu",
	}

	var ml *MemLogger
	var de *DockerExecutor

	n.Setup(func() {
		ml = &MemLogger{}

		de = &DockerExecutor{
			Logger:    ml,
			Client:    &mdc,
			WorkSetup: &mws,
		}
	})

	n.It("runs a command", func() {
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

		attach := backend.AttachToContainerOptions{
			Container:    cont.ID,
			OutputStream: &ml.Buffer,
			ErrorStream:  &ml.Buffer,

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

	n.It("can stop container nicely and not nicely", func() {
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

		attach := backend.AttachToContainerOptions{
			Container:    cont.ID,
			OutputStream: &ml.Buffer,
			ErrorStream:  &ml.Buffer,

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

		mdc.On("StopContainer", cont.ID, uint(600)).Return(nil)
		th.Stop(false)

		mdc.On("StopContainer", cont.ID, uint(5)).Return(nil)
		th.Stop(true)

		mdc.On("WaitContainer", cont.ID).Return(0, nil)
		mdc.On("RemoveContainer", backend.RemoveContainerOptions{ID: cont.ID}).Return(nil)

		err = th.Wait()
		require.NoError(t, err)
	})

	n.It("pulls an image if it's not available", func() {
		mws.On("TaskDir", task).Return("/tmp", nil)

		mdc.On("InspectImage", "ubuntu").Return((*backend.Image)(nil), backend.ErrNoSuchImage)

		pio := backend.PullImageOptions{
			Repository:   "ubuntu",
			OutputStream: &ml.Buffer,
		}

		mdc.On("PullImage", pio, backend.AuthConfiguration{}).Return(nil)

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

		attach := backend.AttachToContainerOptions{
			Container:    cont.ID,
			OutputStream: &ml.Buffer,
			ErrorStream:  &ml.Buffer,

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

	n.It("writes metadata to the work dir", func() {
		task := &Task{
			Id: "task1",
			Description: &TaskDescription{
				Command: "echo 'hello'",
				Container: &ContainerDetails{
					Image: "ubuntu",
				},
				MetaData: map[string]interface{}{
					"stuff": "is cool",
				},
			},
		}

		tmpDir, err := ioutil.TempDir("", "test")
		require.NoError(t, err)

		defer os.RemoveAll(tmpDir)

		mws.On("TaskDir", task).Return(tmpDir, nil)

		mdc.On("InspectImage", "ubuntu").Return((*backend.Image)(nil), backend.ErrNoSuchImage)

		pio := backend.PullImageOptions{
			Repository:   "ubuntu",
			OutputStream: &ml.Buffer,
		}

		mdc.On("PullImage", pio, backend.AuthConfiguration{}).Return(nil)

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

		attach := backend.AttachToContainerOptions{
			Container:    cont.ID,
			OutputStream: &ml.Buffer,
			ErrorStream:  &ml.Buffer,

			Stream: true,
			Stdout: true,
			Stderr: true,
		}

		mdc.On("AttachToContainer", attach).Return(nil)

		hostCfg := &backend.HostConfig{
			Binds: []string{tmpDir + ":/tmp/strive-sandbox"},
		}

		mdc.On("StartContainer", cont.ID, hostCfg).Return(nil)

		th, err := de.Run(task)
		require.NoError(t, err)

		mdc.On("WaitContainer", cont.ID).Return(0, nil)
		mdc.On("RemoveContainer", backend.RemoveContainerOptions{ID: cont.ID}).Return(nil)

		err = th.Wait()
		require.NoError(t, err)

		data, err := ioutil.ReadFile(filepath.Join(tmpDir, "metadata.json"))
		require.NoError(t, err)

		exp := `{"stuff":"is cool"}` + "\n"
		assert.Equal(t, exp, string(data))
	})

	n.It("writes valid json even if metadata is empty", func() {
		task := &Task{
			Id: "task1",
			Description: &TaskDescription{
				Command: "echo 'hello'",
				Container: &ContainerDetails{
					Image: "ubuntu",
				},
			},
		}

		tmpDir, err := ioutil.TempDir("", "test")
		require.NoError(t, err)

		defer os.RemoveAll(tmpDir)

		mws.On("TaskDir", task).Return(tmpDir, nil)

		mdc.On("InspectImage", "ubuntu").Return((*backend.Image)(nil), backend.ErrNoSuchImage)

		pio := backend.PullImageOptions{
			Repository:   "ubuntu",
			OutputStream: &ml.Buffer,
		}

		mdc.On("PullImage", pio, backend.AuthConfiguration{}).Return(nil)

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

		attach := backend.AttachToContainerOptions{
			Container:    cont.ID,
			OutputStream: &ml.Buffer,
			ErrorStream:  &ml.Buffer,

			Stream: true,
			Stdout: true,
			Stderr: true,
		}

		mdc.On("AttachToContainer", attach).Return(nil)

		hostCfg := &backend.HostConfig{
			Binds: []string{tmpDir + ":/tmp/strive-sandbox"},
		}

		mdc.On("StartContainer", cont.ID, hostCfg).Return(nil)

		th, err := de.Run(task)
		require.NoError(t, err)

		mdc.On("WaitContainer", cont.ID).Return(0, nil)
		mdc.On("RemoveContainer", backend.RemoveContainerOptions{ID: cont.ID}).Return(nil)

		err = th.Wait()
		require.NoError(t, err)

		data, err := ioutil.ReadFile(filepath.Join(tmpDir, "metadata.json"))
		require.NoError(t, err)

		exp := `{}` + "\n"
		assert.Equal(t, exp, string(data))
	})

	n.It("uses a requested port", func() {
		task := &Task{
			Id: "task1",
			Description: &TaskDescription{
				Command: "echo 'hello'",
				Container: &ContainerDetails{
					Image: "ubuntu",
					Ports: []PortBinding{
						{
							HostPort:      32001,
							ContainerPort: 32001,
							Protocol:      "tcp",
						},
					},
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
				Env:          []string{"STRIVE_TASKID=" + task.Id, "PORT=32001"},
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

		attach := backend.AttachToContainerOptions{
			Container:    cont.ID,
			OutputStream: &ml.Buffer,
			ErrorStream:  &ml.Buffer,

			Stream: true,
			Stdout: true,
			Stderr: true,
		}

		mdc.On("AttachToContainer", attach).Return(nil)

		hostCfg := &backend.HostConfig{
			Binds: []string{"/tmp:/tmp/strive-sandbox"},
			PortBindings: map[backend.Port][]backend.PortBinding{
				backend.Port("32001/tcp"): {{HostPort: "32001"}},
			},
		}

		mdc.On("StartContainer", cont.ID, hostCfg).Return(nil)

		th, err := de.Run(task)
		require.NoError(t, err)

		mdc.On("WaitContainer", cont.ID).Return(0, nil)
		mdc.On("RemoveContainer", backend.RemoveContainerOptions{ID: cont.ID}).Return(nil)

		err = th.Wait()
		require.NoError(t, err)
	})

	n.It("cleans up a container when start fails", func() {
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

		hostCfg := &backend.HostConfig{
			Binds: []string{"/tmp:/tmp/strive-sandbox"},
		}

		mdc.On("StartContainer", cont.ID, hostCfg).Return(io.EOF)

		mdc.On("RemoveContainer", backend.RemoveContainerOptions{ID: cont.ID}).Return(nil)

		th, err := de.Run(task)
		assert.Equal(t, err, io.EOF)

		assert.Nil(t, th)
	})

	n.It("it downloads any urls into the work dir", func() {
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

		attach := backend.AttachToContainerOptions{
			Container:    cont.ID,
			OutputStream: &ml.Buffer,
			ErrorStream:  &ml.Buffer,

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

	n.It("can stop container nicely and not nicely", func() {
		task := &Task{
			Id: "task1",
			Description: &TaskDescription{
				Command: "echo 'hello'",
				Container: &ContainerDetails{
					Image: "ubuntu",
				},
				URLs: []string{"http://test.this/foo"},
			},
		}

		mws.On("TaskDir", task).Return("/tmp", nil)
		mws.On("DownloadURL", "http://test.this/foo", "/tmp").Return(nil)

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

		attach := backend.AttachToContainerOptions{
			Container:    cont.ID,
			OutputStream: &ml.Buffer,
			ErrorStream:  &ml.Buffer,

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

		mdc.On("StopContainer", cont.ID, uint(600)).Return(nil)
		th.Stop(false)

		mdc.On("StopContainer", cont.ID, uint(5)).Return(nil)
		th.Stop(true)

		mdc.On("WaitContainer", cont.ID).Return(0, nil)
		mdc.On("RemoveContainer", backend.RemoveContainerOptions{ID: cont.ID}).Return(nil)

		err = th.Wait()
		require.NoError(t, err)
	})

	n.Meow()
}
