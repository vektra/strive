package strive

import (
	"encoding/json"
	"expvar"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

var (
	// Counters
	haShellRuns  = expvar.NewInt("agent.shell.runs")
	haShellStops = expvar.NewInt("agent.shell.stops")

	// Gauges
	haShellMonitors = expvar.NewInt("agent.shell.monitors")
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
	haShellStops.Add(1)
	return sh.Process.Kill()
}

func (sh *shellHandle) Wait() error {
	err := sh.Cmd.Wait()
	haShellMonitors.Add(-1)
	sh.log.Close()
	return err
}

func (se *ShellExecutor) Run(task *Task) (TaskHandle, error) {
	haShellRuns.Add(1)

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

	for _, url := range task.Description.Urls {
		err = se.WorkSetup.DownloadURL(url, dir)
		if err != nil {
			return nil, err
		}
	}

	f, err := os.Create(filepath.Join(dir, "metadata.json"))
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

	var cmd *exec.Cmd

	if len(task.Description.Exec) > 0 {
		args := task.Description.Exec

		cmd = exec.Command(args[0], args[1:]...)
	} else {
		cmd = exec.Command("bash", "-c", task.Description.GetCommand())
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

	env := []string{"STRIVE_TASKID=" + task.GetTaskId()}

	for _, v := range task.Description.Env {
		env = append(env, v.StringKV())
	}

	cmd.Env = env

	cmd.Start()

	haShellMonitors.Add(1)

	return &shellHandle{cmd, out}, nil
}
