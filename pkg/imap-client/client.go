package imap_client

import "C"
import (
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/hack-pad/hackpadfs"
	log "github.com/sirupsen/logrus"
)

const FetchBatchSize = 100
const FetchBatchWaitTime = 500 * time.Millisecond
const loopLimitTime = 2 * time.Second

var FetchBodySection = imap.BodySectionName{}
var FetchItems = []imap.FetchItem{
	imap.FetchUid,
	imap.FetchEnvelope,
	FetchBodySection.FetchItem(),
}

type HandleMessagePlugin interface {
	HandleMessage(mailbox string, message *imap.Message)
}

type SelectMailboxesPlugin interface {
	SelectMailboxes() []string
}

type FS interface {
	hackpadfs.FS
	hackpadfs.OpenFileFS
	hackpadfs.MkdirAllFS
	hackpadfs.StatFS
	hackpadfs.RenameFS
}

type Config struct {
	ImapAddr          string  `json:"imapAddr" yaml:"imapAddr"`
	ImapUsername      string  `json:"imapUsername" yaml:"imapUsername"`
	ImapPassword      string  `json:"imapPassword" yaml:"imapPassword"`
	StateDir          string  `json:"stateDir" yaml:"stateDir"`
	StateFile         *string `json:"stateFile" yaml:"stateFile"`
	LastMessageOffset uint32  `json:"lastMessageOffset" yaml:"lastMessageOffset"`
}

type Client struct {
	activeConnection  *Connection
	idleConnection    *Connection
	config            Config
	messageHandlers   []HandleMessagePlugin
	state             *State
	stateFS           FS
	stateDirectory    string
	lastMessageOffset uint32
	stateFile         string
}

func NewClient(stateFS FS, cfg Config, messageHandlers []HandleMessagePlugin) *Client {
	stateFile := ".state.json"
	if cfg.StateFile != nil {
		stateFile = *cfg.StateFile
	}

	return &Client{
		stateFS:           stateFS,
		stateDirectory:    cfg.StateDir,
		state:             NewState(),
		config:            cfg,
		messageHandlers:   messageHandlers,
		lastMessageOffset: cfg.LastMessageOffset,
		stateFile:         stateFile,
		activeConnection: NewConnection(ConnectionParams{
			ImapAddr:     cfg.ImapAddr,
			ImapUsername: cfg.ImapUsername,
			ImapPassword: cfg.ImapPassword,
		}),
		idleConnection: NewConnection(ConnectionParams{
			ImapAddr:     cfg.ImapAddr,
			ImapUsername: cfg.ImapUsername,
			ImapPassword: cfg.ImapPassword,
		}),
	}
}

// func (c *Client) GetImapClient() *client.Client {
// 	return c.activeConnection.GetClient()
// }

func (c *Client) open() error {
	err := c.idleConnection.Open()
	if err != nil {
		return err
	}

	return c.activeConnection.Open()
}

func (c *Client) Close() error {
	for _, plugin := range c.messageHandlers {
		if closer, ok := plugin.(io.Closer); ok {
			closer.Close()
		}
	}

	if c.activeConnection != nil {
		c.activeConnection.Close()
	}

	if c.idleConnection != nil {
		c.idleConnection.Close()
	}

	return nil
}

func (c *Client) Open() error {
	return c.open()
}

func (c *Client) Run() error {
	lastLoopRun := time.Now()

	for {
		log.Info("starting loop")
		mailboxes, err := c.listMailboxNames(c.activeConnection)
		if err != nil {
			return err
		}

		err = c.readState()
		if err != nil {
			return err
		}

		for _, mbName := range mailboxes {
			err = c.runOnMailbox(mbName)
			if err != nil {
				log.WithField("mailbox", mbName).Error(err)
				continue
			}
		}

		err = c.waitForMailboxUpdate("INBOX")
		if err != nil {
			log.WithError(err).Error("failed to wait for mailbox update. sleeping for 1 hour")
			time.Sleep(1 * time.Hour)
			continue
		}

		limitCalls(&lastLoopRun)
	}
}

