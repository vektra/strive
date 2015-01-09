package strive

import "errors"

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

func (t *Txn) HandleMessage(msg interface{}) error {
	switch specific := msg.(type) {
	case *UpdateState:
		return t.updateState(specific)
	}

	return nil
}

var ErrNotEnoughResource = errors.New("not enough of a resource")
var ErrNoResource = errors.New("no such resource")

func (t *Txn) updateState(us *UpdateState) error {
	t.state.Hosts = us.Hosts

	for name, host := range us.Hosts {
		t.available[name] = host.Resources
	}

	for _, task := range us.Tasks {
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
	for name, task := range us.Tasks {
		for host, res := range task.Resources {
			avail := t.available[host]

			for rname, rval := range res {
				avail[rname] -= rval
			}
		}

		t.state.Tasks[name] = task
	}

	return nil
}
