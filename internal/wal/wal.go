package wal

import (
	"bufio"
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ssg2526/shunya/config"
)

const (
	segmentPrefix = "wal-"
	segmentSuffix = ".log"
)

type WAL struct {
	logDir            string
	currSegment       *os.File
	bufWriter         *bufio.Writer
	currSegmentIndex  int
	currSegmentOffset int
	writeBufSize      int
	shouldFsync       bool
	bufSyncTicker     *time.Ticker
	maxSegmentSize    int
	lastLSN           uint64
	mu                sync.Mutex
	ctx               context.Context
	cancel            context.CancelFunc
}

type WAL_Entry struct {
	lsn       uint64
	length    int32
	checksum  uint64
	timestamp int64
	data      []byte
}

func InitWal() *WAL {

	currSegmentFile, currSegmentIndex, lastLSN := getCurrSegment(config.ShunyaConfigs.WALDir)
	bufWriter := bufio.NewWriterSize(currSegmentFile, config.ShunyaConfigs.WALWriteBufferSize)
	currSegmentOffset, err := currSegmentFile.Seek(0, io.SeekEnd)
	ctx, cancel := context.WithCancel(context.Background())
	if err != nil {

	}

	wal := &WAL{
		logDir:            config.ShunyaConfigs.WALDir,
		shouldFsync:       config.ShunyaConfigs.WALShouldFsync,
		bufSyncTicker:     time.NewTicker(time.Duration(config.ShunyaConfigs.WALBufSyncIntervalMillis)),
		maxSegmentSize:    config.ShunyaConfigs.WALMaxSegmentSize,
		currSegment:       currSegmentFile,
		currSegmentIndex:  currSegmentIndex,
		currSegmentOffset: int(currSegmentOffset),
		bufWriter:         bufWriter,
		writeBufSize:      config.ShunyaConfigs.WALWriteBufferSize,
		lastLSN:           lastLSN,
		ctx:               ctx,
		cancel:            cancel,
	}

	go wal.syncWalBufferToDisk()

	return wal
}

func (wal *WAL) AppendToWal(commandData []byte) {
	newLsn := atomic.AddUint64(&wal.lastLSN, 1)
	walEntry := &WAL_Entry{
		lsn:       newLsn,
		length:    int32(len(commandData)),
		data:      commandData,
		checksum:  checksum(commandData),
		timestamp: time.Now().UnixMilli(),
	}
	byteDataWalEntry, walEntryByteLength := MarshalWalEntry(walEntry)

	wal.mu.Lock()
	defer wal.mu.Unlock()

	wal.rotateWalSegmentIfRequired(walEntryByteLength)
	if _, err := wal.bufWriter.Write(byteDataWalEntry); err != nil {
		// handle err
	}
	wal.currSegmentOffset += walEntryByteLength
}

func (wal *WAL) rotateWalSegmentIfRequired(size int) {
	if wal.maxSegmentSize-wal.currSegmentOffset < size {
		wal.rotateWalSegment()
	}
}

func (wal *WAL) rotateWalSegment() {
	newSegmentIndex := wal.currSegmentIndex + 1
	newSegmentFile := getNewSegmentName(newSegmentIndex)
	file, err := os.OpenFile(filepath.Join(wal.logDir, newSegmentFile), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		// handle
	}
	wal.currSegment = file
	wal.currSegmentOffset = 0
	wal.currSegmentIndex++
}

func (wal *WAL) syncWalBufferToDisk() {
	for {
		select {
		case <-wal.bufSyncTicker.C:
			wal.mu.Lock()
			err := wal.bufWriter.Flush()
			wal.mu.Unlock()
			if err != nil {
				//handle err
			}
		case <-wal.ctx.Done():
			return
		}
	}
}
