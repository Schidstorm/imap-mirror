package imap_filter

import (
	"errors"
	"slices"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/sg3des/eml"
	"github.com/sirupsen/logrus"
)

var log *logrus.Logger = logrus.New()

type FilterClient struct {
	filters []Filter
	client  *client.Client
}

func NewFilterClient(filters ...Filter) *FilterClient {
	failedFilters := []int{}
	for i, filter := range filters {
		err := filter.Init()
		if err != nil {
			log.WithError(err).Errorf("failed to init filter %d", i)
			failedFilters = append(failedFilters, i)
		}
	}
	slices.Reverse(failedFilters)

	for _, i := range failedFilters {
		filters = append(filters[:i], filters[i+1:]...)
	}

	return &FilterClient{
		filters: filters,
	}
}

func (f *FilterClient) SetClient(c *client.Client) {
	f.client = c
}

func (f *FilterClient) SelectMailboxes() []string {
	mailboxes := []string{}
	for _, filter := range f.filters {
		mailboxes = append(mailboxes, filter.SelectMailboxes()...)
	}
	return mailboxes

}

func (f *FilterClient) HandleMessage(mailbox string, message *imap.Message) {
	result := f.FilterImap(mailbox, message)
	if result != FilterResultAccept {

		f.applyResultToMessage(result, mailbox, message.Uid)
	}
}

func (f *FilterClient) applyResultToMessage(filterResult FilterResult, mailbox string, message uint32) error {
	if filterResult.Kind == FilterResultKindNoop {
		return nil
	}

	if mailbox == "Trash" {
		return nil
	}

	c := f.client
	_, err := c.Select(mailbox, false)
	if err != nil {
		return err
	}

	msgSeq := new(imap.SeqSet)
	msgSeq.AddNum(message)

	if filterResult.Kind == FilterResultKindDelete {
		log.Infof("deleting message %d from %s", message, mailbox)
		return c.UidMove(msgSeq, "Spam.Shit")
	} else if filterResult.Kind == FilterResultKindMove {
		log.Infof("moving message %d from %s to %s", message, mailbox, filterResult.Target)
		return c.UidMove(msgSeq, filterResult.Target)
	} else {
		return errors.New("failed to process unknown FilterResultKind")
	}
}

func (f *FilterClient) FilterImap(mailbox string, imapMessage *imap.Message) FilterResult {
	msg, err := fromImapMessage(imapMessage)
	if err != nil {
		log.WithError(err).Error("failed to convert imap message to mail")
		return FilterResultAccept
	}

	return f.filter(mailbox, &msg)
}

func (f *FilterClient) FilterEml(mailbox string, imapMessage *eml.Message) FilterResult {
	msg, err := fromEmlMessage(imapMessage)
	if err != nil {
		log.WithError(err).Error("failed to convert imap message to mail")
		return FilterResultAccept
	}

	return f.filter(mailbox, &msg)
}

func (f *FilterClient) filter(mailbox string, message *Mail) FilterResult {
	for _, filter := range f.filters {
		result, err := filter.Filter(mailbox, message)
		if err != nil {
			log.WithError(err).Error("failed to filter message")
			continue
		}
		if result != FilterResultAccept {
			return result
		}
	}

	return FilterResultAccept
}
