package strive

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektra/neko"
	"github.com/vektra/vega"
)

func TestTxn(t *testing.T) {
	n := neko.Start(t)

	var vm *vega.Message

	n.Setup(func() {
		vm = &vega.Message{}
	})

	set := func(us *UpdateState) {
		vm.Type = "UpdateState"
		setBody(vm, us)
	}

	n.It("adds new hosts to the state", func() {
		host := &Host{
			ID: "t1",
			Resources: map[string]int{
				"cpu": 4,
				"mem": 1024,
			},
		}

		set(&UpdateState{
			AddHosts: []*Host{host},
		})

		txn := NewTxn()

		err := txn.HandleMessage(vm)
		require.NoError(t, err)

		h2 := txn.state.Hosts["t1"]

		assert.Equal(t, host, h2)

		assert.Equal(t, txn.available["t1"]["cpu"], 4)
		assert.Equal(t, txn.available["t1"]["mem"], 1024)
	})

	n.It("accepts new tasks", func() {
		task := &Task{
			Id: "task1",
			Resources: map[string]TaskResources{
				"t1": TaskResources{
					"cpu": 1,
					"mem": 512,
				},
			},
		}

		set(&UpdateState{
			AddTasks: []*Task{task},
		})

		txn := NewTxn()

		txn.available["t1"] = map[string]int{
			"cpu": 4,
			"mem": 1024,
		}

		err := txn.HandleMessage(vm)
		require.NoError(t, err)

		assert.Equal(t, txn.state.Tasks["task1"], task)

		assert.Equal(t, txn.available["t1"]["cpu"], 3)
		assert.Equal(t, txn.available["t1"]["mem"], 512)
	})

	n.It("rejects new tasks when a resource is missing", func() {
		task := &Task{
			Id: "task1",
			Resources: map[string]TaskResources{
				"t1": TaskResources{
					"cpu": 1,
					"mem": 512,
				},
			},
		}

		set(&UpdateState{
			AddTasks: []*Task{task},
		})

		txn := NewTxn()

		err := txn.HandleMessage(vm)
		assert.Equal(t, err, ErrNoResource)

		assert.Nil(t, txn.state.Tasks["task1"])
	})

	n.It("rejects new tasks when there is not enough of a resource", func() {
		task := &Task{
			Id: "task1",
			Resources: map[string]TaskResources{
				"t1": TaskResources{
					"cpu": 1,
					"mem": 2048,
				},
			},
		}

		set(&UpdateState{
			AddTasks: []*Task{task},
		})

		txn := NewTxn()

		txn.available["t1"] = map[string]int{
			"cpu": 4,
			"mem": 1024,
		}

		err := txn.HandleMessage(vm)
		assert.Equal(t, err, ErrNotEnoughResource)

		assert.Nil(t, txn.state.Tasks["task1"])

		assert.Equal(t, txn.available["t1"]["cpu"], 4)
		assert.Equal(t, txn.available["t1"]["mem"], 1024)
	})

	n.Meow()
}
