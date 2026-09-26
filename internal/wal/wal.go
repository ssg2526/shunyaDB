package wal

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ssg2526/shunya/config"
	constants "github.com/ssg2526/shunya/internal/constants"
)

const (
	segmentPrefix = "wal-"
	segmentSuffix = ".log"
)

type WAL struct {
	logDir            string
	currSegment       *os.File
	bufWriter         *bufio.Writer
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
	fmt.Println(config.ShunyaConfigs.WALDir)
	currSegmentFile, lastLSN := getCurrSegment(config.ShunyaConfigs.WALDir)
	fmt.Printf("currseg, lastLsn = %v,%v\n", currSegmentFile, lastLSN)
	bufWriter := bufio.NewWriterSize(currSegmentFile, config.ShunyaConfigs.WALWriteBufferSize)
	currSegmentOffset, err := currSegmentFile.Seek(0, io.SeekEnd)
	ctx, cancel := context.WithCancel(context.Background())
	if err != nil {
		fmt.Println(err)
	}

	wal := &WAL{
		logDir:            config.ShunyaConfigs.WALDir,
		shouldFsync:       config.ShunyaConfigs.WALShouldFsync,
		bufSyncTicker:     time.NewTicker(time.Duration(config.ShunyaConfigs.WALBufSyncIntervalMillis) * time.Millisecond),
		maxSegmentSize:    config.ShunyaConfigs.WALMaxSegmentSize,
		currSegment:       currSegmentFile,
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

func (wal *WAL) AppendToWal(commandData []byte) constants.LsnType {
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

	wal.rotateWalSegmentIfRequired(walEntryByteLength, newLsn)
	if _, err := wal.bufWriter.Write(byteDataWalEntry); err != nil {
		if err != nil {
			// fmt.Println(err)
		}
		// handle err
	}
	wal.currSegmentOffset += walEntryByteLength
	return constants.LsnType(newLsn)
}

func (wal *WAL) ReplayWal(sinceLsn constants.LsnType, apply func(lsn constants.LsnType, data []byte) error) error {
	dirEntries, err := os.ReadDir(wal.logDir)
	if err != nil {
		return err
	}

	var filenames []string
	for _, entry := range dirEntries {
		name := entry.Name()
		if strings.HasPrefix(name, segmentPrefix) && strings.HasSuffix(name, segmentSuffix) {
			filenames = append(filenames, name)
		}
	}

	startIdx := 0
	for i, filename := range filenames {
		startLsn, err := parseSegmentStartLsn(filename)
		if err != nil {
			return err
		}
		if constants.LsnType(startLsn) <= sinceLsn {
			startIdx = i
		} else {
			break
		}
	}

	for _, filename := range filenames[startIdx:] {
		if err := wal.replaySegment(filepath.Join(wal.logDir, filename), sinceLsn, apply); err != nil {
			return err
		}
	}
	return nil
}

func (wal *WAL) replaySegment(path string, sinceLsn constants.LsnType, apply func(lsn constants.LsnType, data []byte) error) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	for {
		walEntry, err := readNextWalEntry(file)
		if err != nil {
			return err
		}
		if walEntry == nil {
			return nil
		}

		if constants.LsnType(walEntry.lsn) > sinceLsn {
			if err := apply(constants.LsnType(walEntry.lsn), walEntry.data); err != nil {
				return err
			}
		}
	}
}

func (wal *WAL) rotateWalSegmentIfRequired(size int, newLsn uint64) {
	if wal.maxSegmentSize-wal.currSegmentOffset < size {
		wal.rotateWalSegment(newLsn)
	}
}

func (wal *WAL) rotateWalSegment(newLsn uint64) {
	if wal.bufWriter != nil {
		if err := wal.bufWriter.Flush(); err != nil {
			fmt.Println("flush error:", err)
		}
	}

	if wal.currSegment != nil {
		if errClose := wal.currSegment.Close(); errClose != nil {
			fmt.Println("closing segment err", errClose)
		}
	}

	newSegmentFile := getNewSegmentName(newLsn)
	file, err := os.OpenFile(filepath.Join(wal.logDir, newSegmentFile), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("open new segment file err", err)
	}

	wal.bufWriter = bufio.NewWriterSize(file, config.ShunyaConfigs.WALWriteBufferSize)
	// wal.bufWriter.Reset(file) // could be another option
	wal.currSegment = file
	wal.currSegmentOffset = 0
}

func (wal *WAL) syncWalBufferToDisk() {
	for {
		select {
		case <-wal.bufSyncTicker.C:
			wal.mu.Lock()
			err := wal.bufWriter.Flush()
			wal.mu.Unlock()
			if err != nil {
				fmt.Println(err)
				//handle err
			}
		case <-wal.ctx.Done():
			return
		}
	}
}
