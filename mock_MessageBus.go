package strive

import "github.com/stretchr/testify/mock"

type MockMessageBus struct {
	mock.Mock
}

func (m *MockMessageBus) SendMessage(who string, vm *Message) error {
	ret := m.Called(who, vm)

	r0 := ret.Error(0)

	return r0
}
