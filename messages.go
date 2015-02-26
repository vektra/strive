package strive

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/vektra/vega"
)

type Message vega.Message

func (m *Message) GoString() string {
	body, err := decodeMessage(m)
	if err != nil {
		panic(err)
	}

	str := fmt.Sprintf("&Message{Headers:%#v, ContentType:%#v, ContentEncoding:%#v, Priority:%d, CorrelationId:%#v, ReplyTo:%#v, MessageId:%#v, Timestamp:%#v, Type:%#v, UserId:%#v, AppId:%#v, Body:%s}",
		m.Headers, m.ContentType, m.ContentEncoding, m.Priority,
		m.CorrelationId, m.ReplyTo, m.MessageId, m.Timestamp,
		m.Type, m.UserId, m.AppId, body)

	return str
}

func (f FetchState) Encode() *Message {
	bytes, err := f.Marshal()
	if err != nil {
		panic(err)
	}

	return &Message{Type: "FetchState", Body: bytes}
}

func (c ClusterState) Encode() *Message {
	bytes, err := c.Marshal()
	if err != nil {
		panic(err)
	}

	return &Message{Type: "ClusterState", Body: bytes}
}

func (s UpdateState) Encode() *Message {
	bytes, err := s.Marshal()
	if err != nil {
		panic(err)
	}

	return &Message{Type: "UpdateState", Body: bytes}
}

func (s StartTask) Encode() *Message {
	bytes, err := s.Marshal()
	if err != nil {
		panic(err)
	}

	return &Message{Type: "StartTask", Body: bytes}
}

func (s StopTask) Encode() *Message {
	bytes, err := s.Marshal()
	if err != nil {
		panic(err)
	}

	return &Message{Type: "StopTask", Body: bytes}

}

func (t TaskStatusChange) Encode() *Message {
	bytes, err := t.Marshal()
	if err != nil {
		panic(err)
	}

	return &Message{Type: "TaskStatusChange", Body: bytes}
}

func (o OpAcknowledged) Encode() *Message {
	bytes, err := o.Marshal()
	if err != nil {
		panic(err)
	}

	return &Message{Type: "OpAcknowledged", Body: bytes}
}

func (t StopError) Encode() *Message {
	bytes, err := t.Marshal()
	if err != nil {
		panic(err)
	}

	return &Message{Type: "StopError", Body: bytes}
}

func (t ListTasks) Encode() *Message {
	bytes, err := t.Marshal()
	if err != nil {
		panic(err)
	}

	return &Message{Type: "ListTasks", Body: bytes}
}

func (t CurrentTasks) Encode() *Message {
	bytes, err := t.Marshal()
	if err != nil {
		panic(err)
	}

	return &Message{Type: "CurrentTasks", Body: bytes}
}

func (t GenericError) Encode() *Message {
	bytes, err := t.Marshal()
	if err != nil {
		panic(err)
	}

	return &Message{Type: "GenericError", Body: bytes}
}

func (t HostStatusChange) Encode() *Message {
	bytes, err := t.Marshal()
	if err != nil {
		panic(err)
	}

	return &Message{Type: "HostStatusChange", Body: bytes}
}

func (t CheckedTaskList) Encode() *Message {
	bytes, err := t.Marshal()
	if err != nil {
		panic(err)
	}

	return &Message{Type: "CheckedTaskList", Body: bytes}
}

func (t CheckTasks) Encode() *Message {
	bytes, err := t.Marshal()
	if err != nil {
		panic(err)
	}

	return &Message{Type: "CheckTasks", Body: bytes}
}

var ErrUnknownMessage = errors.New("unknown message")

func decodeMessage(vm *Message) (interface{}, error) {
	switch vm.Type {
	case "FetchState":
		var st FetchState

		err := st.Unmarshal(vm.Body)
		if err != nil {
			return nil, err
		}

		return &st, nil
	case "ClusterState":
		var st ClusterState

		err := st.Unmarshal(vm.Body)
		if err != nil {
			return nil, err
		}

		return &st, nil
	case "UpdateState":
		var st UpdateState

		err := st.Unmarshal(vm.Body)
		if err != nil {
			return nil, err
		}

		return &st, nil
	case "StartTask":
		var st StartTask

		err := st.Unmarshal(vm.Body)
		if err != nil {
			return nil, err
		}

		return &st, nil
	case "StopTask":
		var st StopTask

		err := st.Unmarshal(vm.Body)
		if err != nil {
			return nil, err
		}

		return &st, nil
	case "TaskStatusChange":
		var st TaskStatusChange

		err := st.Unmarshal(vm.Body)
		if err != nil {
			return nil, err
		}

		return &st, nil
	case "OpAcknowledged":
		var st OpAcknowledged

		err := st.Unmarshal(vm.Body)
		if err != nil {
			return nil, err
		}

		return &st, nil
	case "StopError":
		var st StopError

		err := st.Unmarshal(vm.Body)
		if err != nil {
			return nil, err
		}

		return &st, nil
	case "ListTasks":
		var st ListTasks

		err := st.Unmarshal(vm.Body)
		if err != nil {
			return nil, err
		}

		return &st, nil
	case "CurrentTasks":
		var st CurrentTasks

		err := st.Unmarshal(vm.Body)
		if err != nil {
			return nil, err
		}

		return &st, nil
	case "GenericError":
		var st GenericError

		err := st.Unmarshal(vm.Body)
		if err != nil {
			return nil, err
		}

		return &st, nil
	case "HostStatusChange":
		var st HostStatusChange

		err := st.Unmarshal(vm.Body)
		if err != nil {
			return nil, err
		}

		return &st, nil
	case "CheckedTaskList":
		var st CheckedTaskList

		err := st.Unmarshal(vm.Body)
		if err != nil {
			return nil, err
		}

		return &st, nil
	case "CheckTasks":
		var st CheckTasks

		err := st.Unmarshal(vm.Body)
		if err != nil {
			return nil, err
		}

		return &st, nil
	}

	return nil, ErrUnknownMessage
}

func JsondecodeMessage(vm *Message) (interface{}, error) {
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
