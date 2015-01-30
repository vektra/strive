package strive

import (
	"errors"
	"expvar"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/vektra/vega"
)

var (
	// Counters
	haMessages     = expvar.NewInt("agent.messages")
	haErrors       = expvar.NewInt("agent.errors")
	haUnknownTasks = expvar.NewInt("agent.unknown_tasks")
	haStartTasks   = expvar.NewInt("agent.start_tasks")
	haStopTasks    = expvar.NewInt("agent.stop_tasks")

	// Gauges
	haRunningTasks = expvar.NewInt("agent.running_tasks")
)

type HostAgent struct {
	Id string

	mb MessageBus
	se TaskExecutor

	watchwg      sync.WaitGroup
	runningTasks map[string]TaskHandle

	messages chan *vega.Message
	shutdown chan bool
	done     chan error

	txn    string
	hbtime time.Duration
}

const cReceiveBacklog = 10

func NewHostAgent(id, txn string, hbtime time.Duration, mb MessageBus, se TaskExecutor) *HostAgent {
	return &HostAgent{
		Id:           id,
		mb:           mb,
		se:           se,
		runningTasks: make(map[string]TaskHandle),
		messages:     make(chan *vega.Message, cReceiveBacklog),
		shutdown:     make(chan bool, 3),
		done:         make(chan error),

		txn:    txn,
		hbtime: hbtime,
	}
}

func (ha *HostAgent) Shutdown() error {
	ha.shutdown <- false
	return nil
}

func (ha *HostAgent) WaitTilIdle() {
	ha.watchwg.Wait()
}

func (ha *HostAgent) Wait() error {
	ha.WaitTilIdle()
	return <-ha.done
}

func (ha *HostAgent) Close() error {
	ha.Shutdown()
	return ha.Wait()
}

func (ha *HostAgent) Kill() {
	ha.shutdown <- true
	ha.Wait()
}

func (ha *HostAgent) process() {
	tick := time.NewTicker(ha.hbtime)

	for {
		select {
		case <-tick.C:
			ha.SendHeartbeat(ha.txn)
		case msg := <-ha.messages:
			ha.HandleMessage(msg)
		case kill := <-ha.shutdown:
			if kill {
				close(ha.done)
				return
			}

			var dis vega.Message

			dis.Type = "HostStatusChange"

			setBody(&dis, &HostStatusChange{Status: "disabled", Host: ha.Id})
			ha.mb.SendMessage(ha.txn, &dis)

			waitChan := make(chan struct{})

			go func() {
				ha.WaitTilIdle()
				waitChan <- struct{}{}
			}()

			select {
			case <-waitChan:
				// ok!
			case <-ha.shutdown:
				// gtfo!
			}

			var vm vega.Message

			vm.Type = "HostStatusChange"

			setBody(&vm, &HostStatusChange{Status: "offline", Host: ha.Id})
			ha.mb.SendMessage(ha.txn, &vm)

			close(ha.done)
			return
		}
	}
}

func (ha *HostAgent) HandleMessage(vm *vega.Message) error {
	haMessages.Add(1)

	msg, err := decodeMessage(vm)
	if err != nil {
		haErrors.Add(1)

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

	haErrors.Add(1)

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
	haStartTasks.Add(1)

	var ack vega.Message
	ack.Type = "OpAcknowledged"
	setBody(&ack, &OpAcknowledged{st.OpId})

	ha.mb.SendMessage(vm.ReplyTo, &ack)

	var out vega.Message

	th, err := ha.se.Run(st.Task)
	if err != nil {
		haErrors.Add(1)

		out.Type = "TaskStatusChange"
		setBody(&out, &TaskStatusChange{Id: st.Task.Id, Status: "error", Error: err.Error()})

		ha.mb.SendMessage(vm.ReplyTo, &out)

		return err
	}

	haRunningTasks.Add(1)

	out.Type = "TaskStatusChange"
	setBody(&out, &TaskStatusChange{Id: st.Task.Id, Status: "running"})

	ha.mb.SendMessage(vm.ReplyTo, &out)

	ha.watchwg.Add(1)
	go func() {
		defer ha.watchwg.Done()

		err := th.Wait()

		haRunningTasks.Add(-1)

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
	haStopTasks.Add(1)

	var status vega.Message
	status.Type = "OpAcknowledged"
	setBody(&status, &OpAcknowledged{st.OpId})

	ha.mb.SendMessage(vm.ReplyTo, &status)

	th, ok := ha.runningTasks[st.Id]
	if !ok {
		haErrors.Add(1)
		haUnknownTasks.Add(1)

		var se vega.Message
		se.Type = "StopError"
		setBody(&se, &StopError{OpId: st.OpId, Id: st.Id, Error: ErrUnknownTask.Error()})

		ha.mb.SendMessage(vm.ReplyTo, &se)

		return ErrUnknownTask
	}

	err := th.Stop(st.Force)
	if err != nil {
		haErrors.Add(1)

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
