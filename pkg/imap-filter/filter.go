package imap_filter

type FilterResult int

const (
	FilterResultAccept FilterResult = iota
	FilterResultReject
)

type Filter interface {
	Init() error
	Filter(mailbox string, message *Mail) (FilterResult, error)
	SelectMailboxes() []string
}
