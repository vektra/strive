package strive

import (
	"errors"
	"time"

	"github.com/vektra/vega"
)

type state struct {
	Hosts map[string]*Host
	Tasks map[string]*Task
}

type Txn struct {
	mb MessageBus

	available map[string]map[string]int
	state     *state
}

func NewTxn(mb MessageBus) *Txn {
	return &Txn{
		mb:        mb,
		available: make(map[string]map[string]int),
		state: &state{
			Hosts: make(map[string]*Host),
			Tasks: make(map[string]*Task),
		},
	}
}

func (t *Txn) HandleMessage(vm *vega.Message) error {
	msg, err := decodeMessage(vm)
	if err != nil {
		return err
	}

	switch specific := msg.(type) {
	case *UpdateState:
		return t.updateState(specific)
	case *HostStatusChange:
		return t.updateHost(specific)
	}

	return nil
}

var ErrUnknownHost = errors.New("unknown host sent status change")

func (t *Txn) updateHost(hs *HostStatusChange) error {
	host, ok := t.state.Hosts[hs.Host]
	if !ok {
		return ErrUnknownHost
	}

	host.Status = hs.Status
	host.LastHeartbeat = time.Now()

	return nil
}

var ErrNotEnoughResource = errors.New("not enough of a resource")
var ErrNoResource = errors.New("no such resource")

func (t *Txn) updateState(us *UpdateState) error {
	for _, host := range us.AddHosts {
		t.state.Hosts[host.ID] = host
		t.available[host.ID] = host.Resources
	}

	for _, task := range us.AddTasks {
		for host, res := range task.Resources {
			avail := t.available[host]

			for rname, rval := range res {
				aval, ok := avail[rname]
				if !ok {
					return ErrNoResource
				}

				if aval < rval {
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

		t.state.Tasks[task.Id] = task
	}

	return nil
}

func (t *Txn) checkHeartbeats(dur time.Duration) error {
	threshold := time.Now().Add(-dur)

	for _, host := range t.state.Hosts {
		if host.LastHeartbeat.Before(threshold) {
			host.Status = "offline"

			for _, task := range t.state.Tasks {
				if task.Host == host.ID {
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
