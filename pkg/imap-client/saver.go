package imap_client

import (
	"fmt"
	"github.com/emersion/go-imap"
	"github.com/hirochachacha/go-smb2"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

func (c *Client) SaveMessage(mailbox string, message *imap.Message, fs *smb2.Share, backupDir string) error {
	filePath := path.Join(backupDir, GetPathOfMessage(mailbox, message))
	err := fs.MkdirAll(path.Dir(filePath), os.ModePerm)
	if err != nil {
		return err
	}

	err = (func() error {
		fd, err := fs.OpenFile(filePath, os.O_CREATE|os.O_RDWR, os.ModePerm)
		if err != nil {
			return err
		}
		defer fd.Close()

		body := message.GetBody(&FetchBodySection)

		_, err = io.Copy(fd, body)
		if err != nil {
			return err
		}

		return nil
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

	replaceRegex := regexp.MustCompile("[^a-zA-Z\\d_\\-]")

	return fmt.Sprintf("%s/%s.eml", mailbox, replaceRegex.ReplaceAllString(secureString.String(), "_"))
}

func cropString(in string, max int) string {
	if utf8.RuneCountInString(in) > max {
		return string([]rune(in)[0:max])
	}
	return in
}
