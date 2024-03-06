package imap_client

import (
	"encoding/json"
	"io"
)

type State struct {
	Mailboxes MailboxStateCollection `json:"mailboxes"`
}

func NewState() *State {
	return &State{MailboxStateCollection{}}
}

func (s State) WriteTo(writer io.Writer) (int64, error) {
	data, err := json.Marshal(&s)
	if err != nil {
		return 0, err
	}

	n, err := writer.Write(data)
	if err != nil {
		return 0, err
	}

	return int64(n), nil
}

func (s *State) ReadFrom(reader io.Reader) (int64, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return 0, err
	}

	return int64(len(data)), json.Unmarshal(data, s)
}

type MailboxState struct {
	SavedLastUid     uint32 `json:"savedLastUid"`
	SavedUidValidity uint32 `json:"savedUidValidity"`
}

type MailboxStateCollection map[string]*MailboxState

func (m *MailboxStateCollection) Mailbox(name string) *MailboxState {
	if state, ok := (*m)[name]; ok {
		return state
	}

	(*m)[name] = &MailboxState{
		SavedLastUid:     0,
		SavedUidValidity: 0,
	}

	return m.Mailbox(name)
}

func (m *MailboxStateCollection) HasMailbox(name string) bool {
	_, ok := (*m)[name]
	return ok
}
