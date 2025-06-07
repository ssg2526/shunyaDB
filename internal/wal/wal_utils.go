package wal

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/cespare/xxhash/v2"
)

const (
	lsnSize         = 8
	timestampSize   = 8
	checksumSize    = 8
	lengthSize      = 4
	totalHeaderSize = lsnSize + lengthSize + checksumSize + timestampSize
)

func MarshalWalEntry(walEntry *WAL_Entry) ([]byte, int) {

	// is it possible to not use this make and use a sync.Pool with a bigger size
	walDataLen := len(walEntry.data)
	marshalled := make([]byte, totalHeaderSize+walDataLen)

	binary.LittleEndian.PutUint64(marshalled[:8], walEntry.lsn)
	binary.LittleEndian.PutUint64(marshalled[8:16], uint64(walEntry.timestamp))
	binary.LittleEndian.PutUint64(marshalled[16:24], walEntry.checksum)
	binary.LittleEndian.PutUint32(marshalled[24:totalHeaderSize], uint32(walEntry.length))

	copy(marshalled[totalHeaderSize:], walEntry.data)

	return marshalled, totalHeaderSize + walDataLen
}

func UnmarshalWalEntry(byteDataWalEntry []byte) *WAL_Entry {

	dataLength := int32(binary.LittleEndian.Uint32(byteDataWalEntry[24:totalHeaderSize]))

	return &WAL_Entry{
		lsn:       binary.LittleEndian.Uint64(byteDataWalEntry[:8]),
		timestamp: int64(binary.LittleEndian.Uint64(byteDataWalEntry[8:16])),
		checksum:  binary.LittleEndian.Uint64(byteDataWalEntry[16:24]),
		length:    dataLength,
		data:      byteDataWalEntry[totalHeaderSize:dataLength],
	}
}

func getCurrSegment(logDir string) (*os.File, int, uint64) {
	_, err := os.Stat(logDir)

	if os.IsNotExist(err) {
		os.Mkdir(logDir, 0755)
	}

	dirEntries, err := filepath.Glob(filepath.Join(logDir, segmentPrefix+"*"+segmentPrefix))

	if err != nil {
		panic(err)
	}
	if len(dirEntries) != 0 {
		filename := getNewSegmentName(0)
		file, err := os.OpenFile(filepath.Join(logDir, filename), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return file, 0, 0
		}
	}

	sort.Slice(dirEntries, func(i int, j int) bool {
		return dirEntries[i] < dirEntries[j]
	})
	lastFileName := dirEntries[len(dirEntries)-1]
	file, err := os.OpenFile(filepath.Join(logDir, lastFileName), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		currentSegmentIndex := getSegmentIndexFromFileName(lastFileName)
		lastLSN := getLastLogSequenceNumber(file) // handle error

		return file, currentSegmentIndex, lastLSN
	}
	return nil, 0, 0
}

func getSegmentIndexFromFileName(filename string) int {
	currentSegmentIndex, err := strconv.Atoi(filename[len(segmentPrefix) : len(segmentPrefix)+16])
	if err != nil {
		panic(err)
	}
	return currentSegmentIndex
}

func getLastLogSequenceNumber(file *os.File) uint64 {
	eofOffset, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		//handle err
	}
	for step := int64(1); eofOffset-step >= 0; step++ {
		currOffset := eofOffset - step
		file.Seek(currOffset, io.SeekStart)
		readBytes := make([]byte, 1)
		file.Read(readBytes)
		if readBytes[0] == '\n' && step != 1 {
			break
		}
	}
	lsnBytesBuf := make([]byte, 8)
	_, err1 := file.Read(lsnBytesBuf)
	if err1 != nil {
		//handle err
	}
	lsn := binary.LittleEndian.Uint64(lsnBytesBuf)
	return lsn
}

func getNewSegmentName(currSegmentIndex int) string {
	return segmentPrefix + fmt.Sprintf("%016d", currSegmentIndex) + segmentSuffix
}

func checksum(data []byte) uint64 {
	return xxhash.Sum64(data)
}
