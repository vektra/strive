package strive

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"code.google.com/p/goprotobuf/proto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektra/neko"
)

func TestShellExecutor(t *testing.T) {
	n := neko.Start(t)

	var mws MockWorkSetup

	n.CheckMock(&mws.Mock)

	n.It("runs a command", func() {
		task := &Task{
			Description: &TaskDescription{
				Command: proto.String("echo 'hello'"),
			},
		}

		var ml MemLogger

		se := &ShellExecutor{
			Logger:    &ml,
			WorkSetup: &mws,
		}

		mws.On("TaskDir", task).Return("/tmp", nil)

		th, err := se.Run(task)
		require.NoError(t, err)

		err = th.Wait()
		require.NoError(t, err)

		assert.Equal(t, "hello\n", ml.Buffers["output"].String())
	})

	n.It("passes exec args directly if given", func() {
		task := &Task{
			Description: &TaskDescription{
				Exec: []string{"/bin/bash", "-c", "echo 'hello'"},
			},
		}

		var ml MemLogger

		se := &ShellExecutor{
			Logger:    &ml,
			WorkSetup: &mws,
		}

		mws.On("TaskDir", task).Return("/tmp", nil)

		th, err := se.Run(task)
		require.NoError(t, err)

		err = th.Wait()
		require.NoError(t, err)

		assert.Equal(t, "hello\n", ml.Buffers["output"].String())
	})

	n.It("sets up a workdir for the task", func() {
		task := &Task{
			Description: &TaskDescription{
				Command: proto.String("pwd"),
			},
		}

		var ml MemLogger

		se := &ShellExecutor{
			Logger:    &ml,
			WorkSetup: &mws,
		}

		tmpDir, err := ioutil.TempDir("", "test")
		require.NoError(t, err)

		defer os.RemoveAll(tmpDir)

		mws.On("TaskDir", task).Return(tmpDir, nil)

		th, err := se.Run(task)
		require.NoError(t, err)

		err = th.Wait()
		require.NoError(t, err)

		dir, err := filepath.EvalSymlinks(tmpDir)
		require.NoError(t, err)

		assert.Equal(t, dir+"\n", ml.Buffers["output"].String())
	})

	n.It("pass environment variables to the command", func() {
		task := &Task{
			Description: &TaskDescription{
				Command: proto.String("echo $GREETING"),
				Env: []*Variable{
					NewVariable("GREETING", NewStringValue("hello")),
				},
			},
		}

		var ml MemLogger

		se := &ShellExecutor{
			Logger:    &ml,
			WorkSetup: &mws,
		}

		mws.On("TaskDir", task).Return("/tmp", nil)

		th, err := se.Run(task)
		require.NoError(t, err)

		err = th.Wait()
		require.NoError(t, err)

		assert.Equal(t, "hello\n", ml.Buffers["output"].String())
	})

	n.It("passes the task id as an env var", func() {
		task := &Task{
			TaskId: proto.String("task1"),
			Description: &TaskDescription{
				Command: proto.String("echo $STRIVE_TASKID"),
			},
		}

		var ml MemLogger

		se := &ShellExecutor{
			Logger:    &ml,
			WorkSetup: &mws,
		}

		mws.On("TaskDir", task).Return("/tmp", nil)

		th, err := se.Run(task)
		require.NoError(t, err)

		err = th.Wait()
		require.NoError(t, err)

		assert.Equal(t, "task1\n", ml.Buffers["output"].String())
	})

	n.It("pulls down urls into the work dir", func() {
		task := &Task{
			TaskId: proto.String("task1"),
			Description: &TaskDescription{
				Command: proto.String("echo $STRIVE_TASKID"),
				Urls:    []string{"http://test.this/foo"},
			},
		}

		var ml MemLogger

		se := &ShellExecutor{
			Logger:    &ml,
			WorkSetup: &mws,
		}

		mws.On("TaskDir", task).Return("/tmp", nil)
		mws.On("DownloadURL", "http://test.this/foo", "/tmp").Return(nil)

		th, err := se.Run(task)
		require.NoError(t, err)

		err = th.Wait()
		require.NoError(t, err)

		assert.Equal(t, "task1\n", ml.Buffers["output"].String())
	})

	n.It("writes the metadata into the work dir", func() {
		task := &Task{
			Description: &TaskDescription{
				Command: proto.String("cat metadata.json"),
				Metadata: []*Variable{
					NewVariable("stuff", NewStringValue("is cool")),
				},
			},
		}

		var ml MemLogger

		se := &ShellExecutor{
			Logger:    &ml,
			WorkSetup: &mws,
		}

		tmpDir, err := ioutil.TempDir("", "test")
		require.NoError(t, err)

		defer os.RemoveAll(tmpDir)

		mws.On("TaskDir", task).Return(tmpDir, nil)

		th, err := se.Run(task)
		require.NoError(t, err)

		err = th.Wait()
		require.NoError(t, err)

		exp := `{"stuff":"is cool"}` + "\n"

		assert.Equal(t, exp, ml.Buffers["output"].String())
	})

	n.It("writes valid json if metadata is empty", func() {
		task := &Task{
			Description: &TaskDescription{
				Command: proto.String("cat metadata.json"),
			},
		}

		var ml MemLogger

		se := &ShellExecutor{
			Logger:    &ml,
			WorkSetup: &mws,
		}

		tmpDir, err := ioutil.TempDir("", "test")
		require.NoError(t, err)

		defer os.RemoveAll(tmpDir)

		mws.On("TaskDir", task).Return(tmpDir, nil)

		th, err := se.Run(task)
		require.NoError(t, err)

		err = th.Wait()
		require.NoError(t, err)

		exp := `{}` + "\n"

		assert.Equal(t, exp, ml.Buffers["output"].String())
	})

	n.It("setups up a separate stderr stream", func() {
		task := &Task{
			Description: &TaskDescription{
				Command: proto.String("echo 'hello' > /dev/stderr"),
			},
		}

		var ml MemLogger

		se := &ShellExecutor{
			Logger:    &ml,
			WorkSetup: &mws,
		}

		mws.On("TaskDir", task).Return("/tmp", nil)

		th, err := se.Run(task)
		require.NoError(t, err)

		err = th.Wait()
		require.NoError(t, err)

		assert.Equal(t, "hello\n", ml.Buffers["error"].String())
	})

	n.Meow()
}
