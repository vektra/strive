package strive

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"

	backend "github.com/vektra/go-dockerclient"
)

type DockerExecutor struct {
	Logger    Logger
	Client    DockerBackend
	WorkSetup WorkSetup
}

type dockerHandle struct {
	de    *DockerExecutor
	cont  *backend.Container
	watch sync.WaitGroup

	done chan error
}

func (dh *dockerHandle) startLogs(out io.Writer) {
	dh.watch.Add(1)

	dh.done = make(chan error)

	go func() {
		attach := backend.AttachToContainerOptions{
			Container:    dh.cont.ID,
			OutputStream: out,
			ErrorStream:  out,
			Stream:       true,
			Stdout:       true,
			Stderr:       true,
		}

		dh.done <- dh.de.Client.AttachToContainer(attach)
	}()
}

func (dh *dockerHandle) Wait() error {
	_, err := dh.de.Client.WaitContainer(dh.cont.ID)
	if err != nil {
		return err
	}

	return <-dh.done
}

func (dh *dockerHandle) Stop(force bool) error {
	var timeout uint

	if force {
		timeout = 5
	} else {
		timeout = 600
	}

	return dh.de.Client.StopContainer(dh.cont.ID, timeout)
}

func (de *DockerExecutor) Run(task *Task) (TaskHandle, error) {
	out, err := de.Logger.SetupStream(task)
	if err != nil {
		return nil, err
	}

	tmpDir, err := de.WorkSetup.TaskDir(task)
	if err != nil {
		return nil, err
	}

	f, err := os.Create(filepath.Join(tmpDir, "metadata.json"))
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

	imgName := task.Description.Container.Image

	_, err = de.Client.InspectImage(imgName)
	if err != nil {
		if err != backend.ErrNoSuchImage {
			return nil, err
		}

		pio := backend.PullImageOptions{
			Repository:   imgName,
			OutputStream: out,
		}

		err = de.Client.PullImage(pio, backend.AuthConfiguration{})
		if err != nil {
			return nil, err
		}
	}

	cco := backend.CreateContainerOptions{
		Name: "strive-" + task.Id,
		Config: &backend.Config{
			Image:        imgName,
			Hostname:     "strive-" + task.Id,
			Cmd:          []string{"/bin/bash", "-c", task.Description.Command},
			Env:          []string{"TASKID=" + task.Id},
			WorkingDir:   "/tmp/strive-sandbox",
			AttachStdout: true,
			AttachStderr: true,
		},
	}

	cont, err := de.Client.CreateContainer(cco)
	if err != nil {
		return nil, err
	}

	dh := &dockerHandle{de: de, cont: cont}

	dh.startLogs(out)

	hostCfg := &backend.HostConfig{
		Binds: []string{tmpDir + ":/tmp/strive-sandbox"},
	}

	err = de.Client.StartContainer(cont.ID, hostCfg)
	if err != nil {
		return nil, err
	}

	return dh, nil
}
