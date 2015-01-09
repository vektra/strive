package strive

import "github.com/stretchr/testify/mock"

type MockTaskHandle struct {
	mock.Mock
}

func (m *MockTaskHandle) Wait() error {
	ret := m.Called()

	r0 := ret.Error(0)

	return r0
}
func (m *MockTaskHandle) Stop(force bool) error {
	ret := m.Called(force)

	r0 := ret.Error(0)

	return r0
}
