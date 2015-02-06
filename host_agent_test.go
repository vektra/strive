package strive

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektra/neko"
	"github.com/vektra/vega"
)

func setBody(vm *vega.Message, msg interface{}) {
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
		Id: "task1",
		Resources: map[string]TaskResources{
			"t1": TaskResources{
				"cpu": 1,
				"mem": 512,
			},
		},

		Description: &TaskDescription{
			Command: "date",
		},

		LastUpdate: time.Now(),
	}

	startMsg := &StartTask{
		OpId: "1",
		Task: task,
	}

	stopMsg := &StopTask{
		OpId:  "2",
		Id:    task.Id,
		Force: false,
	}

	/*
		killMsg := &StopTask{
			Id:    task.Id,
			Force: true,
		}
	*/

	var ha *HostAgent

	n.Setup(func() {
		ha = NewHostAgent("h1", "txn", 60*time.Second, &mb, &te)
		go ha.process()
	})

	n.Cleanup(func() {
		ha.Kill()
	})

	n.It("starts new tasks", func() {
		var vm vega.Message

		body, err := json.Marshal(startMsg)
		require.NoError(t, err)

		vm.ReplyTo = "sched"
		vm.Type = "StartTask"
		vm.Body = body

		var out vega.Message
		out.Type = "TaskStatusChange"
		setBody(&out, &TaskStatusChange{Id: task.Id, Status: "running"})

		mb.On("SendMessage", "sched", &out).Return(nil)

		var ack vega.Message
		ack.Type = "OpAcknowledged"
		setBody(&ack, &OpAcknowledged{startMsg.OpId})

		mb.On("SendMessage", "sched", &ack).Return(nil)

		var done vega.Message
		done.Type = "TaskStatusChange"
		setBody(&done, &TaskStatusChange{Id: task.Id, Status: "finished"})

		mb.On("SendMessage", "sched", &done).Return(nil)

		te.On("Run", task).Return(&th, nil)
		th.On("Wait").Return(nil)

		err = ha.HandleMessage(&vm)
		require.NoError(t, err)

		ha.WaitTilIdle()
	})

	n.It("sends a status change when the task ends", func() {
		var vm vega.Message

		body, err := json.Marshal(startMsg)
		require.NoError(t, err)

		vm.ReplyTo = "sched"
		vm.Type = "StartTask"
		vm.Body = body

		var ack vega.Message
		ack.Type = "OpAcknowledged"
		setBody(&ack, &OpAcknowledged{startMsg.OpId})

		mb.On("SendMessage", "sched", &ack).Return(nil)

		var out vega.Message
		out.Type = "TaskStatusChange"
		setBody(&out, &TaskStatusChange{Id: task.Id, Status: "running"})

		mb.On("SendMessage", "sched", &out).Return(nil)
		te.On("Run", task).Return(&th, nil)
		th.On("Wait").Return(nil)

		var done vega.Message
		done.Type = "TaskStatusChange"
		setBody(&done, &TaskStatusChange{Id: task.Id, Status: "finished"})

		mb.On("SendMessage", "sched", &done).Return(nil)

		err = ha.HandleMessage(&vm)
		require.NoError(t, err)

		ha.WaitTilIdle()
	})

	n.It("sends a status change if run errors out", func() {
		var vm vega.Message

		body, err := json.Marshal(startMsg)
		require.NoError(t, err)

		vm.ReplyTo = "sched"
		vm.Type = "StartTask"
		vm.Body = body

		var ack vega.Message
		ack.Type = "OpAcknowledged"
		setBody(&ack, &OpAcknowledged{startMsg.OpId})

		mb.On("SendMessage", "sched", &ack).Return(nil)

		var out vega.Message
		out.Type = "TaskStatusChange"
		setBody(&out, &TaskStatusChange{Id: task.Id, Status: "error", Error: "unable to start"})

		exp := errors.New("unable to start")

		mb.On("SendMessage", "sched", &out).Return(nil)
		te.On("Run", task).Return(&th, exp)

		err = ha.HandleMessage(&vm)
		assert.Equal(t, err, exp)
	})

	n.It("sends a status change if wait errors out", func() {
		var vm vega.Message

		body, err := json.Marshal(startMsg)
		require.NoError(t, err)

		vm.ReplyTo = "sched"
		vm.Type = "StartTask"
		vm.Body = body

		var ack vega.Message
		ack.Type = "OpAcknowledged"
		setBody(&ack, &OpAcknowledged{startMsg.OpId})

		mb.On("SendMessage", "sched", &ack).Return(nil)

		var out vega.Message
		out.Type = "TaskStatusChange"
		setBody(&out, &TaskStatusChange{Id: task.Id, Status: "running"})

		exp := errors.New("unable to start")

		mb.On("SendMessage", "sched", &out).Return(nil)
		te.On("Run", task).Return(&th, nil)
		th.On("Wait").Return(exp)

		var done vega.Message
		done.Type = "TaskStatusChange"
		setBody(&done, &TaskStatusChange{Id: task.Id, Status: "failed", Error: "unable to start"})

		mb.On("SendMessage", "sched", &done).Return(nil)

		err = ha.HandleMessage(&vm)
		require.NoError(t, err)

		ha.WaitTilIdle()
	})

	n.It("can stop a task that is running", func() {
		var vm vega.Message

		vm.ReplyTo = "sched"
		vm.Type = "StopTask"
		setBody(&vm, stopMsg)

		var status vega.Message
		status.Type = "OpAcknowledged"
		setBody(&status, &OpAcknowledged{stopMsg.OpId})

		ha.runningTasks["task1"] = &th

		th.On("Stop", false).Return(nil)
		mb.On("SendMessage", "sched", &status).Return(nil)

		err := ha.HandleMessage(&vm)
		require.NoError(t, err)

		ha.WaitTilIdle()
	})

	n.It("sends an error message if the task is unknown", func() {
		var status vega.Message
		status.Type = "OpAcknowledged"
		setBody(&status, &OpAcknowledged{stopMsg.OpId})

		var vm vega.Message

		vm.ReplyTo = "sched"
		vm.Type = "StopTask"
		setBody(&vm, stopMsg)

		var se vega.Message

		se.Type = "StopError"
		setBody(&se, &StopError{OpId: stopMsg.OpId, Id: stopMsg.Id, Error: "unknown task"})

		mb.On("SendMessage", "sched", &status).Return(nil)
		mb.On("SendMessage", "sched", &se).Return(nil)

		err := ha.HandleMessage(&vm)
		require.Equal(t, err, ErrUnknownTask)
	})

	n.It("reports an error if stop errors out", func() {
		var vm vega.Message

		vm.ReplyTo = "sched"
		vm.Type = "StopTask"
		setBody(&vm, stopMsg)

		var status vega.Message
		status.Type = "OpAcknowledged"
		setBody(&status, &OpAcknowledged{stopMsg.OpId})

		ha.runningTasks["task1"] = &th

		fe := errors.New("unable to stop")

		var se vega.Message
		se.Type = "StopError"
		setBody(&se, &StopError{OpId: stopMsg.OpId, Id: stopMsg.Id, Error: fe.Error()})

		th.On("Stop", false).Return(fe)
		mb.On("SendMessage", "sched", &status).Return(nil)
		mb.On("SendMessage", "sched", &se).Return(nil)

		err := ha.HandleMessage(&vm)
		require.Equal(t, err, fe)

		ha.WaitTilIdle()
	})

	n.It("can list the tasks it is running", func() {
		var vm vega.Message

		vm.ReplyTo = "sched"
		vm.Type = "ListTasks"
		setBody(&vm, &ListTasks{OpId: "3"})

		ha.runningTasks["task1"] = &th
		ha.runningTasks["task2"] = &th

		var list vega.Message
		list.Type = "CurrentTasks"
		setBody(&list, &CurrentTasks{"3", []string{"task1", "task2"}})

		mb.On("SendMessage", "sched", &list).Return(nil)

		err := ha.HandleMessage(&vm)
		require.NoError(t, err)
	})

	n.It("sends a generic error message when it can't decode a message", func() {
		var vm vega.Message
		vm.ReplyTo = "sched"
		vm.Type = "NotARealMessageType"

		var em vega.Message
		em.Type = "GenericError"
		setBody(&em,
			&GenericError{
				fmt.Sprintf("%s: %s", ErrUnknownMessage.Error(), vm.Type),
			},
		)

		mb.On("SendMessage", "sched", &em).Return(nil)

		err := ha.HandleMessage(&vm)
		require.Equal(t, err, ErrUnknownMessage)
	})

	n.It("sends a generic error message when it doesn't handle a message", func() {
		var vm vega.Message
		vm.ReplyTo = "sched"
		vm.Type = "TaskStatusChange"
		setBody(&vm, &TaskStatusChange{})

		var em vega.Message
		em.Type = "GenericError"
		setBody(&em,
			&GenericError{
				fmt.Sprintf("%s: %s", ErrUnknownMessage.Error(), vm.Type),
			},
		)

		mb.On("SendMessage", "sched", &em).Return(nil)

		err := ha.HandleMessage(&vm)
		require.Equal(t, err, ErrUnknownMessage)
	})

	n.It("sends host status change messages at an interval", func() {
		var vm vega.Message
		vm.Type = "HostStatusChange"
		setBody(&vm, &HostStatusChange{Status: "online"})

		mb.On("SendMessage", "txn", &vm).Return(nil)

		err := ha.SendHeartbeat("txn")
		require.NoError(t, err)
	})

	n.It("sends the heartbeat with a period", func() {
		ha := NewHostAgent("h1", "txn", 60*time.Millisecond, &mb, &te)
		defer ha.Kill()

		go ha.process()

		var vm vega.Message
		vm.Type = "HostStatusChange"
		setBody(&vm, &HostStatusChange{Status: "online"})

		mb.On("SendMessage", "txn", &vm).Return(nil)

		time.Sleep(100 * time.Millisecond)
	})

	n.It("sends an offline message when shutdown", func() {
		var dis vega.Message
		dis.Type = "HostStatusChange"

		setBody(&dis, &HostStatusChange{Status: "disabled", Host: ha.Id})

		mb.On("SendMessage", "txn", &dis).Return(nil)

		var vm vega.Message
		vm.Type = "HostStatusChange"

		setBody(&vm, &HostStatusChange{Status: "offline", Host: ha.Id})

		mb.On("SendMessage", "txn", &vm).Return(nil)

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

		var vm vega.Message

		body, err := json.Marshal(startMsg)
		require.NoError(t, err)

		vm.ReplyTo = "sched"
		vm.Type = "StartTask"
		vm.Body = body

		var out vega.Message
		out.Type = "TaskStatusChange"
		setBody(&out, &TaskStatusChange{Id: task.Id, Status: "running"})

		mb.On("SendMessage", "sched", &out).Return(nil)

		var ack vega.Message
		ack.Type = "OpAcknowledged"
		setBody(&ack, &OpAcknowledged{startMsg.OpId})

		mb.On("SendMessage", "sched", &ack).Return(nil)

		err = ha.HandleMessage(&vm)
		require.NoError(t, err)

		var dis vega.Message
		dis.Type = "HostStatusChange"

		setBody(&dis, &HostStatusChange{Status: "disabled", Host: ha.Id})

		mb.On("SendMessage", "txn", &dis).Return(nil)

		ha.Shutdown()

		// We need the shutdown code to start
		time.Sleep(10 * time.Millisecond)

		mb.AssertExpectations(t)

		var done vega.Message
		done.Type = "TaskStatusChange"
		setBody(&done, &TaskStatusChange{Id: task.Id, Status: "finished"})

		mb.On("SendMessage", "sched", &done).Return(nil)

		var hcm vega.Message
		hcm.Type = "HostStatusChange"

		setBody(&hcm, &HostStatusChange{Status: "offline", Host: ha.Id})

		mb.On("SendMessage", "txn", &hcm).Return(nil)

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

		var vm vega.Message

		body, err := json.Marshal(startMsg)
		require.NoError(t, err)

		vm.ReplyTo = "sched"
		vm.Type = "StartTask"
		vm.Body = body

		var out vega.Message
		out.Type = "TaskStatusChange"
		setBody(&out, &TaskStatusChange{Id: task.Id, Status: "running"})

		mb.On("SendMessage", "sched", &out).Return(nil)

		var ack vega.Message
		ack.Type = "OpAcknowledged"
		setBody(&ack, &OpAcknowledged{startMsg.OpId})

		mb.On("SendMessage", "sched", &ack).Return(nil)

		err = ha.HandleMessage(&vm)
		require.NoError(t, err)

		var dis vega.Message
		dis.Type = "HostStatusChange"

		setBody(&dis, &HostStatusChange{Status: "disabled", Host: ha.Id})

		mb.On("SendMessage", "txn", &dis).Return(nil)

		ha.Shutdown()

		// We need the shutdown code to start
		time.Sleep(10 * time.Millisecond)

		mb.AssertExpectations(t)

		ha.Shutdown()

		var hcm vega.Message
		hcm.Type = "HostStatusChange"

		setBody(&hcm, &HostStatusChange{Status: "offline", Host: ha.Id})

		mb.On("SendMessage", "txn", &hcm).Return(nil)

		// We need the shutdown code to run again
		time.Sleep(10 * time.Millisecond)

		assert.False(t, sleepTe.finished)

		// In the background, it will eventually finish so we need to put
		// this here.
		var done vega.Message
		done.Type = "TaskStatusChange"
		setBody(&done, &TaskStatusChange{Id: task.Id, Status: "finished"})

		mb.On("SendMessage", "sched", &done).Return(nil)
	})

	n.It("diffs the list of tasks and returns nothing if all tasks present", func() {
		var vm vega.Message

		vm.ReplyTo = "txn"
		vm.Type = "CheckTasks"
		setBody(&vm, &CheckTasks{Tasks: []string{"t1"}})

		var ctl vega.Message
		ctl.Type = "CheckedTaskList"
		setBody(&ctl, &CheckedTaskList{Host: ha.Id})

		mb.On("SendMessage", "txn", &ctl).Return(nil)

		ha.runningTasks["t1"] = &th

		err := ha.HandleMessage(&vm)
		require.NoError(t, err)
	})

	n.It("diffs the list of tasks and returns ones it's missing", func() {
		var vm vega.Message

		vm.ReplyTo = "txn"
		vm.Type = "CheckTasks"
		setBody(&vm, &CheckTasks{Tasks: []string{"t1"}})

		var ctl vega.Message
		ctl.Type = "CheckedTaskList"
		setBody(&ctl, &CheckedTaskList{Host: ha.Id, Missing: []string{"t1"}})

		mb.On("SendMessage", "txn", &ctl).Return(nil)

		err := ha.HandleMessage(&vm)
		require.NoError(t, err)
	})

	n.It("diffs the list of tasks and returns ones it has but the request doesn't", func() {
		var vm vega.Message

		vm.ReplyTo = "txn"
		vm.Type = "CheckTasks"
		setBody(&vm, &CheckTasks{Tasks: []string{"t1"}})

		var ctl vega.Message
		ctl.Type = "CheckedTaskList"
		setBody(&ctl, &CheckedTaskList{Host: ha.Id, Unknown: []string{"t2"}})

		mb.On("SendMessage", "txn", &ctl).Return(nil)

		ha.runningTasks["t1"] = &th
		ha.runningTasks["t2"] = &th

		err := ha.HandleMessage(&vm)
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
