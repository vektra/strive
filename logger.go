package strive

import (
	"bytes"
	"io"
)

type closeBuffer struct {
	bytes.Buffer

	name string
}

func (cb *closeBuffer) Close() error {
	return nil
}

type MemLogger struct {
	Buffers map[string]*closeBuffer
}

func (ml *MemLogger) SetupStream(name string, task *Task) (io.WriteCloser, error) {

	if ml.Buffers == nil {
		ml.Buffers = make(map[string]*closeBuffer)
	}

	buf := &closeBuffer{name: name}

	ml.Buffers[name] = buf

	return buf, nil
}