func limitCalls(lastCall *time.Time) {
	time.Sleep(loopLimitTime - time.Since(*lastCall))
	*lastCall = time.Now()
}

func (c *Client) waitForMailboxUpdate(mailbox string) error {
	const logoutTimeout = 1 * time.Minute

	log.WithField("mailbox", mailbox).Info("waiting for mailbox update")
	defer log.WithField("mailbox", mailbox).Info("mailbox updated")

	_, err := c.idleConnection.Select(mailbox, true)
	if err != nil {
		return err
	}

	updateChan := make(chan client.Update, 16)
	c.idleConnection.SetUpdates(updateChan)
	defer func() {
		c.idleConnection.SetUpdates(nil)
		close(updateChan)
	}()

	stopChan := make(chan struct{})
	go func() {
		defer close(stopChan)

		for update := range updateChan {
			switch update.(type) {
			case *client.MailboxUpdate:
				return
			default:
				continue
			}
		}
	}()

	return c.idleConnection.Idle(stopChan, &client.IdleOptions{LogoutTimeout: logoutTimeout})
}

func (c *Client) readState() error {
	stateFilePath, backupFilePath, _ := c.stateFiles()
	c.state = NewState()
	stateDoesNotExists := false

	{ // try to read from stat file
		stateFile, err := c.stateFS.OpenFile(stateFilePath, os.O_RDONLY, os.ModePerm)
		if err == nil {
			defer stateFile.Close()
			_, err = c.state.ReadFrom(stateFile)
			if err == nil {
				return nil
			} else {
				log.WithError(err).Warn("failed to read/parse state file")
			}
		} else {
			if os.IsNotExist(err) {
				stateDoesNotExists = true
				log.WithError(err).Info("state file does not exist")
			} else {
				log.WithError(err).Warn("failed to open state file")
			}
		}
	}

	log.Warn("continuing with backup file")

	{ // try to read from backup file
		backupStateFile, err := c.stateFS.OpenFile(backupFilePath, os.O_RDONLY, os.ModePerm)
		if err == nil {
			defer backupStateFile.Close()
			_, err = c.state.ReadFrom(backupStateFile)
			if err == nil {
				return nil
			} else {
				log.WithError(err).Error("failed to read/parse backup file")
				return err
			}
		} else {
			if os.IsNotExist(err) {
				if stateDoesNotExists {
					log.WithError(err).Info("backup file does not exist. assuming blank state")
					return nil
				}
			}

			log.WithError(err).Error("failed to open backup file")
			return err
		}
	}
}

func (c *Client) runOnMailbox(mailboxName string) error {
	log.WithField("mailbox", mailboxName).Info("processing mailbox")
	if !c.state.Mailboxes.HasMailbox(mailboxName) {
		return c.fetchAllMessages(mailboxName)
	}

	mbStatus, err := c.activeConnection.Status(mailboxName, []imap.StatusItem{imap.StatusUidValidity})
	if err != nil {
		return err
	}

	mbState := c.state.Mailboxes.Mailbox(mailboxName)
	if mbStatus.UidValidity != mbState.SavedUidValidity {
		return c.fetchAllMessages(mailboxName)
	}

	return c.fetchUids(mailboxName, mbState.SavedLastUid)
}

func (c *Client) fetchAllMessages(mailbox string) error {
	mbStatus, err := c.activeConnection.Select(mailbox, true)
	if err != nil {
		return err
	}
	state := c.state.Mailboxes.Mailbox(mailbox)
	state.SavedLastUid = 0
	state.SavedUidValidity = mbStatus.UidValidity

	for i := uint32(1); i < mbStatus.Messages; i += FetchBatchSize {
		err = c.fetchBatched(c.activeConnection, i, FetchBatchSize)
		if err != nil {
			return err
		}

		time.Sleep(FetchBatchWaitTime)
	}

	return nil
}

