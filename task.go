package strive

type TaskResources map[string]int

type ContainerDetails struct {
	Image string
}

type TaskDescription struct {
	Command  string
	Env      map[string]string
	URLs     []string
	MetaData map[string]interface{}

	Container *ContainerDetails
}

type Task struct {
	Id          string
	Resources   map[string]TaskResources
	Description *TaskDescription
}
