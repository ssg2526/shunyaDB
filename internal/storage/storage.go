package storage

import (
	"encoding/binary"
	"sync"

	"github.com/ssg2526/shunya/config"
	constants "github.com/ssg2526/shunya/internal/constants"
	"github.com/ssg2526/shunya/internal/memtable"
	"github.com/ssg2526/shunya/internal/wal"
)

type Storage struct {
	mu         sync.Mutex
	mtQ        []memtable.Memtable
	activeMem  memtable.Memtable
	flushQueue chan memtable.Memtable
	wal        *wal.WAL
}

const (
	MEM_TABLE_FLUSH_SIZE = 417792
)

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
		mtQ:        []memtable.Memtable{memtableObj},
		activeMem:  memtableObj,
		flushQueue: make(chan memtable.Memtable, config.ShunyaConfigs.FlushQueueSize),
		wal:        wal.InitWal(),
	}
	return storage
}

func (storage *Storage) addMemTable() memtable.Memtable {
	var memtableObj memtable.Memtable

	switch config.ShunyaConfigs.MemTableType {
	case "skiplist":
		memtableObj = memtable.NewMemtable(memtable.SKIPLIST)
	default:
		memtableObj = memtable.NewMemtable(memtable.SKIPLIST)
	}
	storage.mtQ = append(storage.mtQ, memtableObj)
	return memtableObj
}

func (storage *Storage) Get(key []byte, lsn constants.LsnType) []byte {
	//TODO: check in mutable mem first then check in immutable mem and if not present move to sstables
	data := storage.activeMem.Get(key, lsn)
	if data != nil {
		return data
	}
	if data == nil {
		for i := 0; i < len(storage.mtQ); i++ {
			if storage.mtQ[i] != storage.activeMem {
				data = storage.mtQ[i].Get(key, lsn)
				if data != nil {
					return data
				}
			}
		}
	}

	return []byte("")
}

// TODO: need to implememt soft throttling and hard throttling
func (storage *Storage) Put(key []byte, value []byte, lsn constants.LsnType) []byte {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	if storage.activeMem.Size() > MEM_TABLE_FLUSH_SIZE {
		storage.activeMem.Freeze()
		memToflush := storage.activeMem
		storage.activeMem = storage.addMemTable()
		// TODO: fix this, for correctness as well
		storage.flushQueue <- memToflush
	}
	targetMem := storage.activeMem
	targetMem.Put(key, value, lsn, constants.PutEntry)
	return []byte("OK")
}

func (storage *Storage) Del(key []byte, lsn constants.LsnType) []byte {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	if storage.activeMem.Size() > MEM_TABLE_FLUSH_SIZE {
		storage.activeMem.Freeze()
		storage.activeMem = storage.addMemTable()
	}
	targetMem := storage.activeMem
	targetMem.Put(key, nil, lsn, constants.PutEntry)
	return []byte("OK")
}

func (storage *Storage) Range(key []byte, limit int, lsnSnapshot constants.LsnType) [][]byte {
	it := storage.mtQ[0].NewIterator(lsnSnapshot)
	res := [][]byte{}
	it.Seek(key)
	for it.Valid() && limit > 0 {
		key := it.Key()
		val := it.Value()
		keyLen := len(key)
		valLen := len(val)
		kvBuf := make([]byte, len(key)+len(val)+8)
		binary.LittleEndian.PutUint32(kvBuf, uint32(keyLen))
		binary.LittleEndian.PutUint32(kvBuf, uint32(valLen))
		copy(kvBuf[8:], key)
		copy(kvBuf[8+keyLen:], val)
		res = append(res, kvBuf)
		it.Next()
		limit--
	}
	return res
}
