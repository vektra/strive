package strive

import (
	"encoding/json"
	"fmt"
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

	pullLock sync.Mutex
}

type dockerHandle struct {
	de   *DockerExecutor
	cont *backend.Container

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

	err = <-dh.done

	dh.de.Client.RemoveContainer(backend.RemoveContainerOptions{ID: dh.cont.ID})

	return err
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

	defer func() {
		if err != nil {
			out.Close()
		}
	}()

	tmpDir, err := de.WorkSetup.TaskDir(task)
	if err != nil {
		return nil, err
	}

	for _, url := range task.Description.URLs {
		err = de.WorkSetup.DownloadURL(url, tmpDir)
		if err != nil {
			return nil, err
		}
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

		// There are known issues with doing concurrent pulls
		// against docker. And because we might be spinning up
		// multiple tasks at once, we simple serialize any pulling
		// to keep things sane.

		de.pullLock.Lock()

		pio := backend.PullImageOptions{
			Repository:   imgName,
			OutputStream: out,
		}

		err = de.Client.PullImage(pio, backend.AuthConfiguration{})

		de.pullLock.Unlock()

		if err != nil {
			return nil, err
		}
	}

	env := []string{"STRIVE_TASKID=" + task.Id}

	hostCfg := &backend.HostConfig{
		Binds: []string{tmpDir + ":/tmp/strive-sandbox"},
	}

	ports := task.Description.Container.Ports

	if len(ports) > 0 {
		pb := make(map[backend.Port][]backend.PortBinding)

		for _, port := range ports {
			key := backend.Port(port.DockerContainerPort())
			pb[key] = append(
				pb[key],
				backend.PortBinding{HostPort: port.DockerHostPort()},
			)

			env = append(env, fmt.Sprintf("PORT=%d", port.ContainerPort))
		}

		hostCfg.PortBindings = pb
	}

	cco := backend.CreateContainerOptions{
		Name: "strive-" + task.Id,
		Config: &backend.Config{
			Image:        imgName,
			Hostname:     "strive-" + task.Id,
			Env:          env,
			WorkingDir:   "/tmp/strive-sandbox",
			AttachStdout: true,
			AttachStderr: true,
		},
	}

	if len(task.Description.Exec) > 0 {
		cco.Config.Cmd = task.Description.Exec
	} else if task.Description.Command != "" {
		cco.Config.Cmd = []string{"/bin/bash", "-c", task.Description.Command}
	}

	cont, err := de.Client.CreateContainer(cco)
	if err != nil {
		return nil, err
	}

	dh := &dockerHandle{de: de, cont: cont}

	err = de.Client.StartContainer(cont.ID, hostCfg)
	if err != nil {
		de.Client.RemoveContainer(backend.RemoveContainerOptions{ID: cont.ID})
		return nil, err
	}

	dh.startLogs(out)

	return dh, nil
}
