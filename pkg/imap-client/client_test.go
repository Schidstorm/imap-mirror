package imap_client

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/hack-pad/hackpadfs/mem"
	"github.com/stretchr/testify/assert"
)

func TestReadState(t *testing.T) {
	state := NewState()
	state.Mailboxes["test"] = &MailboxState{
		SavedLastUid:     1,
		SavedUidValidity: 2,
	}

	buf := bytes.NewBuffer(nil)
	_, err := state.WriteTo(buf)
	assert.NoError(t, err)

	readState := NewState()
	_, err = readState.ReadFrom(buf)
	assert.NoError(t, err)
	assert.Equal(t, state, readState)
}

func TestClientReadState(t *testing.T) {
	fs, err := mem.NewFS()
	assert.NoError(t, err)

	client := NewClient(fs, Config{}, nil)
	err = client.readState()
	assert.NoError(t, err)

	assert.NotNil(t, client.state)
	assert.Equal(t, 0, len(client.state.Mailboxes))
}

func TestClientReadStateFromBackup(t *testing.T) {
	fs, err := mem.NewFS()
	assert.NoError(t, err)

	err = fs.MkdirAll("state", os.ModePerm)
	assert.NoError(t, err)

	assert.NoError(t, writeToFile(fs, "state/.state.json.backup", `{"mailboxes":{
		"test1":{"savedLastUid":1,"savedUidValidity":2}
	}}`))

	client := NewClient(fs, Config{
		StateDir: "state",
	}, nil)
	err = client.readState()
	assert.NoError(t, err)

	assert.NotNil(t, client.state)
	assert.Equal(t, 1, len(client.state.Mailboxes))

	assert.Equal(t, uint32(1), client.state.Mailboxes["test1"].SavedLastUid)
	assert.Equal(t, uint32(2), client.state.Mailboxes["test1"].SavedUidValidity)
}

func TestClientUpdateStateFile(t *testing.T) {
	fs, err := mem.NewFS()
	assert.NoError(t, err)

	client := NewClient(fs, Config{
		StateDir: "state",
	}, nil)
	err = client.updateStateFile()
	assert.NoError(t, err)

	_, err = fs.Stat("state/.state.json")
	assert.NoError(t, err)
}

func TestClientUpdateStateFileBackupGeneration(t *testing.T) {
	fs, err := mem.NewFS()
	assert.NoError(t, err)

	client := NewClient(fs, Config{
		StateDir: "state",
	}, nil)
	err = client.updateStateFile()
	assert.NoError(t, err)
	err = client.updateStateFile()
	assert.NoError(t, err)

	_, err = fs.Stat("state/.state.json")
	assert.NoError(t, err)
	_, err = fs.Stat("state/.state.json.backup")
	assert.NoError(t, err)
}

func TestClientReadStateFromBackupOnFailure(t *testing.T) {
	fs, _ := mem.NewFS()
	_ = fs.MkdirAll("state", os.ModePerm)

	assert.NoError(t, writeToFile(fs, "state/.state.json.backup", `{"mailboxes":{
		"test1":{"savedLastUid":1,"savedUidValidity":2}
	}}`))

	assert.NoError(t, writeToFile(fs, "state/.state.json", `{"mailboxes":{
		"test1":{"savedLastUid":3,"savedUidValidity":4}# intentional parsing error
	}}`))

	client := NewClient(fs, Config{
		StateDir: "state",
	}, nil)
	err := client.readState()
	assert.NoError(t, err)

	assert.NotNil(t, client.state)
	assert.Equal(t, 1, len(client.state.Mailboxes))

	assert.Equal(t, uint32(1), client.state.Mailboxes["test1"].SavedLastUid)
	assert.Equal(t, uint32(2), client.state.Mailboxes["test1"].SavedUidValidity)
}

func writeToFile(fs *mem.FS, path string, data string) error {
	f, err := fs.OpenFile(path, os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return err
	}
	defer f.Close()

	if wf, ok := f.(io.Writer); !ok {
		return errors.New("failed to write to temp file. tempFile is not an io.Writer")
	} else {
		_, err = wf.Write([]byte(data))
		return err
	}
}
