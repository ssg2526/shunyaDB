package contsants

type EntryType uint8

const (
	PutEntry EntryType = iota
	DelEntry
)

type LsnType uint64
