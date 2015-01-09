package strive

import "github.com/stretchr/testify/mock"

import "github.com/vektra/vega"

type MockMessageBus struct {
	mock.Mock
}

func (m *MockMessageBus) SendMessage(who string, vm *vega.Message) error {
	ret := m.Called(who, vm)

	r0 := ret.Error(0)

	return r0
}
