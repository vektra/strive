package strive

import (
	"testing"
	"time"

	"code.google.com/p/goprotobuf/proto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektra/neko"
	"github.com/vektra/tai64n"
)

func TestTxn(t *testing.T) {
	n := neko.Start(t)

	var (
		mb   MockMessageBus
		opid MockOpIdGenerator
		vm   *Message
		txn  *Txn
	)

	n.CheckMock(&mb.Mock)
	n.CheckMock(&opid.Mock)

	var host *Host
	var task *Task

	n.Setup(func() {
		vm = &Message{}
		txn = NewTxn(&TxnConfig{&mb, &opid, 2 * time.Minute})
		host = &Host{
			HostId: proto.String("t1"),
			Resources: []*Resource{
				{
					Type:  Resource_CPU.Enum(),
					Value: NewIntValue(4),
				},
				{
					Type:  Resource_MEMORY.Enum(),
					Value: NewIntValue(1024),
				},
			},
		}

		task = &Task{
			TaskId: proto.String("task1"),
			Resources: []*HostResource{
				{
					HostId: proto.String("t1"),
					Resources: []*Resource{
						{
							Type:  Resource_CPU.Enum(),
							Value: NewIntValue(1),
						},
						{
							Type:  Resource_MEMORY.Enum(),
							Value: NewIntValue(512),
						},
					},
				},
			},
			Status:     TaskStatus_CREATED.Enum(),
			LastUpdate: tai64n.Now(),
		}

	})

	n.It("adds new hosts to the state", func() {
		us := UpdateState{
			AddHosts: []*Host{host},
		}.Encode()

		err := txn.HandleMessage(us)
		require.NoError(t, err)

		h2 := txn.state.Hosts["t1"]

		assert.Equal(t, host.HostId, h2.HostId)
		assert.Equal(t, h2.Status, HostStatus_ONLINE.Enum())

		cpu, ok := txn.state.Available["t1"].Find(Resource_CPU)
		require.True(t, ok)

		assert.Equal(t, cpu.GetValue().GetIntVal(), 4)

		mem, ok := txn.state.Available["t1"].Find(Resource_MEMORY)
		require.True(t, ok)

		assert.Equal(t, mem.GetValue().GetIntVal(), 1024)
	})

	n.It("updates the LastHeartbeat of new hosts", func() {
		now := tai64n.Now()
		host.LastHeartbeat = now

		us := UpdateState{
			AddHosts: []*Host{host},
		}.Encode()

		err := txn.HandleMessage(us)
		require.NoError(t, err)

		h2 := txn.state.Hosts["t1"]

		assert.True(t, h2.LastHeartbeat.After(now))
	})

	n.It("accepts new tasks", func() {
		us := UpdateState{
			AddTasks: []*Task{task},
		}.Encode()

		txn.state.Available["t1"] = NewResources([]*Resource{
			&Resource{
				Type:  Resource_CPU.Enum(),
				Value: NewIntValue(4),
			},
			&Resource{
				Type:  Resource_MEMORY.Enum(),
				Value: NewIntValue(1024),
			},
		})

		err := txn.HandleMessage(us)
		require.NoError(t, err)

		task.LastUpdate = txn.state.Tasks["task1"].LastUpdate
		assert.Equal(t, txn.state.Tasks["task1"], task)

		cpu, ok := txn.state.Available["t1"].Find(Resource_CPU)
		require.True(t, ok)

		mem, ok := txn.state.Available["t1"].Find(Resource_MEMORY)
		require.True(t, ok)

		assert.Equal(t, cpu.GetValue().GetIntVal(), 3)
		assert.Equal(t, mem.GetValue().GetIntVal(), 512)
	})

	n.It("tracks the port resources of tasks", func() {
		task.Resources = append(task.Resources,
			&HostResource{
				HostId: proto.String("t1"),
				Resources: []*Resource{
					{
						Type: Resource_PORT.Enum(),
						Value: NewRangesValue(
							&Range{
								Start: proto.Int64(1000),
								End:   proto.Int64(1010),
							},
						),
					},
				},
			},
		)

		us := UpdateState{
			AddTasks: []*Task{task},
		}.Encode()

		txn.state.Available["t1"] = NewResources([]*Resource{
			&Resource{
				Type:  Resource_CPU.Enum(),
				Value: NewIntValue(4),
			},
			&Resource{
				Type:  Resource_MEMORY.Enum(),
				Value: NewIntValue(1024),
			},
			&Resource{
				Type: Resource_PORT.Enum(),
				Value: NewRangesValue(&Range{
					Start: proto.Int64(1000),
					End:   proto.Int64(2000),
				}),
			},
		})

		err := txn.HandleMessage(us)
		require.NoError(t, err)

		ports := txn.state.Available["t1"].FindAll(Resource_PORT)
		assert.Equal(t, len(ports), 1)

		r := ports[0].GetValue().GetRangeVal().Ranges[0]

		assert.Equal(t, *r.Start, 1011)
		assert.Equal(t, *r.End, 2000)
	})

	n.It("sets the status of a new task to created", func() {
		task.Status = TaskStatus_RUNNING.Enum()

		us := UpdateState{
			AddTasks: []*Task{task},
		}.Encode()

		txn.state.Available["t1"] = NewResources([]*Resource{
			&Resource{
				Type:  Resource_CPU.Enum(),
				Value: NewIntValue(4),
			},
			&Resource{
				Type:  Resource_MEMORY.Enum(),
				Value: NewIntValue(1024),
			},
		})

		err := txn.HandleMessage(us)
		require.NoError(t, err)

		assert.Equal(t, txn.state.Tasks["task1"].Status, TaskStatus_CREATED.Enum())
	})

	n.It("updates the LastUpdate of a new task to now", func() {
		now := tai64n.Now()

		task.Status = TaskStatus_RUNNING.Enum()

		us := UpdateState{
			AddTasks: []*Task{task},
		}.Encode()

		txn.state.Available["t1"] = NewResources([]*Resource{
			&Resource{
				Type:  Resource_CPU.Enum(),
				Value: NewIntValue(4),
			},
			&Resource{
				Type:  Resource_MEMORY.Enum(),
				Value: NewIntValue(1024),
			},
		})

		err := txn.HandleMessage(us)
		require.NoError(t, err)

		assert.True(t, txn.state.Tasks["task1"].LastUpdate.After(now))
	})

	n.It("rejects new tasks when a resource is missing", func() {
		us := UpdateState{
			AddTasks: []*Task{task},
		}.Encode()

		err := txn.HandleMessage(us)
		assert.Equal(t, err, ErrNoResource)

		assert.Nil(t, txn.state.Tasks["task1"])
	})

	n.It("rejects new tasks when there is not enough of a resource", func() {
		us := UpdateState{
			AddTasks: []*Task{task},
		}.Encode()

		txn.state.Available["t1"] = NewResources([]*Resource{
			&Resource{
				Type:  Resource_CPU.Enum(),
				Value: NewIntValue(4),
			},
			&Resource{
				Type:  Resource_MEMORY.Enum(),
				Value: NewIntValue(128),
			},
		})

		err := txn.HandleMessage(us)
		assert.Equal(t, err, ErrNotEnoughResource)

		assert.Nil(t, txn.state.Tasks["task1"])

		cpu, ok := txn.state.Available["t1"].Find(Resource_CPU)
		require.True(t, ok)

		mem, ok := txn.state.Available["t1"].Find(Resource_MEMORY)
		require.True(t, ok)

		assert.Equal(t, cpu.GetValue().GetIntVal(), 4)
		assert.Equal(t, mem.GetValue().GetIntVal(), 128)
	})

	n.It("rejects an update if the ports are not available", func() {
		task.Resources = append(task.Resources,
			&HostResource{
				HostId: proto.String("t1"),
				Resources: []*Resource{
					{
						Type: Resource_PORT.Enum(),
						Value: NewRangesValue(
							&Range{
								Start: proto.Int64(1999),
								End:   proto.Int64(2003),
							},
						),
					},
				},
			},
		)

		us := UpdateState{
			AddTasks: []*Task{task},
		}.Encode()

		txn.state.Available["t1"] = NewResources([]*Resource{
			&Resource{
				Type:  Resource_CPU.Enum(),
				Value: NewIntValue(4),
			},
			&Resource{
				Type:  Resource_MEMORY.Enum(),
				Value: NewIntValue(1024),
			},
			&Resource{
				Type: Resource_PORT.Enum(),
				Value: NewRangesValue(&Range{
					Start: proto.Int64(1000),
					End:   proto.Int64(2000),
				}),
			},
		})

		err := txn.HandleMessage(us)
		assert.Equal(t, err, ErrNotEnoughResource)

		ports := txn.state.Available["t1"].FindAll(Resource_PORT)
		assert.Equal(t, len(ports), 1)

		r := ports[0].GetValue().GetRangeVal().Ranges[0]

		assert.Equal(t, *r.Start, 1000)
		assert.Equal(t, *r.End, 2000)
	})

	n.It("updates a hosts heartbeat field when it recieves a status change", func() {
		txn.state.Hosts["h1"] = &Host{
			Status:        HostStatus_ONLINE.Enum(),
			LastHeartbeat: tai64n.Now().Add(-61 * time.Second),
		}

		now := tai64n.Now()

		vm := HostStatusChange{
			HostId: proto.String("h1"),
			Status: HostStatus_ONLINE.Enum(),
		}.Encode()

		err := txn.HandleMessage(vm)
		require.NoError(t, err)

		assert.True(t, txn.state.Hosts["h1"].LastHeartbeat.After(now))
	})

	n.It("updates a host status to reflect status in message", func() {
		txn.state.Hosts["h1"] = &Host{
			Status:        HostStatus_ONLINE.Enum(),
			LastHeartbeat: tai64n.Now().Add(-61 * time.Second),
		}

		now := tai64n.Now()

		vm := HostStatusChange{
			HostId: proto.String("h1"),
			Status: HostStatus_OFFLINE.Enum(),
		}.Encode()

		err := txn.HandleMessage(vm)
		require.NoError(t, err)

		assert.True(t, txn.state.Hosts["h1"].LastHeartbeat.After(now))
		assert.Equal(t, txn.state.Hosts["h1"].Status, HostStatus_OFFLINE.Enum())
	})

	n.It("sets hosts to offline if they've missed heartbeats", func() {
		txn.state.Hosts["h1"] = &Host{
			Status:        HostStatus_ONLINE.Enum(),
			LastHeartbeat: tai64n.Now().Add(-61 * time.Second),
		}

		err := txn.checkHeartbeats(60 * time.Second)
		require.NoError(t, err)

		assert.Equal(t, txn.state.Hosts["h1"].Status, HostStatus_OFFLINE.Enum())
	})

	n.It("sets the statuses of all the hosts tasks to lost", func() {
		txn.state.Hosts["h1"] = &Host{
			HostId:        proto.String("h1"),
			Status:        HostStatus_ONLINE.Enum(),
			LastHeartbeat: tai64n.Now().Add(-61 * time.Second),
		}

		txn.state.Tasks["t1"] = &Task{
			TaskId:      proto.String("t1"),
			SchedulerId: proto.String("s1"),
			HostId:      proto.String("h1"),
			Status:      TaskStatus_RUNNING.Enum(),
		}

		vm := TaskStatusChange{
			TaskId: proto.String("t1"),
			Status: TaskStatus_LOST.Enum(),
		}.Encode()

		mb.On("SendMessage", "s1", vm).Return(nil)

		err := txn.checkHeartbeats(60 * time.Second)
		require.NoError(t, err)

		assert.Equal(t, txn.state.Tasks["t1"].Status, TaskStatus_LOST.Enum())
	})

	n.It("updates a task's status via TaskStatusChange messages", func() {
		txn.state.Tasks["t1"] = &Task{
			TaskId:      proto.String("t1"),
			SchedulerId: proto.String("s1"),
			HostId:      proto.String("h1"),
			Status:      TaskStatus_CREATED.Enum(),
		}

		vm := TaskStatusChange{
			TaskId: proto.String("t1"),
			Status: TaskStatus_RUNNING.Enum(),
		}.Encode()

		err := txn.HandleMessage(vm)
		require.NoError(t, err)

		assert.Equal(t, txn.state.Tasks["t1"].Status, TaskStatus_RUNNING.Enum())
	})

	n.It("updates a task's LastUpdated via TaskStatusChange messages", func() {
		now := tai64n.Now()

		txn.state.Tasks["t1"] = &Task{
			TaskId:      proto.String("t1"),
			SchedulerId: proto.String("s1"),
			HostId:      proto.String("h1"),
			Status:      TaskStatus_CREATED.Enum(),
			LastUpdate:  now,
		}

		vm := TaskStatusChange{
			TaskId: proto.String("t1"),
			Status: TaskStatus_RUNNING.Enum(),
		}.Encode()

		err := txn.HandleMessage(vm)
		require.NoError(t, err)

		assert.True(t, txn.state.Tasks["t1"].LastUpdate.After(now))
	})

	n.It("detects and starts missing tasks on agents", func() {
		task := &Task{
			TaskId:      proto.String("t1"),
			SchedulerId: proto.String("s1"),
			HostId:      proto.String("h1"),
			Status:      TaskStatus_CREATED.Enum(),
		}

		txn.state.Tasks["t1"] = task

		vm := CheckTasks{TaskIds: []string{"t1"}}.Encode()

		mb.On("SendMessage", "h1", vm).Return(nil)

		txn.PerformAntiEntropy()

		run := CheckedTaskList{Missing: []string{"t1"}}.Encode()

		opid.On("NextOpId").Return("foo1")

		start := StartTask{OpId: proto.String("foo1"), Task: task}.Encode()

		mb.On("SendMessage", "h1", start).Return(nil)

		err := txn.HandleMessage(run)
		require.NoError(t, err)
	})

	n.It("aggregates task ids when sending CheckTasks", func() {
		task := &Task{
			TaskId:      proto.String("t1"),
			SchedulerId: proto.String("s1"),
			HostId:      proto.String("h1"),
			Status:      TaskStatus_CREATED.Enum(),
		}

		txn.state.Tasks["t1"] = task

		task2 := &Task{
			TaskId:      proto.String("t2"),
			SchedulerId: proto.String("s1"),
			HostId:      proto.String("h1"),
			Status:      TaskStatus_CREATED.Enum(),
		}

		txn.state.Tasks["t2"] = task2

		vm := CheckTasks{TaskIds: []string{"t1", "t2"}}.Encode()

		mb.On("SendMessage", "h1", vm).Return(nil)

		txn.PerformAntiEntropy()
	})

	n.It("stops unknown tasks", func() {
		task := &Task{
			TaskId:      proto.String("t1"),
			SchedulerId: proto.String("s1"),
			HostId:      proto.String("h1"),
			Status:      TaskStatus_CREATED.Enum(),
		}

		txn.state.Tasks["t1"] = task

		run := CheckedTaskList{
			HostId:  proto.String("h1"),
			Unknown: []string{"t2"},
		}.Encode()

		opid.On("NextOpId").Return("foo1")

		start := StopTask{
			OpId:   proto.String("foo1"),
			TaskId: proto.String("t2"),
			Force:  proto.Bool(true),
		}.Encode()

		mb.On("SendMessage", "h1", start).Return(nil)

		err := txn.HandleMessage(run)
		require.NoError(t, err)
	})

	n.It("only starts created or running tasks", func() {
		task1 := &Task{
			TaskId:      proto.String("t1"),
			SchedulerId: proto.String("s1"),
			HostId:      proto.String("h1"),
			Status:      TaskStatus_CREATED.Enum(),
		}

		txn.state.Tasks["t1"] = task1

		task2 := &Task{
			TaskId:      proto.String("t2"),
			SchedulerId: proto.String("s1"),
			HostId:      proto.String("h1"),
			Status:      TaskStatus_RUNNING.Enum(),
		}

		txn.state.Tasks["t2"] = task2

		task3 := &Task{
			TaskId:      proto.String("t3"),
			SchedulerId: proto.String("s1"),
			HostId:      proto.String("h1"),
			Status:      TaskStatus_FINISHED.Enum(),
		}

		txn.state.Tasks["t3"] = task3

		task4 := &Task{
			TaskId:      proto.String("t4"),
			SchedulerId: proto.String("s1"),
			HostId:      proto.String("h1"),
			Status:      TaskStatus_FAILED.Enum(),
		}

		txn.state.Tasks["t4"] = task4

		task5 := &Task{
			TaskId:      proto.String("t5"),
			SchedulerId: proto.String("s1"),
			HostId:      proto.String("h1"),
			Status:      TaskStatus_LOST.Enum(),
		}

		txn.state.Tasks["t5"] = task5

		run := CheckedTaskList{Missing: []string{"t1", "t2", "t3", "t4", "t5"}}.Encode()

		opid.On("NextOpId").Return("foo1")

		start := StartTask{OpId: proto.String("foo1"), Task: task1}.Encode()

		mb.On("SendMessage", "h1", start).Return(nil)

		opid.On("NextOpId").Return("foo1")

		start2 := StartTask{OpId: proto.String("foo1"), Task: task2}.Encode()

		mb.On("SendMessage", "h1", start2).Return(nil)

		err := txn.HandleMessage(run)
		require.NoError(t, err)
	})

	n.Meow()
}
