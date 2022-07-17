package imap_client

import "C"
import (
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/sirupsen/logrus"
	"os"
	"path"
	"time"
)

const FetchBatchSize = 100
const FetchBatchWaitTime = 500 * time.Millisecond

var FetchBodySection = imap.BodySectionName{}
var FetchItems = []imap.FetchItem{
	imap.FetchUid,
	imap.FetchEnvelope,
	FetchBodySection.FetchItem(),
}

type Client struct {
	imapClient *client.Client
	log        *logrus.Logger
	config     Config
	state      *State
	cifsShare  CifsShare
}

func (c *Client) Open(log *logrus.Logger, config Config) error {
	c.log = log
	c.config = config

	if c.imapClient != nil {
		c.imapClient.Close()
		c.imapClient.Logout()
	}

	imapClient, err := client.DialTLS(config.ImapAddr, nil)
	if err != nil {
		log.Fatal(err)
	}
	c.imapClient = imapClient
	log.Info("connected")

	// Login
	if err := imapClient.Login(config.ImapUsername, config.ImapPassword); err != nil {
		return err
	}
	log.Info("logged in")

	c.cifsShare, err = OpenCifsShare(config)
	if err != nil {
		return err
	}
	log.Info("cifs connected")

	return nil
}

func (c *Client) Close() error {
	if c.imapClient != nil {
		c.imapClient.Logout()
		c.imapClient.Close()
	}

	c.cifsShare.Close()
	return nil
}

func (c *Client) MessageHandler(mailbox string, message *imap.Message) {
	log := logrus.WithField("mailbox", mailbox)
	if message != nil && message.Envelope != nil {
		log = log.WithField("subject", message.Envelope.Subject)
	}
	log.Info("received message")

	err := c.SaveMessage(mailbox, message, c.cifsShare.Share, c.config.BackupDir)
	if err != nil {
		log.Error(err)
		return
	}

	c.state.Mailboxes.Mailbox(mailbox).SavedLastUid = message.Uid

	stateFile, err := c.cifsShare.Share.OpenFile(path.Join(c.config.BackupDir, ".state.json"), os.O_RDONLY|os.O_CREATE, os.ModePerm)
	if err != nil {
		log.Error(err)
	}
	_ = stateFile.Truncate(0)
	_ = c.state.WriteTo(stateFile)
	_ = stateFile.Close()
}

func (c *Client) Run(log *logrus.Logger, config Config) error {
	err := c.Open(log, config)
	if err != nil {
		return err
	}
	defer c.Close()

	mailboxes, err := c.ListMailboxes(c.imapClient)
	if err != nil {
		return err
	}
	log.Info("mailbox listed")

	c.state = NewState()
	stateFile, err := c.cifsShare.Share.OpenFile(path.Join(c.config.BackupDir, ".state.json"), os.O_RDONLY, os.ModePerm)
	if err == nil {
		_ = c.state.ReadFrom(stateFile)
		_ = stateFile.Close()
	}

	for _, mb := range mailboxes {
		err = c.RunOnMailbox(mb)
		if err != nil {
			c.log.WithField("mailbox", mb.Name).Error(err)
			continue
		}
	}

	return nil
}

func (c *Client) RunOnMailbox(mb *imap.MailboxInfo) error {
	c.log.WithField("mailbox", mb.Name).Info("processing mailbox")
	if !c.state.Mailboxes.HasMailbox(mb.Name) {
		return c.FetchAllMessages(mb.Name)
	}

	mbStatus, err := c.imapClient.Status(mb.Name, []imap.StatusItem{imap.StatusUidValidity})
	if err != nil {
		return err
	}

	mbState := c.state.Mailboxes.Mailbox(mb.Name)
	if mbStatus.UidValidity != mbState.SavedUidValidity {
		return c.FetchAllMessages(mb.Name)
	}

	return c.FetchUids(mb.Name, mbState.SavedLastUid)
}

func (c *Client) FetchAllMessages(mailbox string) error {
	mbStatus, err := c.imapClient.Select(mailbox, true)
	if err != nil {
		return err
	}
	state := c.state.Mailboxes.Mailbox(mailbox)
	state.SavedLastUid = 0
	state.SavedUidValidity = mbStatus.UidValidity

	for i := uint32(1); i < mbStatus.Messages; i += FetchBatchSize {
		err = c.fetchBatched(c.imapClient, i, FetchBatchSize)
		if err != nil {
			return err
		}

		time.Sleep(FetchBatchWaitTime)
	}

	return nil
}

func (c *Client) FetchUids(mailbox string, uidBegin uint32) error {
	mbStatus, err := c.imapClient.Select(mailbox, true)
	state := c.state.Mailboxes.Mailbox(mailbox)
	state.SavedUidValidity = mbStatus.UidValidity
	if err != nil {
		return err
	}

	seqset := new(imap.SeqSet)
	seqset.AddRange(uidBegin, 0)

	messages := make(chan *imap.Message, 10)
	done := make(chan error, 1)

	go func() {
		done <- c.imapClient.UidFetch(seqset, FetchItems, messages)
	}()

	for msg := range messages {
		// skip over first message because it was the last message on the last run
		if state.SavedLastUid == msg.Uid {
			continue
		}
		state.SavedLastUid = msg.Uid
		c.MessageHandler(mailbox, msg)
	}

	return <-done
}

func (c *Client) fetchBatched(imapClient *client.Client, begin uint32, length uint32) error {
	seqset := new(imap.SeqSet)
	seqset.AddRange(begin, begin+length)

	messages := make(chan *imap.Message, 10)
	done := make(chan error, 1)

	go func() {
		done <- imapClient.Fetch(seqset, FetchItems, messages)
	}()

	mb := imapClient.Mailbox()
	mbName := ""
	if mb != nil {
		mbName = mb.Name
	}

	for msg := range messages {
		c.MessageHandler(mbName, msg)
	}

	return <-done
}

func (c *Client) ListMailboxes(imapClient *client.Client) ([]*imap.MailboxInfo, error) {
	mailboxChannel := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func() {
		done <- imapClient.List("", "*", mailboxChannel)
	}()

	var mailboxes []*imap.MailboxInfo
	for mb := range mailboxChannel {
		mailboxes = append(mailboxes, mb)
	}

	return mailboxes, <-done
}
