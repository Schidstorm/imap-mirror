package imap_filter

import (
	"slices"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/sg3des/eml"
	"github.com/sirupsen/logrus"
)

var log *logrus.Logger = logrus.New()

type FilterClient struct {
	filters      []Filter
	toDeleteList map[string][]uint32
}

func NewFilterClient(filters ...Filter) *FilterClient {
	return &FilterClient{
		filters: filters,
	}
}

func (f *FilterClient) Init(log *logrus.Logger, _ *client.Client) error {
	failedFilters := []int{}
	for i, filter := range f.filters {
		err := filter.Init()
		if err != nil {
			log.WithError(err).Errorf("failed to init filter %d", i)
			failedFilters = append(failedFilters, i)
		}
	}
	slices.Reverse(failedFilters)

	for _, i := range failedFilters {
		f.filters = append(f.filters[:i], f.filters[i+1:]...)
	}

	return nil
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
	if result == FilterResultReject {
		log.Infof("rejecting message %d", message.Uid)
		f.addUidToDeleteList(mailbox, message.Uid)
	}
}

func (f *FilterClient) addUidToDeleteList(mailbox string, uid uint32) {
	if f.toDeleteList == nil {
		f.toDeleteList = make(map[string][]uint32)
	}
	f.toDeleteList[mailbox] = append(f.toDeleteList[mailbox], uid)
}

func (f *FilterClient) ProcessDeletions(c *client.Client) {
	for mailbox, uids := range f.toDeleteList {
		if len(uids) == 0 {
			continue
		}
		log.Infof("deleting %d messages from %s", len(uids), mailbox)
		err := f.deleteMessages(c, mailbox, uids)
		if err != nil {
			log.WithError(err).Error("failed to delete messages")
		}
	}

	f.toDeleteList = nil
}

func (f *FilterClient) deleteMessages(c *client.Client, mailbox string, messages []uint32) error {
	if mailbox == "Trash" {
		return nil
	}

	_, err := c.Select(mailbox, false)
	if err != nil {
		return err
	}

	deleteSeqSet := new(imap.SeqSet)
	for _, uid := range messages {
		deleteSeqSet.AddNum(uid)
	}

	return c.UidMove(deleteSeqSet, "Spam.Shit")
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
		if result == FilterResultReject {
			return FilterResultReject
		}
	}

	return FilterResultAccept
}
