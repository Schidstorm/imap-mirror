package imap_filter

type FilterResultKind int

const (
	FilterResultKindNoop FilterResultKind = iota
	FilterResultKindDelete
	FilterResultKindMove
)

func (k FilterResultKind) String() string {
	switch k {
	case FilterResultKindDelete:
		return "delete"
	case FilterResultKindMove:
		return "move"
	default:
		return "noop"
	}
}

func FilterTypeResultFromString(s string) FilterResultKind {
	switch s {
	case "delete":
		return FilterResultKindDelete
	case "move":
		return FilterResultKindMove
	default:
		return FilterResultKindNoop
	}
}

type FilterResult struct {
	Kind   FilterResultKind
	Target string
}

var FilterResultAccept = FilterResult{Kind: FilterResultKindNoop}
var FilterResultReject = FilterResult{Kind: FilterResultKindMove, Target: "Spam/Shit"}

type Filter interface {
	Init() error
	Filter(mailbox string, message *Mail) (FilterResult, error)
	SelectMailboxes() []string
}
