package strive

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"code.google.com/p/goprotobuf/proto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektra/neko"
	"github.com/vektra/tai64n"
)

func setBody(vm *Message, msg interface{}) {
	body, err := json.Marshal(msg)
	if err != nil {
		panic(err)
	}

	vm.Body = body
}

func TestHostAgent(t *testing.T) {
	n := neko.Start(t)

	var te MockTaskExecutor
	var mb MockMessageBus
	var th MockTaskHandle

	n.CheckMock(&te.Mock)
	n.CheckMock(&mb.Mock)
	n.CheckMock(&th.Mock)

	task := &Task{
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

		Description: &TaskDescription{
			Command: proto.String("date"),
		},

		LastUpdate: tai64n.Now(),
	}

	startMsg := &StartTask{
		OpId: proto.String("1"),
		Task: task,
	}

	stopMsg := &StopTask{
		OpId:   proto.String("2"),
		TaskId: task.TaskId,
		Force:  proto.Bool(false),
	}

	/*
		killMsg := &StopTask{
			Id:    task.Id,
			Force: true,
		}
	*/

	start := startMsg.Encode()
	start.ReplyTo = "sched"

	ack := OpAcknowledged{OpId: startMsg.OpId}.Encode()

	stop := stopMsg.Encode()
	stop.ReplyTo = "sched"

	var ha *HostAgent

	n.Setup(func() {
		ha = NewHostAgent("h1", "txn", 60*time.Second, &mb, &te)
		go ha.process()
	})

	n.Cleanup(func() {
		ha.Kill()
	})

	n.It("starts new tasks", func() {
		out := TaskStatusChange{
			TaskId: task.TaskId,
			Status: TaskStatus_RUNNING.Enum(),
		}.Encode()

		mb.On("SendMessage", "sched", out).Return(nil)

		mb.On("SendMessage", "sched", ack).Return(nil)

		done := TaskStatusChange{
			TaskId: task.TaskId,
			Status: TaskStatus_FINISHED.Enum(),
		}.Encode()

		mb.On("SendMessage", "sched", done).Return(nil)

		te.On("Run", task).Return(&th, nil)
		th.On("Wait").Return(nil)

		err := ha.HandleMessage(start)
		require.NoError(t, err)

		ha.WaitTilIdle()
	})

	n.It("sends a status change when the task ends", func() {
		mb.On("SendMessage", "sched", ack).Return(nil)

		out := TaskStatusChange{
			TaskId: task.TaskId,
			Status: TaskStatus_RUNNING.Enum(),
		}.Encode()

		mb.On("SendMessage", "sched", out).Return(nil)
		te.On("Run", task).Return(&th, nil)
		th.On("Wait").Return(nil)

		done := TaskStatusChange{
			TaskId: task.TaskId,
			Status: TaskStatus_FINISHED.Enum(),
		}.Encode()

		mb.On("SendMessage", "sched", done).Return(nil)

		err := ha.HandleMessage(start)
		require.NoError(t, err)

		ha.WaitTilIdle()
	})

	n.It("sends a status change if run errors out", func() {
		mb.On("SendMessage", "sched", ack).Return(nil)

		out := TaskStatusChange{
			TaskId: task.TaskId,
			Status: TaskStatus_ERROR.Enum(),
			Error:  proto.String("unable to start"),
		}.Encode()

		exp := errors.New("unable to start")

		mb.On("SendMessage", "sched", out).Return(nil)
		te.On("Run", task).Return(&th, exp)

		err := ha.HandleMessage(start)
		assert.Equal(t, err, exp)
	})

	n.It("sends a status change if wait errors out", func() {
		mb.On("SendMessage", "sched", ack).Return(nil)

		out := TaskStatusChange{
			TaskId: task.TaskId,
			Status: TaskStatus_RUNNING.Enum(),
		}.Encode()

		exp := errors.New("unable to start")

		mb.On("SendMessage", "sched", out).Return(nil)
		te.On("Run", task).Return(&th, nil)
		th.On("Wait").Return(exp)

		done := TaskStatusChange{
			TaskId: task.TaskId,
			Status: TaskStatus_FAILED.Enum(),
			Error:  proto.String("unable to start"),
		}.Encode()

		mb.On("SendMessage", "sched", done).Return(nil)

		err := ha.HandleMessage(start)
		require.NoError(t, err)

		ha.WaitTilIdle()
	})

	n.It("can stop a task that is running", func() {
		status := OpAcknowledged{OpId: stopMsg.OpId}.Encode()

		ha.runningTasks["task1"] = &th

		th.On("Stop", false).Return(nil)
		mb.On("SendMessage", "sched", status).Return(nil)

		err := ha.HandleMessage(stop)
		require.NoError(t, err)

		ha.WaitTilIdle()
	})

	n.It("sends an error message if the task is unknown", func() {
		status := OpAcknowledged{OpId: stopMsg.OpId}.Encode()

		se := StopError{
			OpId:   stopMsg.OpId,
			TaskId: stopMsg.TaskId,
			Error:  UnknownTask(stopMsg.GetTaskId()),
		}.Encode()

		mb.On("SendMessage", "sched", status).Return(nil)
		mb.On("SendMessage", "sched", se).Return(nil)

		err := ha.HandleMessage(stop)
		require.Equal(t, err, ErrUnknownTask)
	})

	n.It("reports an error if stop errors out", func() {
		status := OpAcknowledged{OpId: stopMsg.OpId}.Encode()

		ha.runningTasks["task1"] = &th

		fe := errors.New("unable to stop")

		se := StopError{
			OpId:   stopMsg.OpId,
			TaskId: stopMsg.TaskId,
			Error:  UnknownError(fe.Error()),
		}.Encode()

		th.On("Stop", false).Return(fe)
		mb.On("SendMessage", "sched", status).Return(nil)
		mb.On("SendMessage", "sched", se).Return(nil)

		err := ha.HandleMessage(stop)
		require.Equal(t, err, fe)

		ha.WaitTilIdle()
	})

	n.It("can list the tasks it is running", func() {
		list := ListTasks{OpId: proto.String("3")}.Encode()
		list.ReplyTo = "sched"

		ha.runningTasks["task1"] = &th
		ha.runningTasks["task2"] = &th

		cur := CurrentTasks{
			OpId:    proto.String("3"),
			TaskIds: []string{"task1", "task2"},
		}.Encode()

		mb.On("SendMessage", "sched", cur).Return(nil)

		err := ha.HandleMessage(list)
		require.NoError(t, err)
	})

	n.It("sends a generic error message when it can't decode a message", func() {
		var vm Message
		vm.ReplyTo = "sched"
		vm.Type = "NotARealMessageType"

		em := GenericError{
			Error: UnknownMessage(vm.Type),
		}.Encode()

		mb.On("SendMessage", "sched", em).Return(nil)

		err := ha.HandleMessage(&vm)
		require.Equal(t, err, ErrUnknownMessage)
	})

	n.It("sends a generic error message when it doesn't handle a message", func() {
		sc := TaskStatusChange{}.Encode()
		sc.ReplyTo = "sched"

		em := GenericError{
			Error: UnknownMessage(sc.Type),
		}.Encode()

		mb.On("SendMessage", "sched", em).Return(nil)

		err := ha.HandleMessage(sc)
		require.Equal(t, err, ErrUnknownMessage)
	})

	n.It("sends host status change messages at an interval", func() {
		sc := HostStatusChange{
			Status: HostStatus_ONLINE.Enum(),
			HostId: &ha.Id,
		}.Encode()

		mb.On("SendMessage", "txn", sc).Return(nil)

		err := ha.SendHeartbeat("txn")
		require.NoError(t, err)
	})

	n.It("sends the heartbeat with a period", func() {
		ha := NewHostAgent("h1", "txn", 60*time.Millisecond, &mb, &te)
		defer ha.Kill()

		go ha.process()

		sc := HostStatusChange{
			Status: HostStatus_ONLINE.Enum(),
			HostId: &ha.Id,
		}.Encode()
		mb.On("SendMessage", "txn", sc).Return(nil)

		time.Sleep(100 * time.Millisecond)
	})

	n.It("sends an offline message when shutdown", func() {
		dis := HostStatusChange{
			Status: HostStatus_DISABLED.Enum(),
			HostId: &ha.Id,
		}.Encode()

		mb.On("SendMessage", "txn", dis).Return(nil)

		off := HostStatusChange{
			Status: HostStatus_OFFLINE.Enum(),
			HostId: proto.String(ha.Id),
		}.Encode()

		mb.On("SendMessage", "txn", off).Return(nil)

		ha.Close()
	})

	n.It("waits for tasks to finish before going offline", func() {
		sleepTe := &SleepTaskExecutor{
			Time: 1 * time.Second,
			C:    make(chan struct{}),
		}

		ha := NewHostAgent("h1", "txn", 60*time.Millisecond, &mb, sleepTe)
		defer ha.Close()

		go ha.process()

		out := TaskStatusChange{
			TaskId: task.TaskId,
			Status: TaskStatus_RUNNING.Enum(),
		}.Encode()

		mb.On("SendMessage", "sched", out).Return(nil)

		mb.On("SendMessage", "sched", ack).Return(nil)

		err := ha.HandleMessage(start)
		require.NoError(t, err)

		dis := HostStatusChange{
			Status: HostStatus_DISABLED.Enum(),
			HostId: proto.String(ha.Id),
		}.Encode()

		mb.On("SendMessage", "txn", dis).Return(nil)

		ha.Shutdown()

		// We need the shutdown code to start
		time.Sleep(10 * time.Millisecond)

		mb.AssertExpectations(t)

		done := TaskStatusChange{
			TaskId: task.TaskId,
			Status: TaskStatus_FINISHED.Enum(),
		}.Encode()

		mb.On("SendMessage", "sched", done).Return(nil)

		hcm := HostStatusChange{
			Status: HostStatus_OFFLINE.Enum(),
			HostId: proto.String(ha.Id),
		}.Encode()

		mb.On("SendMessage", "txn", hcm).Return(nil)

		select {
		case <-sleepTe.C:
			// ok
		case <-time.Tick(1 * time.Second):
			t.Fatalf("task was not waited on")
		}

		assert.True(t, sleepTe.finished)
	})

	n.It("skips waiting further if shutdown again while shutting down", func() {
		sleepTe := &SleepTaskExecutor{
			Time: 1 * time.Second,
			C:    make(chan struct{}, 1),
		}

		ha := NewHostAgent("h1", "txn", 60*time.Millisecond, &mb, sleepTe)
		defer ha.Close()

		go ha.process()

		out := TaskStatusChange{
			TaskId: task.TaskId,
			Status: TaskStatus_RUNNING.Enum(),
		}.Encode()

		mb.On("SendMessage", "sched", out).Return(nil)

		mb.On("SendMessage", "sched", ack).Return(nil)

		err := ha.HandleMessage(start)
		require.NoError(t, err)

		dis := HostStatusChange{
			Status: HostStatus_DISABLED.Enum(),
			HostId: proto.String(ha.Id),
		}.Encode()

		mb.On("SendMessage", "txn", dis).Return(nil)

		ha.Shutdown()

		// We need the shutdown code to start
		time.Sleep(10 * time.Millisecond)

		mb.AssertExpectations(t)

		ha.Shutdown()

		hcm := HostStatusChange{
			Status: HostStatus_OFFLINE.Enum(),
			HostId: proto.String(ha.Id),
		}.Encode()

		mb.On("SendMessage", "txn", hcm).Return(nil)

		// We need the shutdown code to run again
		time.Sleep(10 * time.Millisecond)

		assert.False(t, sleepTe.finished)

		// In the background, it will eventually finish so we need to put
		// this here.
		done := TaskStatusChange{
			TaskId: task.TaskId,
			Status: TaskStatus_FINISHED.Enum(),
		}.Encode()

		mb.On("SendMessage", "sched", done).Return(nil)
	})

	n.It("diffs the list of tasks and returns nothing if all tasks present", func() {
		vm := CheckTasks{TaskIds: []string{"t1"}}.Encode()
		vm.ReplyTo = "txn"

		ctl := CheckedTaskList{HostId: proto.String(ha.Id)}.Encode()

		mb.On("SendMessage", "txn", ctl).Return(nil)

		ha.runningTasks["t1"] = &th

		err := ha.HandleMessage(vm)
		require.NoError(t, err)
	})

	n.It("diffs the list of tasks and returns ones it's missing", func() {
		vm := CheckTasks{TaskIds: []string{"t1"}}.Encode()
		vm.ReplyTo = "txn"

		ctl := CheckedTaskList{
			HostId:  proto.String(ha.Id),
			Missing: []string{"t1"},
		}.Encode()

		mb.On("SendMessage", "txn", ctl).Return(nil)

		err := ha.HandleMessage(vm)
		require.NoError(t, err)
	})

	n.It("diffs the list of tasks and returns ones it has but the request doesn't", func() {
		vm := CheckTasks{TaskIds: []string{"t1"}}.Encode()
		vm.ReplyTo = "txn"

		ctl := CheckedTaskList{
			HostId:  proto.String(ha.Id),
			Unknown: []string{"t2"},
		}.Encode()

		mb.On("SendMessage", "txn", ctl).Return(nil)

		ha.runningTasks["t1"] = &th
		ha.runningTasks["t2"] = &th

		err := ha.HandleMessage(vm)
		require.NoError(t, err)
	})

	n.Meow()
}

type SleepTaskExecutor struct {
	finished bool
	Time     time.Duration
	C        chan struct{}
}

func (se *SleepTaskExecutor) Run(task *Task) (TaskHandle, error) {
	return se, nil
}

func (se *SleepTaskExecutor) Stop(force bool) error {
	return nil
}

func (se *SleepTaskExecutor) Wait() error {
	time.Sleep(se.Time)
	se.finished = true
	se.C <- struct{}{}
	return nil
}
