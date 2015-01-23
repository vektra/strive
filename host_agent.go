package strive

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/vektra/vega"
)

type HostAgent struct {
	mb MessageBus
	se TaskExecutor

	watchwg      sync.WaitGroup
	runningTasks map[string]TaskHandle
	stop         chan struct{}
}

func NewHostAgent(mb MessageBus, se TaskExecutor) *HostAgent {
	return &HostAgent{
		mb:           mb,
		se:           se,
		runningTasks: make(map[string]TaskHandle),
		stop:         make(chan struct{}),
	}
}

func (ha *HostAgent) Wait() {
	ha.watchwg.Wait()
}

func (ha *HostAgent) Run(recv MessageBusReceiver, hbrecv string, hbtime time.Duration) error {
	go ha.heartbeatLoop(hbrecv, hbtime)
	return recv.Receive(ha)
}

func (ha *HostAgent) Close() error {
	close(ha.stop)
	return nil
}

func (ha *HostAgent) HandleMessage(vm *vega.Message) error {
	msg, err := decodeMessage(vm)
	if err != nil {
		var em vega.Message
		em.Type = "GenericError"
		setBody(&em,
			&GenericError{
				fmt.Sprintf("%s: %s", err, vm.Type),
			},
		)

		ha.mb.SendMessage(vm.ReplyTo, &em)

		return err
	}

	switch specific := msg.(type) {
	case *StartTask:
		return ha.startTask(vm, specific)
	case *StopTask:
		return ha.stopTask(vm, specific)
	case *ListTasks:
		return ha.listTasks(vm, specific)
	}

	var em vega.Message
	em.Type = "GenericError"

	ge := &GenericError{
		fmt.Sprintf("%s: %s", ErrUnknownMessage.Error(), vm.Type),
	}

	setBody(&em, ge)

	ha.mb.SendMessage(vm.ReplyTo, &em)

	return ErrUnknownMessage
}

func (ha *HostAgent) listTasks(vm *vega.Message, lt *ListTasks) error {
	var tasks []string

	for task, _ := range ha.runningTasks {
		tasks = append(tasks, task)
	}

	sort.Strings(tasks)

	var ct vega.Message
	ct.Type = "CurrentTasks"
	setBody(&ct, &CurrentTasks{lt.OpId, tasks})

	return ha.mb.SendMessage(vm.ReplyTo, &ct)
}

func (ha *HostAgent) startTask(vm *vega.Message, st *StartTask) error {
	var ack vega.Message
	ack.Type = "OpAcknowledged"
	setBody(&ack, &OpAcknowledged{st.OpId})

	ha.mb.SendMessage(vm.ReplyTo, &ack)

	var out vega.Message

	th, err := ha.se.Run(st.Task)
	if err != nil {
		out.Type = "TaskStatusChange"
		setBody(&out, &TaskStatusChange{Id: st.Task.Id, Status: "error", Error: err.Error()})

		ha.mb.SendMessage(vm.ReplyTo, &out)

		return err
	}

	out.Type = "TaskStatusChange"
	setBody(&out, &TaskStatusChange{Id: st.Task.Id, Status: "running"})

	ha.mb.SendMessage(vm.ReplyTo, &out)

	ha.watchwg.Add(1)
	go func() {
		defer ha.watchwg.Done()

		err := th.Wait()

		var out vega.Message
		out.Type = "TaskStatusChange"

		if err != nil {
			setBody(&out, &TaskStatusChange{Id: st.Task.Id, Status: "failed", Error: err.Error()})
		} else {
			setBody(&out, &TaskStatusChange{Id: st.Task.Id, Status: "finished"})
		}

		ha.mb.SendMessage(vm.ReplyTo, &out)
	}()

	return nil
}

var ErrUnknownTask = errors.New("unknown task")

func (ha *HostAgent) stopTask(vm *vega.Message, st *StopTask) error {
	var status vega.Message
	status.Type = "OpAcknowledged"
	setBody(&status, &OpAcknowledged{st.OpId})

	ha.mb.SendMessage(vm.ReplyTo, &status)

	th, ok := ha.runningTasks[st.Id]
	if !ok {
		var se vega.Message
		se.Type = "StopError"
		setBody(&se, &StopError{OpId: st.OpId, Id: st.Id, Error: ErrUnknownTask.Error()})

		ha.mb.SendMessage(vm.ReplyTo, &se)

		return ErrUnknownTask
	}

	err := th.Stop(st.Force)
	if err != nil {
		var se vega.Message
		se.Type = "StopError"
		setBody(&se, &StopError{OpId: st.OpId, Id: st.Id, Error: err.Error()})

		ha.mb.SendMessage(vm.ReplyTo, &se)

		return err
	}

	return nil
}

func (ha *HostAgent) SendHeartbeat(mailbox string) error {
	var vm vega.Message
	vm.Type = "HostStatusChange"
	setBody(&vm, &HostStatusChange{Status: "online"})

	return ha.mb.SendMessage(mailbox, &vm)
}

func (ha *HostAgent) heartbeatLoop(hbrecv string, hbtime time.Duration) {
	tick := time.NewTicker(hbtime)

	for {
		select {
		case <-tick.C:
			ha.SendHeartbeat(hbrecv)
		case <-ha.stop:
			return
		}
	}
}
