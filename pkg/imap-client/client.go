package imap_client

import "C"
import (
	"fmt"
	"io"
	"os"
	"path"
	"reflect"
	"slices"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/hack-pad/hackpadfs"
	"github.com/sirupsen/logrus"
)

const FetchBatchSize = 100
const FetchBatchWaitTime = 500 * time.Millisecond

var FetchBodySection = imap.BodySectionName{}
var FetchItems = []imap.FetchItem{
	imap.FetchUid,
	imap.FetchEnvelope,
	FetchBodySection.FetchItem(),
}

type Plugin interface {
	Init(log *logrus.Logger, client *client.Client) error
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
	imapClient        *client.Client
	log               *logrus.Logger
	config            Config
	plugins           []Plugin
	state             *State
	stateFS           FS
	stateDirectory    string
	lastMessageOffset uint32
	stateFile         string
}

func NewClient(stateFS FS, cfg Config, messageHandlers []Plugin) *Client {
	stateFile := ".state.json"
	if cfg.StateFile != nil {
		stateFile = *cfg.StateFile
	}

	return &Client{
		stateFS:           stateFS,
		stateDirectory:    cfg.StateDir,
		state:             NewState(),
		config:            cfg,
		plugins:           messageHandlers,
		lastMessageOffset: cfg.LastMessageOffset,
		stateFile:         stateFile,
	}
}

func (c Client) GetImapClient() *client.Client {
	return c.imapClient
}

func (c *Client) open(log *logrus.Logger) error {
	c.log = log

	if c.imapClient != nil {
		c.imapClient.Close()
		c.imapClient.Logout()
	}

	imapClient, err := client.DialTLS(c.config.ImapAddr, nil)
	if err != nil {
		return err
	}
	c.imapClient = imapClient
	log.Info("connected")

	// Login
	if err := imapClient.Login(c.config.ImapUsername, c.config.ImapPassword); err != nil {
		return err
	}
	log.Info("logged in")

	var removePlugins []int
	for i, plugin := range c.plugins {
		err := plugin.Init(log, imapClient)
		if err != nil {
			log.WithError(err).WithField("plugin", reflect.TypeOf(plugin).String()).Error("failed to init plugin. removing plugin from list.")
			removePlugins = append(removePlugins, i)
			continue
		}
	}

	for _, i := range removePlugins {
		c.plugins = append(c.plugins[:i], c.plugins[i+1:]...)
	}

	return nil
}

func (c *Client) Close() error {
	for _, plugin := range c.plugins {
		if closer, ok := plugin.(io.Closer); ok {
			closer.Close()
		}
	}

	if c.imapClient != nil {
		c.imapClient.Logout()
		c.imapClient.Close()
	}

	return nil
}

func (c *Client) Run(log *logrus.Logger) error {

	err := c.open(log)
	if err != nil {
		return err
	}

	mailboxes, err := c.listMailboxNames(c.imapClient)
	if err != nil {
		return err
	}
	log.Info("mailbox listed")

	err = c.readState()
	if err != nil {
		return err
	}

	for _, mbName := range mailboxes {
		err = c.runOnMailbox(mbName)
		if err != nil {
			c.log.WithField("mailbox", mbName).Error(err)
			continue
		}
	}

	return nil
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
				logrus.WithError(err).Warn("failed to read/parse state file")
			}
		} else {
			if os.IsNotExist(err) {
				stateDoesNotExists = true
				logrus.WithError(err).Info("state file does not exist")
			} else {
				logrus.WithError(err).Warn("failed to open state file")
			}
		}
	}

	logrus.Warn("continuing with backup file")

	{ // try to read from backup file
		backupStateFile, err := c.stateFS.OpenFile(backupFilePath, os.O_RDONLY, os.ModePerm)
		if err == nil {
			defer backupStateFile.Close()
			_, err = c.state.ReadFrom(backupStateFile)
			if err == nil {
				return nil
			} else {
				logrus.WithError(err).Error("failed to read/parse backup file")
				return err
			}
		} else {
			if os.IsNotExist(err) {
				if stateDoesNotExists {
					logrus.WithError(err).Info("backup file does not exist. assuming blank state")
					return nil
				}
			}

			logrus.WithError(err).Error("failed to open backup file")
			return err
		}
	}
}

func (c *Client) runOnMailbox(mailboxName string) error {
	c.log.WithField("mailbox", mailboxName).Info("processing mailbox")
	if !c.state.Mailboxes.HasMailbox(mailboxName) {
		return c.fetchAllMessages(mailboxName)
	}

	mbStatus, err := c.imapClient.Status(mailboxName, []imap.StatusItem{imap.StatusUidValidity})
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

func (c *Client) fetchUids(mailbox string, uidBegin uint32) error {
	mbStatus, err := c.imapClient.Select(mailbox, true)
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

	messages := make(chan *imap.Message, 10)
	done := make(chan error, 1)

	go func() {
		done <- c.imapClient.UidFetch(seqset, FetchItems, messages)
	}()

	for msg := range messages {
		// skip over first message because it was the last message on the last run
		if c.lastMessageOffset == 0 && state.SavedLastUid == msg.Uid {
			continue
		}

		state.SavedLastUid = msg.Uid
		c.handleMessage(mailbox, msg)
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
		c.handleMessage(mbName, msg)
	}

	return <-done
}

func (c *Client) listMailboxNames(imapClient *client.Client) ([]string, error) {

	mailboxPluginPresent := false
	var result []string
	for _, plugin := range c.plugins {
		if selectMailboxesPlugin, ok := plugin.(SelectMailboxesPlugin); ok {
			mailboxPluginPresent = true
			result = append(result, selectMailboxesPlugin.SelectMailboxes()...)
		}
	}

	slices.Sort(result)
	for i, name := range result {
		if i > 0 && result[i-1] == name {
			result = append(result[:i], result[i+1:]...)
		}
	}

	if mailboxPluginPresent {
		return result, nil
	}

	mailboxChannel := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func() {
		done <- imapClient.List("", "*", mailboxChannel)
	}()

	var mailboxes []string
	for mb := range mailboxChannel {
		mailboxes = append(mailboxes, mb.Name)
	}

	return mailboxes, <-done
}

func (c *Client) handleMessage(mailbox string, message *imap.Message) {
	log := logrus.WithField("mailbox", mailbox)
	if message != nil && message.Envelope != nil {
		log = log.WithField("subject", message.Envelope.Subject)
	}
	log.Info("received message")

	for _, plugin := range c.plugins {
		if handleMessagePlugin, ok := plugin.(HandleMessagePlugin); ok {
			handleMessagePlugin.HandleMessage(mailbox, message)
		}
	}

	c.state.Mailboxes.Mailbox(mailbox).SavedLastUid = message.Uid

	err := c.updateStateFile()
	if err != nil {
		logrus.Error(err)
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
