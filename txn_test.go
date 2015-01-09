package strive

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektra/neko"
)

func TestTxn(t *testing.T) {
	n := neko.Start(t)

	n.It("adds new hosts to the state", func() {
		host := &Host{
			Resources: map[string]int{
				"cpu": 4,
				"mem": 1024,
			},
		}

		msg := &UpdateState{
			Hosts: map[string]*Host{
				"t1": host,
			},
		}

		txn := NewTxn()

		err := txn.HandleMessage(msg)
		require.NoError(t, err)

		h2 := txn.state.Hosts["t1"]

		assert.Equal(t, host, h2)

		assert.Equal(t, txn.available["t1"]["cpu"], 4)
		assert.Equal(t, txn.available["t1"]["mem"], 1024)
	})

	n.It("accepts new tasks", func() {
		task := &Task{
			Resources: map[string]TaskResources{
				"t1": TaskResources{
					"cpu": 1,
					"mem": 512,
				},
			},
		}

		msg := &UpdateState{
			Tasks: map[string]*Task{
				"task1": task,
			},
		}

		txn := NewTxn()

		txn.available["t1"] = map[string]int{
			"cpu": 4,
			"mem": 1024,
		}

		err := txn.HandleMessage(msg)
		require.NoError(t, err)

		assert.Equal(t, txn.state.Tasks["task1"], task)

		assert.Equal(t, txn.available["t1"]["cpu"], 3)
		assert.Equal(t, txn.available["t1"]["mem"], 512)
	})

	n.It("rejects new tasks when a resource is missing", func() {
		task := &Task{
			Resources: map[string]TaskResources{
				"t1": TaskResources{
					"cpu": 1,
					"mem": 512,
				},
			},
		}

		msg := &UpdateState{
			Tasks: map[string]*Task{
				"task1": task,
			},
		}

		txn := NewTxn()

		err := txn.HandleMessage(msg)
		assert.Equal(t, err, ErrNoResource)

		assert.Nil(t, txn.state.Tasks["task1"])
	})

	n.It("rejects new tasks when there is not enough of a resource", func() {
		task := &Task{
			Resources: map[string]TaskResources{
				"t1": TaskResources{
					"cpu": 1,
					"mem": 2048,
				},
			},
		}

		msg := &UpdateState{
			Tasks: map[string]*Task{
				"task1": task,
			},
		}

		txn := NewTxn()

		txn.available["t1"] = map[string]int{
			"cpu": 4,
			"mem": 1024,
		}

		err := txn.HandleMessage(msg)
		assert.Equal(t, err, ErrNotEnoughResource)

		assert.Nil(t, txn.state.Tasks["task1"])

		assert.Equal(t, txn.available["t1"]["cpu"], 4)
		assert.Equal(t, txn.available["t1"]["mem"], 1024)
	})

	n.Meow()
}
