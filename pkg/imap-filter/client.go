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
	filters      []Filter
	toDeleteList map[filterTodoKey][]uint32
}

type filterTodoKey struct {
	Kind          FilterResultKind
	TargetMailbox string
	RunInMailbox  string
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
	if result != FilterResultAccept {
		log.Infof("rejecting message %d", message.Uid)
		f.addUidToDeleteList(filterTodoKey{
			Kind:          result.Kind,
			TargetMailbox: result.Target,
			RunInMailbox:  mailbox,
		}, mailbox, message.Uid)
	}
}

func (f *FilterClient) addUidToDeleteList(key filterTodoKey, mailbox string, uid uint32) {
	if f.toDeleteList == nil {
		f.toDeleteList = make(map[filterTodoKey][]uint32)
	}
	f.toDeleteList[key] = append(f.toDeleteList[key], uid)
}

func (f *FilterClient) ProcessDeletions(c *client.Client) {
	for todoKey, uids := range f.toDeleteList {
		if len(uids) == 0 {
			continue
		}

		log.Infof("processing %d messages from %s with kind %s and target '%s'", len(uids), todoKey.RunInMailbox, todoKey.Kind, todoKey.TargetMailbox)
		err := f.deleteMessages(c, todoKey, uids)
		if err != nil {
			log.WithError(err).Error("failed to delete messages")
		}
	}

	f.toDeleteList = nil
}

func (f *FilterClient) deleteMessages(c *client.Client, todoKey filterTodoKey, messages []uint32) error {
	if todoKey.RunInMailbox == "Trash" {
		return nil
	}

	if todoKey.Kind == FilterResultKindNoop {
		return nil
	}

	_, err := c.Select(todoKey.RunInMailbox, false)
	if err != nil {
		return err
	}

	msgSeq := new(imap.SeqSet)
	for _, uid := range messages {
		msgSeq.AddNum(uid)
	}

	if todoKey.Kind == FilterResultKindDelete {
		return c.UidMove(msgSeq, "Spam.Shit")
	} else if todoKey.Kind == FilterResultKindMove {
		return c.UidMove(msgSeq, todoKey.TargetMailbox)
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
