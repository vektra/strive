package strive

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

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
				Command: "echo 'hello'",
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

		assert.Equal(t, "hello\n", ml.Buffer.String())
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

		assert.Equal(t, "hello\n", ml.Buffer.String())
	})

	n.It("sets up a workdir for the task", func() {
		task := &Task{
			Description: &TaskDescription{
				Command: "pwd",
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

		assert.Equal(t, dir+"\n", ml.Buffer.String())
	})

	n.It("pass environment variables to the command", func() {
		task := &Task{
			Description: &TaskDescription{
				Command: "echo $GREETING",
				Env: map[string]string{
					"GREETING": "hello",
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

		assert.Equal(t, "hello\n", ml.Buffer.String())
	})

	n.It("passes the task id as an env var", func() {
		task := &Task{
			Id: "task1",
			Description: &TaskDescription{
				Command: "echo $STRIVE_TASKID",
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

		assert.Equal(t, "task1\n", ml.Buffer.String())
	})

	n.It("pulls down urls into the work dir", func() {
		task := &Task{
			Id: "task1",
			Description: &TaskDescription{
				Command: "echo $STRIVE_TASKID",
				URLs:    []string{"http://test.this/foo"},
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

		assert.Equal(t, "task1\n", ml.Buffer.String())
	})

	n.It("writes the metadata into the work dir", func() {
		task := &Task{
			Description: &TaskDescription{
				Command: "cat metadata.json",
				MetaData: map[string]interface{}{
					"stuff": "is cool",
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

		assert.Equal(t, exp, ml.Buffer.String())
	})

	n.It("writes valid json if metadata is empty", func() {
		task := &Task{
			Description: &TaskDescription{
				Command: "cat metadata.json",
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

		assert.Equal(t, exp, ml.Buffer.String())
	})

	n.Meow()
}
