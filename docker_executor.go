package strive

import (
	"encoding/json"
	"expvar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	backend "github.com/vektra/go-dockerclient"
)

var (
	// Counters
	haDockerStarts  = expvar.NewInt("agent.docker.runs")
	haDockerStops   = expvar.NewInt("agent.docker.stops")
	haDockerCreated = expvar.NewInt("agent.docker.created")
	haDockerPulls   = expvar.NewInt("agent.docker.pulls")

	// Gauges
	haDockerMonitors = expvar.NewInt("agent.docker.monitors")
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

func (dh *dockerHandle) startLogs(out, errstr io.Writer) {
	dh.watch.Add(1)

	dh.done = make(chan error)

	go func() {
		attach := backend.AttachToContainerOptions{
			Container:    dh.cont.ID,
			OutputStream: out,
			ErrorStream:  errstr,
			Stream:       true,
			Stdout:       true,
			Stderr:       true,
		}

		haDockerMonitors.Add(1)

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

	haDockerMonitors.Add(-1)

	return err
}

func (dh *dockerHandle) Stop(force bool) error {
	haDockerStops.Add(1)

	var timeout uint

	if force {
		timeout = 5
	} else {
		timeout = 600
	}

	return dh.de.Client.StopContainer(dh.cont.ID, timeout)
}

func (de *DockerExecutor) Run(task *Task) (TaskHandle, error) {
	haDockerStarts.Add(1)

	out, err := de.Logger.SetupStream("output", task)
	if err != nil {
		return nil, err
	}

	errstr, err := de.Logger.SetupStream("error", task)
	if err != nil {
		out.Close()
		return nil, err
	}

	defer func() {
		if err != nil {
			out.Close()
			errstr.Close()
		}
	}()

	tmpDir, err := de.WorkSetup.TaskDir(task)
	if err != nil {
		return nil, err
	}

	for _, url := range task.Description.Urls {
		err = de.WorkSetup.DownloadURL(url, tmpDir)
		if err != nil {
			return nil, err
		}
	}

	f, err := os.Create(filepath.Join(tmpDir, "metadata.json"))
	if err != nil {
		return nil, err
	}

	if task.Description.Metadata == nil {
		f.Write([]byte("{}\n"))
	} else {
		err = json.NewEncoder(f).Encode(JSONVariables(task.Description.Metadata))
	}

	f.Close()

	if err != nil {
		return nil, err
	}

	imgName := task.Description.Container.GetImage()

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

		haDockerPulls.Add(1)

		err = de.Client.PullImage(pio, backend.AuthConfiguration{})

		de.pullLock.Unlock()

		if err != nil {
			return nil, err
		}
	}

	env := []string{"STRIVE_TASKID=" + task.GetTaskId()}

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

			env = append(env, fmt.Sprintf("PORT=%d", port.GetContainer()))
		}

		hostCfg.PortBindings = pb
	}

	cco := backend.CreateContainerOptions{
		Name: "strive-" + task.GetTaskId(),
		Config: &backend.Config{
			Image:        imgName,
			Hostname:     "strive-" + task.GetTaskId(),
			Env:          env,
			WorkingDir:   "/tmp/strive-sandbox",
			AttachStdout: true,
			AttachStderr: true,
		},
	}

	if len(task.Description.Exec) > 0 {
		cco.Config.Cmd = task.Description.Exec
	} else if task.Description.Command != nil {
		cco.Config.Cmd = []string{"/bin/bash", "-c", task.Description.GetCommand()}
	}

	if injDockerLog, ok := de.Logger.(DockerLogInjector); ok {
		err = injDockerLog.InjectDocker(cco.Config, hostCfg, task)
		if err != nil {
			return nil, err
		}
	}

	haDockerCreated.Add(1)

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

	dh.startLogs(out, errstr)

	return dh, nil
}
