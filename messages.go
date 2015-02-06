package strive

import (
	"encoding/json"
	"errors"
)
import "github.com/vektra/vega"

type UpdateState struct {
	AddHosts []*Host
	AddTasks []*Task
}

type StartTask struct {
	OpId string
	Task *Task
}

type StopTask struct {
	OpId  string
	Id    string
	Force bool
}

type StopError struct {
	OpId  string
	Id    string
	Error string
}

type OpAcknowledged struct {
	OpId string
}

type TaskStatusChange struct {
	Id     string
	Status string
	Error  string
}

type HostStatusChange struct {
	Host   string
	Status string
}

type ListTasks struct {
	OpId string
}

type CurrentTasks struct {
	OpId  string
	Tasks []string
}

type GenericError struct {
	Error string
}

type CheckTasks struct {
	Tasks []string
}

type CheckedTaskList struct {
	Host    string
	Missing []string
	Unknown []string
}

var ErrUnknownMessage = errors.New("unknown message")

func decodeMessage(vm *vega.Message) (interface{}, error) {
	switch vm.Type {
	case "UpdateState":
		var us UpdateState

		err := json.Unmarshal(vm.Body, &us)
		if err != nil {
			return nil, err
		}

		return &us, nil
	case "TaskStatusChange":
		var ts TaskStatusChange

		err := json.Unmarshal(vm.Body, &ts)
		if err != nil {
			return nil, err
		}

		return &ts, nil
	case "HostStatusChange":
		var hs HostStatusChange

		err := json.Unmarshal(vm.Body, &hs)
		if err != nil {
			return nil, err
		}

		return &hs, nil
	case "StartTask":
		var st StartTask

		err := json.Unmarshal(vm.Body, &st)
		if err != nil {
			return nil, err
		}

		return &st, nil
	case "StopTask":
		var st StopTask

		err := json.Unmarshal(vm.Body, &st)
		if err != nil {
			return nil, err
		}

		return &st, nil
	case "ListTasks":
		var lt ListTasks

		err := json.Unmarshal(vm.Body, &lt)
		if err != nil {
			return nil, err
		}

		return &lt, nil
	case "CheckTasks":
		var ct CheckTasks

		err := json.Unmarshal(vm.Body, &ct)
		if err != nil {
			return nil, err
		}

		return &ct, nil
	case "CheckedTaskList":
		var ct CheckedTaskList

		err := json.Unmarshal(vm.Body, &ct)
		if err != nil {
			return nil, err
		}

		return &ct, nil
	default:
		return nil, ErrUnknownMessage
	}
}
