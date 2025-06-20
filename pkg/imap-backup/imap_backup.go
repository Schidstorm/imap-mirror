package imap_backup

import (
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/emersion/go-imap"
	"github.com/hack-pad/hackpadfs"
	log "github.com/sirupsen/logrus"
)

type FS interface {
	hackpadfs.FS
	hackpadfs.WriteFileFS
	hackpadfs.MkdirAllFS
	hackpadfs.ChtimesFS
}

type Config struct {
	BackupDir string `json:"backupDir" yaml:"backupDir"`
}

type ImapBackup struct {
	fileSystem FS
	backupDir  string
}

var FetchBodySection = imap.BodySectionName{}

func NewImapBackup(fileSystem FS, cfg Config) *ImapBackup {
	return &ImapBackup{
		fileSystem: fileSystem,
		backupDir:  cfg.BackupDir,
	}
}

func (i *ImapBackup) HandleMessage(mailbox string, message *imap.Message) {
	err := i.SaveMessage(mailbox, message, i.fileSystem, i.backupDir)
	if err != nil {
		log.Error(err)
		return
	}

}

func (i *ImapBackup) SaveMessage(mailbox string, message *imap.Message, fs FS, backupDir string) error {
	filePath := path.Join(backupDir, GetPathOfMessage(mailbox, message))
	err := fs.MkdirAll(path.Dir(filePath), os.ModePerm)
	if err != nil {
		return err
	}

	err = (func() error {
		body := message.GetBody(&FetchBodySection)
		bodyBytes, err := io.ReadAll(body)
		if err != nil {
			return err
		}

		return fs.WriteFile(filePath, bodyBytes, os.ModePerm)
	})()

	if err != nil {
		return err
	}

	err = fs.Chtimes(filePath, time.Now(), message.Envelope.Date)
	return err
}

func GetPathOfMessage(mailbox string, message *imap.Message) string {
	envelope := message.Envelope
	fileName := ""

	if envelope != nil {
		fileName = fmt.Sprintf("%s_%s", cropString(envelope.Subject, 200), envelope.MessageId)
	} else {
		fileName = fmt.Sprintf("%d", message.Uid)
	}

	secureString := new(strings.Builder)
	for _, c := range fileName {
		if unicode.IsSpace(c) || c == '/' {
			secureString.WriteRune('_')
		} else {
			secureString.WriteRune(c)
		}
	}

	replaceRegex := regexp.MustCompile(`[^a-zA-Z\d_\-]`)

	return fmt.Sprintf("%s/%s.eml", mailbox, replaceRegex.ReplaceAllString(secureString.String(), "_"))
}

func cropString(in string, max int) string {
	if utf8.RuneCountInString(in) > max {
		return string([]rune(in)[0:max])
	}
	return in
}
