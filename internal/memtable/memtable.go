package memtable

import (
	constants "github.com/ssg2526/shunya/internal/constants"
)

type Memtable interface {
	Get(key []byte) []byte
	Put(key []byte, value []byte, lsn constants.LsnType, entryType constants.EntryType) []byte
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
