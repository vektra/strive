package strive

import (
	"bytes"
	"io"
)

type closeBuffer struct {
	bytes.Buffer
}

func (cb *closeBuffer) Close() error {
	return nil
}

type MemLogger struct {
	Buffer closeBuffer
}

func (ml *MemLogger) SetupStream(task *Task) (io.WriteCloser, error) {
	return &ml.Buffer, nil
}
