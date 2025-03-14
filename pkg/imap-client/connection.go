package imap_client

import "github.com/emersion/go-imap/client"

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

	err = imapClient.Login(c.params.ImapUsername, c.params.ImapPassword)
	if err != nil {
		return err
	}

	c.imapClient = imapClient
	return nil
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

func (c *Connection) GetClient() *client.Client {
	return c.imapClient
}
