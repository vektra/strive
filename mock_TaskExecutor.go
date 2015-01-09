package strive

import "github.com/stretchr/testify/mock"

type MockTaskExecutor struct {
	mock.Mock
}

func (m *MockTaskExecutor) Run(task *Task) (TaskHandle, error) {
	ret := m.Called(task)

	r0 := ret.Get(0).(TaskHandle)
	r1 := ret.Error(1)

	return r0, r1
}
