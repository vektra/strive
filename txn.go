package strive

import (
	"errors"
	"expvar"
	"sort"
	"time"

	"github.com/vektra/vega"
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
}

type Txn struct {
	mb   MessageBus
	opId OpIdGenerator

	available map[string]map[string]int
	state     *state

	messages     chan *vega.Message
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
			Hosts: make(map[string]*Host),
			Tasks: make(map[string]*Task),
		},
		messages: make(chan *vega.Message, cReceiveBacklog),
		shutdown: make(chan bool),
		done:     make(chan error),
	}
}

func (t *Txn) HandleMessage(vm *vega.Message) error {
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

	task, ok := t.state.Tasks[ts.Id]
	if !ok {
		txnUnknownTask.Add(1)
		return ErrUnknownTask
	}

	task.Status = ts.Status
	task.LastUpdate = time.Now()

	return nil
}

var ErrUnknownHost = errors.New("unknown host sent status change")

func (t *Txn) updateHost(hs *HostStatusChange) error {
	txnHostStatuses.Add(1)

	host, ok := t.state.Hosts[hs.Host]
	if !ok {
		txnUnknownHost.Add(1)
		return ErrUnknownHost
	}

	host.Status = hs.Status
	host.LastHeartbeat = time.Now()

	return nil
}

var ErrNotEnoughResource = errors.New("not enough of a resource")
var ErrNoResource = errors.New("no such resource")

func (t *Txn) updateState(us *UpdateState) error {
	txnUpdates.Add(1)

	for _, host := range us.AddHosts {
		host.LastHeartbeat = time.Now()
		host.Status = "online"

		t.state.Hosts[host.ID] = host
		t.available[host.ID] = host.Resources

		txnHosts.Set(int64(len(t.state.Hosts)))
	}

	for _, task := range us.AddTasks {
		for host, res := range task.Resources {
			avail := t.available[host]

			for rname, rval := range res {
				aval, ok := avail[rname]
				if !ok {
					txnUpdateErrors.Add(1)
					return ErrNoResource
				}

				if aval < rval {
					txnUpdateErrors.Add(1)
					return ErrNotEnoughResource
				}
			}
		}
	}

	// ok, we can commit the tasks
	for _, task := range us.AddTasks {
		for host, res := range task.Resources {
			avail := t.available[host]

			for rname, rval := range res {
				avail[rname] -= rval
			}
		}

		txnTasksCreated.Add(1)

		task.Status = "created"
		task.LastUpdate = time.Now()

		t.state.Tasks[task.Id] = task

		txnTasks.Set(int64(len(t.state.Tasks)))
	}

	return nil
}

func (t *Txn) checkHeartbeats(dur time.Duration) error {
	threshold := time.Now().Add(-dur)

	for _, host := range t.state.Hosts {
		if host.LastHeartbeat.Before(threshold) {
			txnFailedHeartbeats.Add(1)

			host.Status = "offline"

			for _, task := range t.state.Tasks {
				if task.Host == host.ID {
					txnLostTasks.Add(1)

					task.Status = "lost"

					if task.Scheduler != "" {
						var vm vega.Message
						vm.Type = "TaskStatusChange"
						setBody(&vm, &TaskStatusChange{
							Id:     task.Id,
							Status: "lost",
						})

						t.mb.SendMessage(task.Scheduler, &vm)
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
		hostTasks[task.Host] = append(hostTasks[task.Host], id)
	}

	for host, tasks := range hostTasks {
		sort.Strings(tasks)

		var vm vega.Message

		vm.Type = "CheckTasks"
		setBody(&vm, &CheckTasks{Tasks: tasks})

		t.mb.SendMessage(host, &vm)
	}
}

func (t *Txn) checkTasks(ctl *CheckedTaskList) error {
	for _, id := range ctl.Missing {
		task := t.state.Tasks[id]

		if task.Status == "created" || task.Status == "running" {
			var vm vega.Message

			vm.Type = "StartTask"
			setBody(&vm, &StartTask{OpId: t.opId.NextOpId(), Task: task})

			t.mb.SendMessage(task.Host, &vm)
		}
	}

	for _, id := range ctl.Unknown {
		var vm vega.Message

		vm.Type = "StopTask"
		setBody(&vm, &StopTask{OpId: t.opId.NextOpId(), Id: id, Force: true})

		t.mb.SendMessage(ctl.Host, &vm)
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

func (t *Txn) InjectMessage(vm *vega.Message) {
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
