package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	imap_backup "github.com/Schidstorm/imap-mirror/pkg/imap-backup"
	imapclient "github.com/Schidstorm/imap-mirror/pkg/imap-client"
	logger "github.com/Schidstorm/imap-mirror/pkg/log"
	"github.com/emersion/go-imap"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

const fetchBatchSize uint32 = 100

type Config struct {
	ImapAddr     string `json:"imapAddr" yaml:"imapAddr"`
	ImapUsername string `json:"imapUsername" yaml:"imapUsername"`
	ImapPassword string `json:"imapPassword" yaml:"imapPassword"`
	BackupDir    string `json:"backupDir" yaml:"backupDir"`
}

type LocalFS struct{}

func (LocalFS) Open(name string) (fs.File, error) {
	return os.Open(name)
}

func (LocalFS) OpenFile(name string, flag int, perm os.FileMode) (fs.File, error) {
	return os.OpenFile(name, flag, perm)
}

func (LocalFS) MkdirAll(name string, perm os.FileMode) error {
	return os.MkdirAll(name, perm)
}

func (LocalFS) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}

func (LocalFS) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

func (LocalFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (LocalFS) Chtimes(name string, atime, mtime time.Time) error {
	return os.Chtimes(name, atime, mtime)
}

func main() {
	logger.Configure(log.InfoLevel)

	root := &cobra.Command{
		Use:   "dump",
		Short: "Dumps all IMAP mailboxes to a local folder structure",
		RunE: func(cmd *cobra.Command, args []string) error {
			configFilePath, err := cmd.Flags().GetString("config.file")
			if err != nil {
				return err
			}

			cfg, err := loadConfig(configFilePath)
			if err != nil {
				return err
			}

			return runDump(cfg)
		},
	}

	flags := root.PersistentFlags()
	flags.String("config.file", "config.yml", "config file path")

	root.AddCommand(&cobra.Command{
		Use:   "config-structure",
		Short: "Print an example config",
		RunE: func(cmd *cobra.Command, args []string) error {
			config := Config{
				ImapAddr:     "imap.example.com:993",
				ImapUsername: "user",
				ImapPassword: "password",
				BackupDir:    "dump",
			}

			configBytes, err := yaml.Marshal(config)
			if err != nil {
				return err
			}

			_, err = cmd.OutOrStdout().Write(configBytes)
			return err
		},
	})

	if err := root.Execute(); err != nil {
		log.Error(err)
	}
}

func loadConfig(configFilePath string) (Config, error) {
	configFileBytes, err := os.ReadFile(configFilePath)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{}
	if err := yaml.Unmarshal(configFileBytes, &cfg); err != nil {
		return Config{}, err
	}

	if cfg.ImapAddr == "" || cfg.ImapUsername == "" || cfg.ImapPassword == "" {
		return Config{}, fmt.Errorf("imapAddr, imapUsername and imapPassword are required")
	}

	if cfg.BackupDir == "" {
		return Config{}, fmt.Errorf("backupDir is required")
	}

	cfg.BackupDir = filepath.Clean(cfg.BackupDir)

	return cfg, nil
}

func runDump(cfg Config) error {
	backupFS := LocalFS{}
	backupClient := imap_backup.NewImapBackup(backupFS, imap_backup.Config{BackupDir: cfg.BackupDir})

	conn := imapclient.NewConnection(imapclient.ConnectionParams{
		ImapAddr:     cfg.ImapAddr,
		ImapUsername: cfg.ImapUsername,
		ImapPassword: cfg.ImapPassword,
	})
	if err := conn.Open(); err != nil {
		return err
	}
	defer conn.Close()

	mailboxes, err := conn.List("", "*")
	if err != nil {
		return err
	}

	for _, mailbox := range mailboxes {
		if mailbox == nil {
			continue
		}

		localMailboxPath := normalizeMailboxPath(mailbox.Name, mailbox.Delimiter)

		if err := dumpMailbox(conn, backupClient, mailbox.Name, localMailboxPath); err != nil {
			log.WithError(err).WithField("mailbox", mailbox.Name).Error("failed to dump mailbox")
			continue
		}
	}

	return nil
}

func dumpMailbox(conn *imapclient.Connection, backupClient *imap_backup.ImapBackup, mailbox string, localMailboxPath string) error {
	status, err := conn.Select(mailbox, true)
	if err != nil {
		return err
	}

	if status.Messages == 0 {
		log.WithField("mailbox", mailbox).Info("mailbox is empty")
		return nil
	}

	log.WithFields(log.Fields{"mailbox": mailbox, "messages": status.Messages}).Info("dumping mailbox")

	for start := uint32(1); start <= status.Messages; start += fetchBatchSize {
		end := start + fetchBatchSize - 1
		if end > status.Messages {
			end = status.Messages
		}

		seqSet := new(imap.SeqSet)
		seqSet.AddRange(start, end)

		messages, err := conn.Fetch(seqSet, imapclient.FetchItems)
		if err != nil {
			return err
		}

		for _, msg := range messages {
			if msg == nil {
				continue
			}

			backupClient.HandleMessage(localMailboxPath, msg)
		}
	}

	return nil
}

func normalizeMailboxPath(mailboxName string, delimiter string) string {
	normalized := strings.TrimSpace(mailboxName)
	if normalized == "" {
		return "INBOX"
	}

	if delimiter != "" && delimiter != "/" {
		normalized = strings.ReplaceAll(normalized, delimiter, "/")
	}

	normalized = strings.Trim(normalized, "/")
	if normalized == "" {
		return "INBOX"
	}

	return normalized
}
