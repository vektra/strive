package strive

import "github.com/stretchr/testify/mock"

type MockOpIdGenerator struct {
	mock.Mock
}

func (m *MockOpIdGenerator) NextOpId() string {
	ret := m.Called()

	r0 := ret.Get(0).(string)

	return r0
}
