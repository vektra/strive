package strive

import (
	"fmt"
	"time"
)

type TaskResources map[string]int

type PortBinding struct {
	HostPort      int
	ContainerPort int
	Protocol      string
}

func (pb PortBinding) DockerContainerPort() string {
	return fmt.Sprintf("%d/%s", pb.ContainerPort, pb.Protocol)
}

func (pb PortBinding) DockerHostPort() string {
	return fmt.Sprintf("%d", pb.HostPort)
}

type ContainerDetails struct {
	Image string
	Ports []PortBinding
}

type TaskDescription struct {
	Command  string
	Exec     []string
	Env      map[string]string
	URLs     []string
	Config   map[string]interface{}
	MetaData map[string]interface{}
	Labels   []string

	Container *ContainerDetails
}

type Task struct {
	Id          string
	Host        string
	Scheduler   string
	Resources   map[string]TaskResources
	Description *TaskDescription

	Status     string
	LastUpdate time.Time
}

func (t *Task) ConfigGet(key string) (interface{}, bool) {
	if t.Description == nil || t.Description.Config == nil {
		return nil, false
	}

	val, ok := t.Description.Config[key]
	return val, ok
}

func (t *Task) ConfigBoolGet(key string) (bool, bool) {
	val, ok := t.ConfigGet(key)
	if !ok {
		return false, false
	}

	if bval, ok := val.(bool); ok {
		return bval, true
	}

	return false, false
}
