package strive

import (
	"bytes"
	"io"

	backend "github.com/vektra/go-dockerclient"

	"github.com/stretchr/testify/mock"
)

type MockDockerBackend struct {
	mock.Mock
	nextOutputStream *bytes.Buffer
}

func (m *MockDockerBackend) CreateContainer(o backend.CreateContainerOptions) (*backend.Container, error) {
	ret := m.Called(o)
	return ret.Get(0).(*backend.Container), ret.Error(1)
}

func (m *MockDockerBackend) StartContainer(id string, host *backend.HostConfig) error {
	ret := m.Called(id, host)
	return ret.Error(0)
}

func (m *MockDockerBackend) InspectContainer(id string) (*backend.Container, error) {
	ret := m.Called(id)
	return ret.Get(0).(*backend.Container), ret.Error(1)
}

func (m *MockDockerBackend) StopContainer(id string, timeout uint) error {
	ret := m.Called(id, timeout)
	return ret.Error(0)
}

func (m *MockDockerBackend) RestartContainer(id string, timeout uint) error {
	ret := m.Called(id, timeout)
	return ret.Error(0)
}

func (m *MockDockerBackend) AttachToContainer(opts backend.AttachToContainerOptions) error {
	// sess := opts.Success
	// opts.Success = nil

	ret := m.Called(opts)

	return ret.Error(0)
}

func (m *MockDockerBackend) BuildImage(opts backend.BuildImageOptions) error {
	return m.Called(opts).Error(0)
}

func (m *MockDockerBackend) PullImage(opts backend.PullImageOptions, auth backend.AuthConfiguration) error {
	return m.Called(opts, auth).Error(0)
}

func (m *MockDockerBackend) CopyFromContainer(opts backend.CopyFromContainerOptions) error {
	var orig bytes.Buffer

	io.Copy(opts.OutputStream, m.nextOutputStream)
	opts.OutputStream = &orig

	ret := m.Called(opts)

	return ret.Error(0)
}

func (m *MockDockerBackend) RemoveContainer(opts backend.RemoveContainerOptions) error {
	return m.Called(opts).Error(0)
}

func (m *MockDockerBackend) InspectImage(name string) (*backend.Image, error) {
	ret := m.Called(name)

	return ret.Get(0).(*backend.Image), ret.Error(1)
}

func (m *MockDockerBackend) WaitContainer(id string) (int, error) {
	ret := m.Called(id)

	return ret.Int(0), ret.Error(1)
}
