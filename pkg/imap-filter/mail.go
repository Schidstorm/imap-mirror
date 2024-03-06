package imap_filter

import (
	"time"

	"github.com/emersion/go-imap"
	"github.com/sg3des/eml"

	_ "github.com/paulrosania/go-charset/data"
)

type Mail struct {
	From   []Address
	Bcc    []Address
	Cc     []Address
	To     []Address
	Sender []Address

	Subject string
	Date    time.Time
}

type Address struct {
	Name  string
	Email string
}

type mailBuilder struct {
	mail *Mail
}

func buildMail() *mailBuilder {
	return &mailBuilder{
		mail: &Mail{},
	}
}

func (b *mailBuilder) Subject(subject string) *mailBuilder {
	b.mail.Subject = subject
	return b
}

func (b *mailBuilder) From(addresses ...Address) *mailBuilder {
	b.mail.From = append(b.mail.From, addresses...)
	return b
}

func (b *mailBuilder) Bcc(addresses ...Address) *mailBuilder {
	b.mail.Bcc = append(b.mail.Bcc, addresses...)
	return b
}

func (b *mailBuilder) Cc(addresses ...Address) *mailBuilder {
	b.mail.Cc = append(b.mail.Cc, addresses...)
	return b
}

func (b *mailBuilder) To(addresses ...Address) *mailBuilder {
	b.mail.To = append(b.mail.To, addresses...)
	return b
}

func (b *mailBuilder) Sender(addresses ...Address) *mailBuilder {
	b.mail.Sender = append(b.mail.Sender, addresses...)
	return b
}

func (b *mailBuilder) Build() *Mail {
	return b.mail
}

func fromEmlMessage(message *eml.Message) (Mail, error) {
	return Mail{
		From:    fromEmlAddresses(message.From),
		Bcc:     fromEmlAddresses(message.Bcc),
		Cc:      fromEmlAddresses(message.Cc),
		To:      fromEmlAddresses(message.To),
		Sender:  fromEmlAddresses([]eml.Address{message.Sender}),
		Subject: message.Subject,
		Date:    message.Date.UTC(),
	}, nil
}

func fromEmlAddresses(addresses []eml.Address) []Address {
	var result []Address
	for _, addr := range addresses {
		result = append(result, Address{
			Name:  addr.Name(),
			Email: addr.Email(),
		})
	}
	return result
}

func fromImapMessage(message *imap.Message) (Mail, error) {
	return Mail{
		From:    fromImapAddresses(message.Envelope.From),
		Bcc:     fromImapAddresses(message.Envelope.Bcc),
		Cc:      fromImapAddresses(message.Envelope.Cc),
		To:      fromImapAddresses(message.Envelope.To),
		Sender:  fromImapAddresses(message.Envelope.Sender),
		Subject: message.Envelope.Subject,
		Date:    message.Envelope.Date.UTC(),
	}, nil
}

func fromImapAddresses(addresses []*imap.Address) []Address {
	var result []Address
	for _, addr := range addresses {
		result = append(result, Address{
			Name:  addr.PersonalName,
			Email: addr.Address(),
		})
	}
	return result
}

func fromEmlFileBytes(message []uint8) (Mail, error) {
	m, err := eml.Parse(message)
	if err != nil {
		return Mail{}, err
	}

	return fromEmlMessage(&m)
}
