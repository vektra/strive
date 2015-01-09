package strive

import "github.com/stretchr/testify/mock"

type MockWorkSetup struct {
	mock.Mock
}

func (m *MockWorkSetup) TaskDir(task *Task) (string, error) {
	ret := m.Called(task)

	r0 := ret.Get(0).(string)
	r1 := ret.Error(1)

	return r0, r1
}
func (m *MockWorkSetup) DownloadURL(url string, dir string) error {
	ret := m.Called(url, dir)

	r0 := ret.Error(0)

	return r0
}
