package strive

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektra/neko"
	"github.com/vektra/vega"
)

func TestTxn(t *testing.T) {
	n := neko.Start(t)

	var (
		mb   MockMessageBus
		opid MockOpIdGenerator
		vm   *vega.Message
		txn  *Txn
	)

	n.CheckMock(&mb.Mock)
	n.CheckMock(&opid.Mock)

	n.Setup(func() {
		vm = &vega.Message{}
		txn = NewTxn(&TxnConfig{&mb, &opid, 2 * time.Minute})
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

		err := txn.HandleMessage(vm)
		require.NoError(t, err)

		h2 := txn.state.Hosts["t1"]

		assert.Equal(t, host.ID, h2.ID)
		assert.Equal(t, h2.Status, "online")

		assert.Equal(t, txn.available["t1"]["cpu"], 4)
		assert.Equal(t, txn.available["t1"]["mem"], 1024)
	})

	n.It("updates the LastHeartbeat of new hosts", func() {
		now := time.Now()

		host := &Host{
			ID: "t1",
			Resources: map[string]int{
				"cpu": 4,
				"mem": 1024,
			},
			LastHeartbeat: now,
		}

		set(&UpdateState{
			AddHosts: []*Host{host},
		})

		err := txn.HandleMessage(vm)
		require.NoError(t, err)

		h2 := txn.state.Hosts["t1"]

		assert.True(t, h2.LastHeartbeat.After(now))
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
			Status:     "created",
			LastUpdate: time.Now(),
		}

		set(&UpdateState{
			AddTasks: []*Task{task},
		})

		txn.available["t1"] = map[string]int{
			"cpu": 4,
			"mem": 1024,
		}

		err := txn.HandleMessage(vm)
		require.NoError(t, err)

		task.LastUpdate = txn.state.Tasks["task1"].LastUpdate
		assert.Equal(t, txn.state.Tasks["task1"], task)

		assert.Equal(t, txn.available["t1"]["cpu"], 3)
		assert.Equal(t, txn.available["t1"]["mem"], 512)
	})

	n.It("sets the status of a new task to created", func() {
		task := &Task{
			Id: "task1",
			Resources: map[string]TaskResources{
				"t1": TaskResources{
					"cpu": 1,
					"mem": 512,
				},
			},
			Status: "running",
		}

		set(&UpdateState{
			AddTasks: []*Task{task},
		})

		txn.available["t1"] = map[string]int{
			"cpu": 4,
			"mem": 1024,
		}

		err := txn.HandleMessage(vm)
		require.NoError(t, err)

		assert.Equal(t, txn.state.Tasks["task1"].Status, "created")

		assert.Equal(t, txn.available["t1"]["cpu"], 3)
		assert.Equal(t, txn.available["t1"]["mem"], 512)
	})

	n.It("updates the LastUpdate of a new task to now", func() {
		now := time.Now()

		task := &Task{
			Id: "task1",
			Resources: map[string]TaskResources{
				"t1": TaskResources{
					"cpu": 1,
					"mem": 512,
				},
			},
			Status:     "running",
			LastUpdate: now,
		}

		set(&UpdateState{
			AddTasks: []*Task{task},
		})

		txn.available["t1"] = map[string]int{
			"cpu": 4,
			"mem": 1024,
		}

		err := txn.HandleMessage(vm)
		require.NoError(t, err)

		assert.True(t, txn.state.Tasks["task1"].LastUpdate.After(now))
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

	n.It("updates a hosts heartbeat field when it recieves a status change", func() {
		txn.state.Hosts["h1"] = &Host{
			Status:        "online",
			LastHeartbeat: time.Now().Add(-61 * time.Second),
		}

		now := time.Now()

		var vm vega.Message
		vm.Type = "HostStatusChange"
		setBody(&vm, &HostStatusChange{Host: "h1", Status: "online"})

		err := txn.HandleMessage(&vm)
		require.NoError(t, err)

		assert.True(t, txn.state.Hosts["h1"].LastHeartbeat.After(now))
	})

	n.It("updates a host status to reflect status in message", func() {
		txn.state.Hosts["h1"] = &Host{
			Status:        "online",
			LastHeartbeat: time.Now().Add(-61 * time.Second),
		}

		now := time.Now()

		var vm vega.Message
		vm.Type = "HostStatusChange"
		setBody(&vm, &HostStatusChange{Host: "h1", Status: "offline"})

		err := txn.HandleMessage(&vm)
		require.NoError(t, err)

		assert.True(t, txn.state.Hosts["h1"].LastHeartbeat.After(now))
		assert.Equal(t, txn.state.Hosts["h1"].Status, "offline")
	})

	n.It("sets hosts to offline if they've missed heartbeats", func() {
		txn.state.Hosts["h1"] = &Host{
			Status:        "online",
			LastHeartbeat: time.Now().Add(-61 * time.Second),
		}

		err := txn.checkHeartbeats(60 * time.Second)
		require.NoError(t, err)

		assert.Equal(t, txn.state.Hosts["h1"].Status, "offline")
	})

	n.It("sets the statuses of all the hosts tasks to lost", func() {
		txn.state.Hosts["h1"] = &Host{
			ID:            "h1",
			Status:        "online",
			LastHeartbeat: time.Now().Add(-61 * time.Second),
		}

		txn.state.Tasks["t1"] = &Task{
			Id:        "t1",
			Scheduler: "s1",
			Host:      "h1",
			Status:    "running",
		}

		vm.Type = "TaskStatusChange"
		setBody(vm, &TaskStatusChange{Id: "t1", Status: "lost"})

		mb.On("SendMessage", "s1", vm).Return(nil)

		err := txn.checkHeartbeats(60 * time.Second)
		require.NoError(t, err)

		assert.Equal(t, txn.state.Tasks["t1"].Status, "lost")
	})

	n.It("updates a task's status via TaskStatusChange messages", func() {
		txn.state.Tasks["t1"] = &Task{
			Id:        "t1",
			Scheduler: "s1",
			Host:      "h1",
			Status:    "created",
		}

		vm.Type = "TaskStatusChange"
		setBody(vm, &TaskStatusChange{Id: "t1", Status: "running"})

		err := txn.HandleMessage(vm)
		require.NoError(t, err)

		assert.Equal(t, txn.state.Tasks["t1"].Status, "running")
	})

	n.It("updates a task's LastUpdated via TaskStatusChange messages", func() {
		now := time.Now()

		txn.state.Tasks["t1"] = &Task{
			Id:         "t1",
			Scheduler:  "s1",
			Host:       "h1",
			Status:     "created",
			LastUpdate: now,
		}

		vm.Type = "TaskStatusChange"
		setBody(vm, &TaskStatusChange{Id: "t1", Status: "running"})

		err := txn.HandleMessage(vm)
		require.NoError(t, err)

		assert.True(t, txn.state.Tasks["t1"].LastUpdate.After(now))
	})

	n.It("detects and starts missing tasks on agents", func() {
		task := &Task{
			Id:        "t1",
			Scheduler: "s1",
			Host:      "h1",
			Status:    "created",
		}

		txn.state.Tasks["t1"] = task

		vm.Type = "CheckTasks"
		setBody(vm, &CheckTasks{Tasks: []string{"t1"}})

		mb.On("SendMessage", "h1", vm).Return(nil)

		txn.PerformAntiEntropy()

		run := &vega.Message{}

		run.Type = "CheckedTaskList"
		setBody(run, &CheckedTaskList{Missing: []string{"t1"}})

		opid.On("NextOpId").Return("foo1")

		start := &vega.Message{}
		start.Type = "StartTask"
		setBody(start, &StartTask{OpId: "foo1", Task: task})

		mb.On("SendMessage", "h1", start).Return(nil)

		err := txn.HandleMessage(run)
		require.NoError(t, err)
	})

	n.It("aggregates task ids when sending CheckTasks", func() {
		task := &Task{
			Id:        "t1",
			Scheduler: "s1",
			Host:      "h1",
			Status:    "created",
		}

		txn.state.Tasks["t1"] = task

		task2 := &Task{
			Id:        "t2",
			Scheduler: "s1",
			Host:      "h1",
			Status:    "created",
		}

		txn.state.Tasks["t2"] = task2

		vm.Type = "CheckTasks"
		setBody(vm, &CheckTasks{Tasks: []string{"t1", "t2"}})

		mb.On("SendMessage", "h1", vm).Return(nil)

		txn.PerformAntiEntropy()
	})

	n.It("stops unknown tasks", func() {
		task := &Task{
			Id:        "t1",
			Scheduler: "s1",
			Host:      "h1",
			Status:    "created",
		}

		txn.state.Tasks["t1"] = task

		run := &vega.Message{}

		run.Type = "CheckedTaskList"
		setBody(run, &CheckedTaskList{Host: "h1", Unknown: []string{"t2"}})

		opid.On("NextOpId").Return("foo1")

		start := &vega.Message{}
		start.Type = "StopTask"
		setBody(start, &StopTask{OpId: "foo1", Id: "t2", Force: true})

		mb.On("SendMessage", "h1", start).Return(nil)

		err := txn.HandleMessage(run)
		require.NoError(t, err)
	})

	n.It("only starts created or running tasks", func() {
		task1 := &Task{
			Id:        "t1",
			Scheduler: "s1",
			Host:      "h1",
			Status:    "created",
		}

		txn.state.Tasks["t1"] = task1

		task2 := &Task{
			Id:        "t2",
			Scheduler: "s1",
			Host:      "h1",
			Status:    "running",
		}

		txn.state.Tasks["t2"] = task2

		task3 := &Task{
			Id:        "t3",
			Scheduler: "s1",
			Host:      "h1",
			Status:    "finished",
		}

		txn.state.Tasks["t3"] = task3

		task4 := &Task{
			Id:        "t4",
			Scheduler: "s1",
			Host:      "h1",
			Status:    "failed",
		}

		txn.state.Tasks["t4"] = task4

		run := &vega.Message{}

		run.Type = "CheckedTaskList"
		setBody(run, &CheckedTaskList{Missing: []string{"t1", "t2", "t3", "t4"}})

		opid.On("NextOpId").Return("foo1")

		start := &vega.Message{}
		start.Type = "StartTask"
		setBody(start, &StartTask{OpId: "foo1", Task: task1})

		mb.On("SendMessage", "h1", start).Return(nil)

		opid.On("NextOpId").Return("foo1")

		start2 := &vega.Message{}
		start2.Type = "StartTask"
		setBody(start2, &StartTask{OpId: "foo1", Task: task2})

		mb.On("SendMessage", "h1", start2).Return(nil)

		err := txn.HandleMessage(run)
		require.NoError(t, err)
	})

	n.Meow()
}
