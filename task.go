package strive

import "fmt"

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
	MetaData map[string]interface{}

	Container *ContainerDetails
}

type Task struct {
	Id          string
	Host        string
	Scheduler   string
	Status      string
	Resources   map[string]TaskResources
	Description *TaskDescription
}
