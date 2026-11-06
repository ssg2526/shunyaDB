package contsants

type EntryType uint8

const (
	PutEntry EntryType = iota
	DelEntry
)

type LookupResult uint8

const (
	KeyNotFound LookupResult = iota
	KeyFound
	KeyDeleted
)

type IteratorMode uint8

const (
	Point IteratorMode = iota
	Range
)

type LsnType uint64
