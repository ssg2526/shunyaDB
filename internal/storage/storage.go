package storage

import (
	"github.com/ssg2526/shunya/config"
	constants "github.com/ssg2526/shunya/internal/constants"
	"github.com/ssg2526/shunya/internal/memtable"
	"github.com/ssg2526/shunya/internal/wal"
)

type Storage struct {
	mtQ []memtable.Memtable
	wal *wal.WAL
}

func (storage *Storage) AppendToWal(commandData []byte) constants.LsnType {
	return storage.wal.AppendToWal(commandData)
}

func InitStorage() *Storage {
	var memtableObj memtable.Memtable

	switch config.ShunyaConfigs.MemTableType {
	case "skiplist":
		memtableObj = memtable.NewMemtable(memtable.SKIPLIST)
	default:
		memtableObj = memtable.NewMemtable(memtable.SKIPLIST)
	}

	storage := &Storage{
		mtQ: []memtable.Memtable{memtableObj},
		wal: wal.InitWal(),
	}

	return storage

}

func (storage *Storage) Get(key []byte) string {
	//TODO: check in mutable mem first then check in immutable mem and if not present move to sstables
	storage.mtQ[0].Get(string(key))
	return ""
}

func (storage *Storage) Put(key []byte, value []byte, lsn constants.LsnType) string {
	//TODO: directly put to memtable
	storage.mtQ[0].Put(string(key), string(value), lsn, constants.PutEntry)
	return "OK"
}

func (storage *Storage) Del(key []byte, lsn constants.LsnType) string {
	storage.mtQ[0].Put(string(key), "", lsn, constants.DelEntry)
	return "OK"
}
