package strive

import (
	"errors"
	"expvar"
	"sort"
	"sync"
	"time"

	"code.google.com/p/gogoprotobuf/proto"
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

	messages chan *Message
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
		messages:     make(chan *Message, cReceiveBacklog),
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

			dis := HostStatusChange{
				Status: HostStatus_DISABLED.Enum(),
				HostId: proto.String(ha.Id),
			}.Encode()

			ha.mb.SendMessage(ha.txn, dis)

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

			sc := HostStatusChange{
				Status: HostStatus_OFFLINE.Enum(),
				HostId: proto.String(ha.Id),
			}.Encode()

			ha.mb.SendMessage(ha.txn, sc)

			close(ha.done)
			return
		}
	}
}

func (ha *HostAgent) HandleMessage(vm *Message) error {
	haMessages.Add(1)

	msg, err := decodeMessage(vm)
	if err != nil {
		haErrors.Add(1)

		em := GenericError{
			Error: UnknownMessage(vm.Type),
		}.Encode()

		ha.mb.SendMessage(vm.ReplyTo, em)

		return err
	}

	switch specific := msg.(type) {
	case *StartTask:
		return ha.startTask(vm, specific)
	case *StopTask:
		return ha.stopTask(vm, specific)
	case *ListTasks:
		return ha.listTasks(vm, specific)
	case *CheckTasks:
		return ha.checkTasks(vm, specific)
	}

	haErrors.Add(1)

	em := GenericError{
		Error: UnknownMessage(vm.Type),
	}.Encode()

	ha.mb.SendMessage(vm.ReplyTo, em)

	return ErrUnknownMessage
}

func (ha *HostAgent) listTasks(vm *Message, lt *ListTasks) error {
	var tasks []string

	for task, _ := range ha.runningTasks {
		tasks = append(tasks, task)
	}

	sort.Strings(tasks)

	ct := CurrentTasks{
		OpId:    lt.OpId,
		TaskIds: tasks,
	}.Encode()

	return ha.mb.SendMessage(vm.ReplyTo, ct)
}

func (ha *HostAgent) startTask(vm *Message, st *StartTask) error {
	haStartTasks.Add(1)

	ack := OpAcknowledged{OpId: st.OpId}.Encode()
	ha.mb.SendMessage(vm.ReplyTo, ack)

	var out *Message

	th, err := ha.se.Run(st.Task)
	if err != nil {
		haErrors.Add(1)

		out := TaskStatusChange{
			TaskId: st.Task.TaskId,
			Status: TaskStatus_ERROR.Enum(),
			Error:  proto.String(err.Error()),
		}.Encode()

		ha.mb.SendMessage(vm.ReplyTo, out)

		return err
	}

	haRunningTasks.Add(1)

	out = TaskStatusChange{
		TaskId: st.Task.TaskId,
		Status: TaskStatus_RUNNING.Enum(),
	}.Encode()

	ha.mb.SendMessage(vm.ReplyTo, out)

	ha.watchwg.Add(1)
	go func() {
		defer ha.watchwg.Done()

		err := th.Wait()

		haRunningTasks.Add(-1)

		var out *Message

		if err != nil {
			out = TaskStatusChange{
				TaskId: st.Task.TaskId,
				Status: TaskStatus_FAILED.Enum(),
				Error:  proto.String(err.Error()),
			}.Encode()
		} else {
			out = TaskStatusChange{
				TaskId: st.Task.TaskId,
				Status: TaskStatus_FINISHED.Enum(),
			}.Encode()
		}

		ha.mb.SendMessage(vm.ReplyTo, out)
	}()

	return nil
}

var ErrUnknownTask = errors.New("unknown task")

func (ha *HostAgent) stopTask(vm *Message, st *StopTask) error {
	haStopTasks.Add(1)

	status := OpAcknowledged{OpId: st.OpId}.Encode()
	ha.mb.SendMessage(vm.ReplyTo, status)

	th, ok := ha.runningTasks[st.GetTaskId()]

	if !ok {
		haErrors.Add(1)
		haUnknownTasks.Add(1)

		se := StopError{
			OpId:   st.OpId,
			TaskId: st.TaskId,
			Error:  UnknownTask(st.GetTaskId()),
		}.Encode()

		ha.mb.SendMessage(vm.ReplyTo, se)

		return ErrUnknownTask
	}

	err := th.Stop(st.GetForce())
	if err != nil {
		haErrors.Add(1)

		se := StopError{
			OpId:   st.OpId,
			TaskId: st.TaskId,
			Error:  UnknownError(err.Error()),
		}.Encode()

		ha.mb.SendMessage(vm.ReplyTo, se)

		return err
	}

	return nil
}

func (ha *HostAgent) checkTasks(vm *Message, ct *CheckTasks) error {
	var missing []string

	for _, id := range ct.TaskIds {
		if _, ok := ha.runningTasks[id]; !ok {
			missing = append(missing, id)
		}
	}

	var unknown []string

	for local, _ := range ha.runningTasks {
		found := false

		for _, remote := range ct.TaskIds {
			if local == remote {
				found = true
				break
			}
		}

		if !found {
			unknown = append(unknown, local)
		}
	}

	out := CheckedTaskList{
		HostId:  proto.String(ha.Id),
		Missing: missing,
		Unknown: unknown,
	}.Encode()

	return ha.mb.SendMessage(vm.ReplyTo, out)
}

func (ha *HostAgent) SendHeartbeat(mailbox string) error {
	sc := HostStatusChange{
		Status: HostStatus_ONLINE.Enum(),
		HostId: proto.String(ha.Id),
	}.Encode()

	return ha.mb.SendMessage(mailbox, sc)
}
