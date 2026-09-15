package storage

import (
	"time"

	"github.com/ssg2526/shunya/internal/memtable"
	"github.com/ssg2526/shunya/internal/sstable"
)

func (storage *Storage) StartFlushLoop() {
	go func() {
		for memTable := range storage.flushQueue {
			storage.FlushMemTable(memTable)
		}
	}()
}

func (storage *Storage) StartIdleFlushWorker(maxLifetime time.Duration) {
	for {
		select {
		case <-storage.idleFlushTicker.C:
			storage.mu.Lock()
			if storage.activeMem.Size() > 0 && time.Since(storage.activeMem.GetCreationTime()) >= maxLifetime {
				storage.activeMem.Freeze()
				memToflush := storage.activeMem
				storage.activeMem = storage.addMemTable()
				storage.mu.Unlock()
				// TODO: fix this, for correctness as well for high write throught when the writes are having mor vol than flushing SST
				storage.flushQueue <- memToflush
				continue
			}
			storage.mu.Unlock()

		case <-storage.ctx.Done():
			return
		}
	}
}

func (storage *Storage) FlushMemTable(memTable memtable.Memtable) {
	sstable := sstable.OpenSSTable()
	sstable.Flush(memTable)
	//TODO: record the sst file so that reads can use the file
	//TODO: also need to close the file at some point

	storage.mu.Lock()
	defer storage.mu.Unlock()
	for i, mt := range storage.mtQ {
		if mt == memTable {
			storage.mtQ = append(storage.mtQ[:i], storage.mtQ[i+1:]...)
			break
		}
	}
}
