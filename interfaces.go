package strive

import (
	"io"

	backend "github.com/vektra/go-dockerclient"
	"github.com/vektra/vega"
)

type TaskHandle interface {
	Wait() error
	Stop(force bool) error
}

type TaskExecutor interface {
	Run(task *Task) (TaskHandle, error)
}

type MessageBus interface {
	SendMessage(who string, vm *vega.Message) error
}

type MessageHandler interface {
	HandleMessage(vm *vega.Message) error
}

type MessageBusReceiver interface {
	Receive(hnd MessageHandler) error
}

type Logger interface {
	SetupStream(task *Task) (io.WriteCloser, error)
}

type WorkSetup interface {
	TaskDir(task *Task) (string, error)
	DownloadURL(url string, dir string) error
}

type DockerBackend interface {
	CreateContainer(backend.CreateContainerOptions) (*backend.Container, error)
	StartContainer(id string, host *backend.HostConfig) error
	StopContainer(id string, timeout uint) error
	RestartContainer(id string, timeout uint) error
	InspectContainer(id string) (*backend.Container, error)
	RemoveContainer(opts backend.RemoveContainerOptions) error
	AttachToContainer(opts backend.AttachToContainerOptions) error
	BuildImage(opts backend.BuildImageOptions) error
	CopyFromContainer(opts backend.CopyFromContainerOptions) error
	InspectImage(name string) (*backend.Image, error)
	WaitContainer(id string) (int, error)
	PullImage(opts backend.PullImageOptions, auth backend.AuthConfiguration) error
}
