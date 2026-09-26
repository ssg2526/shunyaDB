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
		data:      byteDataWalEntry[totalHeaderSize : totalHeaderSize+int(dataLength)],
	}
}

// readNextWalEntry reads one WAL entry sequentially from file, starting at
// its current offset. It returns (nil, nil) on a clean EOF or a torn/short
// trailing entry (the only place that's expected to happen is at the very
// end of the most recently written segment), so callers should treat a nil
// entry as "stop reading this segment," not as an error.
func readNextWalEntry(file *os.File) (*WAL_Entry, error) {
	header := make([]byte, totalHeaderSize)
	if _, err := io.ReadFull(file, header); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil, nil
		}
		return nil, err
	}

	dataLength := binary.LittleEndian.Uint32(header[24:totalHeaderSize])
	full := make([]byte, totalHeaderSize+int(dataLength))
	copy(full, header)
	if _, err := io.ReadFull(file, full[totalHeaderSize:]); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil, nil
		}
		return nil, err
	}

	walEntry := UnmarshalWalEntry(full)
	if checksum(walEntry.data) != walEntry.checksum {
		// corrupt/torn entry - stop reading this segment here, same as EOF
		return nil, nil
	}

	return walEntry, nil
}

func getCurrSegment(logDir string) (*os.File, uint64) {
	_, err := os.Stat(logDir)

	if os.IsNotExist(err) {
		err := os.MkdirAll(logDir, 0755)
		if err != nil {
			fmt.Println(err)
			panic(err)
		}
	}

	dirEntries, err := filepath.Glob(filepath.Join(logDir, segmentPrefix+"*"+segmentSuffix))
	if err != nil {
		panic(err)
	}
	if len(dirEntries) == 0 {
		filename := getNewSegmentName(0)
		file, err := os.OpenFile(filepath.Join(logDir, filename), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Println(err)
		}
		return file, 0
	}

	filenames := make([]string, len(dirEntries))
	for i, name := range dirEntries {
		filenames[i] = filepath.Base(name)
	}

	sort.Slice(filenames, func(i int, j int) bool {
		return filenames[i] < filenames[j]
	})
	lastFileName := filenames[len(filenames)-1]
	fmt.Println(lastFileName)
	file, err := os.OpenFile(filepath.Join(logDir, lastFileName), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println(err)
	}
	lastLSN := getLastLogSequenceNumber(file)

	return file, lastLSN
}

func getSegmentIndexFromFileName(filename string) int {
	currentSegmentIndex, err := strconv.Atoi(filename[len(segmentPrefix) : len(segmentPrefix)+16])
	if err != nil {
		panic(err)
	}
	return currentSegmentIndex
}

// parseSegmentStartLsn extracts the starting LSN encoded in a segment's
// filename (wal-<016d starting LSN>.log), as set by getNewSegmentName.
func parseSegmentStartLsn(filename string) (uint64, error) {
	return strconv.ParseUint(filename[len(segmentPrefix):len(segmentPrefix)+16], 10, 64)
}

func getLastLogSequenceNumber(file *os.File) uint64 {
	eofOffset, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		fmt.Println(err)
		//handle err
	}
	for step := int64(1); eofOffset-step >= 0; step++ {
		currOffset := eofOffset - step
		file.Seek(currOffset, io.SeekStart)
		readBytes := make([]byte, 1)
		file.Read(readBytes)
		if readBytes[0] == '\n' && step != 1 {
			file.Seek(currOffset+1, io.SeekStart)
			break
		}
	}
	lsnBytesBuf := make([]byte, 8)
	_, err1 := file.Read(lsnBytesBuf)
	if err1 != nil {
		fmt.Println(err1)
		//handle err
	}
	lsn := binary.LittleEndian.Uint64(lsnBytesBuf)
	return lsn
}

func getNewSegmentName(currLsn uint64) string {
	return segmentPrefix + fmt.Sprintf("%016d", currLsn) + segmentSuffix
}

func checksum(data []byte) uint64 {
	return xxhash.Sum64(data)
}
