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
	return client.Terminate()
}

// func (c *Connection) GetClient() *client.Client {
// 	return c.imapClient
// }

func (c *Connection) State() imap.ConnState {
	return c.imapClient.State()
}

func (c *Connection) Status(name string, items []imap.StatusItem) (result *imap.MailboxStatus, err error) {
	_, err = try2simplifyAutoRelogin(c, func(data chan any) error {
		defer close(data)
		result, err = c.imapClient.Status(name, items)
		return err
	})

	return result, err
}

func (c *Connection) Mailbox() *imap.MailboxStatus {
	return c.imapClient.Mailbox()
}

func (c *Connection) Select(name string, readOnly bool) (result *imap.MailboxStatus, err error) {
	_, err = try2simplifyAutoRelogin(c, func(data chan any) error {
		defer close(data)
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
	_, err := try2simplifyAutoRelogin(c, func(data chan any) error {
		defer close(data)
		return c.imapClient.Create(name)
	})
	return err
}

func (c *Connection) Delete(name string) error {
	_, err := try2simplifyAutoRelogin(c, func(data chan any) error {
		defer close(data)
		return c.imapClient.Delete(name)
	})
	return err
}

func (c *Connection) Rename(oldName, newName string) error {
	_, err := try2simplifyAutoRelogin(c, func(data chan any) error {
		defer close(data)
		return c.imapClient.Rename(oldName, newName)
	})
	return err
}

func (c *Connection) List(ref, name string) ([]*imap.MailboxInfo, error) {
	return try2simplifyAutoRelogin(c, func(ch chan *imap.MailboxInfo) error {
		return c.imapClient.List(ref, name, ch)
	})
}

func (c *Connection) UidFetch(seqset *imap.SeqSet, items []imap.FetchItem) ([]*imap.Message, error) {
	return try2simplifyAutoRelogin(c, func(ch chan *imap.Message) error {
		return c.imapClient.UidFetch(seqset, items, ch)
	})
}

func (c *Connection) UidMove(seqset *imap.SeqSet, dest string) error {
	_, err := try2simplifyAutoRelogin(c, func(data chan any) error {
		defer close(data)
		return c.imapClient.UidMove(seqset, dest)
	})
	return err
}

func (c *Connection) Fetch(seqset *imap.SeqSet, items []imap.FetchItem) ([]*imap.Message, error) {
	return try2simplifyAutoRelogin(c, func(data chan *imap.Message) error {
		return c.imapClient.Fetch(seqset, items, data)
	})
}

func (c *Connection) Idle(stop <-chan struct{}, opts *client.IdleOptions) error {
	_, err := try2simplifyAutoRelogin(c, func(data chan any) error {
		defer close(data)
		return c.imapClient.Idle(stop, opts)
	})
	return err
}

func try2simplifyAutoRelogin[T any](c *Connection, f func(chan T) error) ([]T, error) {
	res, err := try2simplify(f)
	if err == client.ErrNotLoggedIn {
		c.imapClient.Terminate()
		err = c.Open()
		if err == nil {
			return try2simplifyAutoRelogin(c, f)
		}
	}

	return res, err
}

func try2simplify[T any](f func(chan T) error) ([]T, error) {
	resultChannel := make(chan T, 10)
	done := make(chan error, 1)
	go func() {
		done <- f(resultChannel)
	}()

	var resultList []T
	for item := range resultChannel {
		resultList = append(resultList, item)
	}

	return resultList, <-done
}

func (c Connection) GetUpdates() chan<- client.Update {
	return c.imapClient.Updates
}

func (c *Connection) SetUpdates(ch chan<- client.Update) {
	c.imapClient.Updates = ch
}
