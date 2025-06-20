package imap_filter

import (
	"errors"
	"slices"
	"sync"

	imap_client "git.schidlow.ski/gitea/imap-mirror/pkg/imap-client"
	"github.com/emersion/go-imap"
	"github.com/sg3des/eml"
	log "github.com/sirupsen/logrus"
)

type FilterClient struct {
	filters    []Filter
	client     *imap_client.Connection
	closeChan  chan struct{}
	closedWg   *sync.WaitGroup
	applyTasks chan struct {
		srcMailbox  string
		msguid      uint32
		destMailbox string
	}
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

	fc := &FilterClient{
		filters:   filters,
		closeChan: make(chan struct{}),
		closedWg:  &sync.WaitGroup{},
		applyTasks: make(chan struct {
			srcMailbox  string
			msguid      uint32
			destMailbox string
		}, 1024),
	}

	fc.closedWg.Add(1)
	go fc.filterApplyer()
	return fc
}

func (f *FilterClient) Close() {
	close(f.closeChan)
	f.closedWg.Wait()
}

func (f *FilterClient) filterApplyer() {
	for {
		select {
		case <-f.closeChan:
			f.closedWg.Done()
			return
		case task := <-f.applyTasks:
			mb := f.client.Mailbox()
			if mb.Name != task.srcMailbox {
				_, err := f.client.Select(task.srcMailbox, false)
				if err != nil {
					log.WithError(err).Errorf("failed to select mailbox %s", task.srcMailbox)
					continue
				}
			}

			msgSeq := new(imap.SeqSet)
			msgSeq.AddNum(task.msguid)
			log.Infof("moving message %d from %s to %s", task.msguid, task.srcMailbox, task.destMailbox)
			err := f.client.UidMove(msgSeq, task.destMailbox)
			if err != nil {
				log.WithError(err).Error("failed to move message")
				continue
			}
		}
	}
}

func (f *FilterClient) SetConnection(c *imap_client.Connection) {
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

	msgSeq := new(imap.SeqSet)
	msgSeq.AddNum(message)

	if filterResult.Kind == FilterResultKindDelete {
		f.applyTasks <- struct {
			srcMailbox  string
			msguid      uint32
			destMailbox string
		}{mailbox, message, "Spam.Shit"}
	} else if filterResult.Kind == FilterResultKindMove {
		f.applyTasks <- struct {
			srcMailbox  string
			msguid      uint32
			destMailbox string
		}{mailbox, message, filterResult.Target}
	} else {
		return errors.New("failed to process unknown FilterResultKind")
	}

	return nil
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
