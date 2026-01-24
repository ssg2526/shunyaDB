package memtable

import (
	constants "github.com/ssg2526/shunya/internal/constants"
)

type Memtable interface {
	Get(key string) string
	Put(key string, value string, lsn constants.LsnType, entryType constants.EntryType) string
	Size() int
}

type MemTableType uint8
type MemTableStatus uint8

const (
	MUTABLE MemTableStatus = iota
	FLUSHPENDING
	FLUSHING
)

const (
	SKIPLIST MemTableType = iota
)

type BaseMemtable struct {
	status MemTableStatus
}

func NewMemtable(mtType MemTableType) Memtable {
	switch mtType {
	case SKIPLIST:
		return NewMemSkiplist()
	default:
		return NewMemSkiplist()
	}
}