func (c *Client) fetchUids(mailbox string, uidBegin uint32) error {
	mbStatus, err := c.activeConnection.Select(mailbox, true)
	state := c.state.Mailboxes.Mailbox(mailbox)
	state.SavedUidValidity = mbStatus.UidValidity
	if err != nil {
		return err
	}

	seqset := new(imap.SeqSet)
	offsettedUidBegin := uint32(0)
	if uidBegin >= c.lastMessageOffset {
		offsettedUidBegin = uidBegin - (c.lastMessageOffset - 1)
	}
	seqset.AddRange(offsettedUidBegin, 0)

	messages, err := c.activeConnection.UidFetch(seqset, FetchItems)
	if err != nil {
		return fmt.Errorf("failed to fetch messages: %w", err)
	}

	for _, msg := range messages {
		// skip over first message because it was the last message on the last run
		if c.lastMessageOffset == 0 && state.SavedLastUid == msg.Uid {
			continue
		}

		state.SavedLastUid = msg.Uid
		c.handleMessage(mailbox, msg)
	}

	return nil
}

func (c *Client) fetchBatched(conn *Connection, begin uint32, length uint32) error {
	seqset := new(imap.SeqSet)
	seqset.AddRange(begin, begin+length)

	messages, err := conn.Fetch(seqset, FetchItems)
	if err != nil {
		return fmt.Errorf("failed to fetch messages: %w", err)
	}

	mb := conn.Mailbox()
	mbName := ""
	if mb != nil {
		mbName = mb.Name
	}

	for _, msg := range messages {
		c.handleMessage(mbName, msg)
	}

	return nil
}

func (c *Client) listMailboxNames(conn *Connection) ([]string, error) {
	mailboxes, err := conn.List("", "*")
	if err != nil {
		return nil, fmt.Errorf("failed to list mailboxes: %w", err)
	}

	var mailboxNames []string
	for _, mb := range mailboxes {
		mailboxNames = append(mailboxNames, mb.Name)
	}

	return mailboxNames, nil
}

func (c *Client) handleMessage(mailbox string, message *imap.Message) {
	log := log.WithField("mailbox", mailbox)
	if message != nil && message.Envelope != nil {
		log = log.WithField("subject", message.Envelope.Subject)
		log.Info("received message")
	} else {
		log.Info("skipping message")
		return
	}

	for _, handleMessagePlugin := range c.messageHandlers {
		handleMessagePlugin.HandleMessage(mailbox, message)
	}

	c.state.Mailboxes.Mailbox(mailbox).SavedLastUid = message.Uid

	err := c.updateStateFile()
	if err != nil {
		log.Error(err)
	}
}

func (c *Client) updateStateFile() error {
	stateFilePath, backupFilePath, tempFilePath := c.stateFiles()
	c.stateFS.MkdirAll(c.stateDirectory, os.ModePerm)

	// write to temp file
	{
		tempFile, err := c.stateFS.OpenFile(tempFilePath, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer tempFile.Close()

		if truncFile, ok := tempFile.(hackpadfs.TruncaterFile); ok {
			err = truncFile.Truncate(0)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("failed to write to temp file. tempFile is not a hackpadfs.TruncaterFile")
		}

		if tempFile, ok := tempFile.(io.Writer); ok {
			_, err = c.state.WriteTo(tempFile)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("failed to write to temp file. tempFile is not an io.Writer")
		}
	}

	if _, err := c.stateFS.Stat(stateFilePath); !os.IsNotExist(err) {
		// move state file to backup file
		err := c.stateFS.Rename(stateFilePath, backupFilePath)
		if err != nil {
			return err
		}
	}

	// rename temp file to state file
	err := c.stateFS.Rename(tempFilePath, stateFilePath)
	if err != nil {
		return err
	}

	return nil
}

func (c Client) stateFiles() (stateFile, backupFile, tmpFile string) {
	stateFile = path.Join(c.stateDirectory, c.stateFile)
	backupFile = stateFile + ".backup"
	tmpFile = stateFile + ".tmp"
	return
}

func (c Client) GetConnection() *Connection {
	return c.activeConnection
}
