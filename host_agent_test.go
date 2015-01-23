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
		ha = NewHostAgent(&mb, &te)
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

		te.On("Run", task).Return(&th, nil)

		err = ha.HandleMessage(&vm)
		require.NoError(t, err)
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

		ha.Wait()
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

		ha.Wait()
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

		ha.Wait()
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

		ha.Wait()
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
		defer ha.Close()

		var vm vega.Message
		vm.Type = "HostStatusChange"
		setBody(&vm, &HostStatusChange{Status: "online"})

		mb.On("SendMessage", "txn", &vm).Return(nil)

		go ha.heartbeatLoop("txn", 50*time.Millisecond)

		time.Sleep(100 * time.Millisecond)
	})

	n.Meow()
}
