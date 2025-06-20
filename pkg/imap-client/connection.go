package imap_client

import (
	"errors"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

type ConnectionParams struct {
	ImapAddr     string
	ImapUsername string
	ImapPassword string
}

type Connection struct {
	imapClient *client.Client
	params     ConnectionParams
}

func NewConnection(params ConnectionParams) *Connection {
	return &Connection{
		params: params,
	}
}

func (c *Connection) Open() error {
	if c.imapClient != nil {
		c.Close()
	}

	imapClient, err := client.DialTLS(c.params.ImapAddr, nil)
	if err != nil {
		return err
	}
	c.imapClient = imapClient

	return c.imapClient.Login(c.params.ImapUsername, c.params.ImapPassword)
}

func (c *Connection) Close() error {
	client := c.imapClient
	if client == nil {
		return nil
	}

	c.imapClient = nil
	client.Logout()
	return client.Close()
}

// func (c *Connection) GetClient() *client.Client {
// 	return c.imapClient
// }

func (c *Connection) State() imap.ConnState {
	return c.imapClient.State()
}

func (c *Connection) Status(name string, items []imap.StatusItem) (result *imap.MailboxStatus, err error) {
	err = c.call(func() error {
		result, err = c.imapClient.Status(name, items)
		return err
	})

	return result, err
}

func (c *Connection) Mailbox() *imap.MailboxStatus {
	return c.imapClient.Mailbox()
}

func (c *Connection) Select(name string, readOnly bool) (result *imap.MailboxStatus, err error) {
	err = c.call(func() error {
		result, err = c.imapClient.Select(name, readOnly)
		return err
	})
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, errors.New("failed to select mailbox")
	}

	return result, nil
}

func (c *Connection) Create(name string) error {
	return c.call(func() error {
		return c.imapClient.Create(name)
	})
}

func (c *Connection) Delete(name string) error {
	return c.call(func() error {
		return c.imapClient.Delete(name)
	})
}

func (c *Connection) Rename(oldName, newName string) error {
	return c.call(func() error {
		return c.imapClient.Rename(oldName, newName)
	})
}

func (c *Connection) List(ref, name string, ch chan *imap.MailboxInfo) error {
	return c.call(func() error {
		return c.imapClient.List(ref, name, ch)
	})
}

func (c *Connection) UidFetch(seqset *imap.SeqSet, items []imap.FetchItem, ch chan *imap.Message) error {
	return c.call(func() error {
		return c.imapClient.UidFetch(seqset, items, ch)
	})
}

func (c *Connection) UidMove(seqset *imap.SeqSet, dest string) error {
	return c.call(func() error {
		return c.imapClient.UidMove(seqset, dest)
	})
}

func (c *Connection) Fetch(seqset *imap.SeqSet, items []imap.FetchItem, ch chan *imap.Message) error {
	return c.call(func() error {
		return c.imapClient.Fetch(seqset, items, ch)
	})
}

func (c *Connection) Idle(stop <-chan struct{}, opts *client.IdleOptions) error {
	return c.call(func() error {
		return c.imapClient.Idle(stop, opts)
	})
}

func (c *Connection) call(f func() error) error {
	err := f()
	if err == client.ErrNotLoggedIn {
		err = c.Open()
		if err == nil {
			return c.call(f)
		}
	}
	if err != nil {
		return err
	}

	return nil
}

func (c Connection) GetUpdates() chan<- client.Update {
	return c.imapClient.Updates
}

func (c *Connection) SetUpdates(ch chan<- client.Update) {
	c.imapClient.Updates = ch
}
