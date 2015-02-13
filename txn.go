package strive

import (
	"errors"
	"expvar"
	"sort"
	"time"

	"code.google.com/p/goprotobuf/proto"

	"github.com/vektra/tai64n"
)

var (
	// Counters
	txnMessages         = expvar.NewInt("txn.messages")
	txnUpdates          = expvar.NewInt("txn.updates")
	txnHostStatuses     = expvar.NewInt("txn.host_statuses")
	txnTaskStatuses     = expvar.NewInt("txn.task_statuses")
	txnUnknownTask      = expvar.NewInt("txn.unknown_tasks")
	txnUnknownHost      = expvar.NewInt("txn.unknown_hosts")
	txnUpdateErrors     = expvar.NewInt("txn.update_errors")
	txnTasksCreated     = expvar.NewInt("txn.tasks_created")
	txnFailedHeartbeats = expvar.NewInt("txn.failed_heartbeats")
	txnLostTasks        = expvar.NewInt("txn.tasks_lost")

	// Gauges
	txnHosts = expvar.NewInt("txn.hosts")
	txnTasks = expvar.NewInt("txn.tasks")
)

type state struct {
	Hosts map[string]*Host
	Tasks map[string]*Task

	Available map[string]Resources
}

type Txn struct {
	mb   MessageBus
	opId OpIdGenerator

	available map[string]map[string]int
	state     *state

	messages     chan *Message
	shutdown     chan bool
	done         chan error
	entropyTimer time.Duration
}

type TxnConfig struct {
	Bus          MessageBus
	OpId         OpIdGenerator
	EntropyTimer time.Duration
}

func NewTxn(cfg *TxnConfig) *Txn {
	return &Txn{
		mb:           cfg.Bus,
		opId:         cfg.OpId,
		entropyTimer: cfg.EntropyTimer,
		available:    make(map[string]map[string]int),
		state: &state{
			Hosts:     make(map[string]*Host),
			Tasks:     make(map[string]*Task),
			Available: make(map[string]Resources),
		},
		messages: make(chan *Message, cReceiveBacklog),
		shutdown: make(chan bool),
		done:     make(chan error),
	}
}

func (t *Txn) HandleMessage(vm *Message) error {
	txnMessages.Add(1)

	msg, err := decodeMessage(vm)
	if err != nil {
		return err
	}

	switch specific := msg.(type) {
	case *UpdateState:
		return t.updateState(specific)
	case *HostStatusChange:
		return t.updateHost(specific)
	case *TaskStatusChange:
		return t.updateTask(specific)
	case *CheckedTaskList:
		return t.checkTasks(specific)
	}

	return ErrUnknownMessage
}

func (t *Txn) updateTask(ts *TaskStatusChange) error {
	txnTaskStatuses.Add(1)

	task, ok := t.state.Tasks[ts.GetTaskId()]
	if !ok {
		txnUnknownTask.Add(1)
		return ErrUnknownTask
	}

	task.Status = ts.Status
	task.LastUpdate = tai64n.Now()

	return nil
}

var ErrUnknownHost = errors.New("unknown host sent status change")

func (t *Txn) updateHost(hs *HostStatusChange) error {
	txnHostStatuses.Add(1)

	host, ok := t.state.Hosts[hs.GetHostId()]
	if !ok {
		txnUnknownHost.Add(1)
		return ErrUnknownHost
	}

	host.Status = hs.Status
	host.LastHeartbeat = tai64n.Now()

	return nil
}

var ErrNotEnoughResource = errors.New("not enough of a resource")
var ErrNoResource = errors.New("no such resource")

func (t *Txn) updateState(us *UpdateState) error {
	txnUpdates.Add(1)

	for _, host := range us.AddHosts {
		host.LastHeartbeat = tai64n.Now()
		host.Status = HostStatus_ONLINE.Enum()

		t.state.Hosts[host.GetHostId()] = host
		t.state.Available[host.GetHostId()] = NewResources(host.Resources)

		txnHosts.Set(int64(len(t.state.Hosts)))
	}

	// calculate a new available resources list for a host and
	// bail if that's not possible.

	changes := make(map[string]Resources)

	for hr, ress := range t.state.Available {
		changes[hr] = ress.Dup()
	}

	for _, task := range us.AddTasks {
		for _, hostres := range task.Resources {
			avail := changes[hostres.GetHostId()]

			for _, res := range hostres.Resources {
				if res.StoreOnly() {
					avail.Add(res)
				} else {
					cur, ok := avail.Find(res.GetType())
					if !ok {
						return ErrNoResource
					}

					up, err := cur.Remove(res)
					if err != nil {
						return ErrNotEnoughResource
					}

					avail.Replace(cur, up)
				}
			}
		}
	}

	t.state.Available = changes

	for _, task := range us.AddTasks {
		txnTasksCreated.Add(1)

		task.Status = TaskStatus_CREATED.Enum()
		task.LastUpdate = tai64n.Now()

		t.state.Tasks[task.GetTaskId()] = task

		txnTasks.Set(int64(len(t.state.Tasks)))
	}

	return nil
}

func (t *Txn) checkHeartbeats(dur time.Duration) error {
	threshold := tai64n.Now().Add(-dur)

	for _, host := range t.state.Hosts {
		if host.LastHeartbeat.Before(threshold) {
			txnFailedHeartbeats.Add(1)

			host.Status = HostStatus_OFFLINE.Enum()

			for _, task := range t.state.Tasks {
				if task.GetHostId() == host.GetHostId() {
					txnLostTasks.Add(1)

					task.Status = TaskStatus_LOST.Enum()

					if sch := task.SchedulerId; sch != nil {
						vm := TaskStatusChange{
							TaskId: task.TaskId,
							Status: TaskStatus_LOST.Enum(),
						}.Encode()

						t.mb.SendMessage(*sch, vm)
					}
				}
			}
		}
	}

	return nil
}

func (t *Txn) PerformAntiEntropy() {
	hostTasks := map[string][]string{}

	for id, task := range t.state.Tasks {
		hostTasks[task.GetHostId()] = append(hostTasks[task.GetHostId()], id)
	}

	for host, tasks := range hostTasks {
		sort.Strings(tasks)

		vm := CheckTasks{TaskIds: tasks}.Encode()

		t.mb.SendMessage(host, vm)
	}
}

func (t *Txn) checkTasks(ctl *CheckedTaskList) error {
	for _, id := range ctl.Missing {
		task := t.state.Tasks[id]

		if task.Healthy() {
			vm := StartTask{
				OpId: proto.String(t.opId.NextOpId()),
				Task: task,
			}.Encode()

			t.mb.SendMessage(task.GetHostId(), vm)
		}
	}

	for _, id := range ctl.Unknown {
		vm := StopTask{
			OpId:   proto.String(t.opId.NextOpId()),
			TaskId: proto.String(id),
			Force:  proto.Bool(true),
		}.Encode()

		t.mb.SendMessage(ctl.GetHostId(), vm)
	}

	return nil
}

func (t *Txn) Process() {
	tick := time.NewTicker(t.entropyTimer)

	for {
		select {
		case m := <-t.messages:
			t.HandleMessage(m)
		case <-tick.C:
			t.PerformAntiEntropy()
		case <-t.shutdown:
			close(t.done)
			return
		}
	}
}

func (t *Txn) InjectMessage(vm *Message) {
	t.messages <- vm
}

func (t *Txn) Close() error {
	select {
	case t.shutdown <- true:
		// nothing
	default:
		close(t.shutdown)
	}

	<-t.done
	return nil
}
