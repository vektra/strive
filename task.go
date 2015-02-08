package strive

import "fmt"

func (pb *PortBinding) DockerContainerPort() string {
	var proto string

	switch pb.GetProtocol() {
	case PortBinding_TCP:
		proto = "tcp"
	case PortBinding_UDP:
		proto = "udp"
	default:
		panic("unknown port binding protocol")
	}

	return fmt.Sprintf("%d/%s", *pb.Container, proto)
}

func (pb *PortBinding) DockerHostPort() string {
	return fmt.Sprintf("%d", *pb.Host)
}

func (t *Task) ConfigGet(key string) (*Value, bool) {
	if t.Description == nil || t.Description.Config == nil {
		return nil, false
	}

	for _, val := range t.Description.Config {
		if val.GetName() == key {
			return val.Value, true
		}
	}

	return nil, false
}

func (t *Task) ConfigBoolGet(key string) (bool, bool) {
	val, ok := t.ConfigGet(key)
	if !ok {
		return false, false
	}

	if val.GetValueType() == Value_BOOL {
		return val.GetBoolVal(), true
	}

	return false, false
}

func (t *Task) Healthy() bool {
	switch t.GetStatus() {
	case TaskStatus_CREATED, TaskStatus_RUNNING:
		return true
	}

	return false
}
