package strive

import (
	"errors"

	"github.com/vektra/vega"
)

type state struct {
	Hosts map[string]*Host
	Tasks map[string]*Task
}

type Txn struct {
	available map[string]map[string]int
	state     *state
}

func NewTxn() *Txn {
	return &Txn{
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
	}

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
