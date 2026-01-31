package sstable

import (
	"bufio"
	"fmt"
	"os"
	"path"

	"github.com/ssg2526/shunya/config"
	constants "github.com/ssg2526/shunya/internal/constants"
	"github.com/ssg2526/shunya/internal/memtable"
)

type SSTable struct {
	sstFile      *os.File
	bufWriter    *bufio.Writer
	headerOffset int
	footerOffset int
	dataOffset   int
}

type SSTBlock struct {
	blockLen   int
	entryCount int
	entries    []*SSTBlockEntry
	checksum   uint64
}

type SSTBlockEntry struct {
	lsn   constants.LsnType
	key   []byte
	value []byte
}

type SSTFooter struct {
	indexOffset int
	indexLen    int
	version     uint16
	magic       [8]byte
}

type SSTIndex struct {
	keyLen    int
	keyOffset int
	key       []byte //this is start key of the block
}

type SSTHeader struct {
}

func OpenSSTable() *SSTable {
	sstFile, err := os.OpenFile(path.Join(config.ShunyaConfigs.SSTableDir, "sstable1"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("open new sstable file err", err)
	}
	sstable := &SSTable{
		sstFile: sstFile,
	}
	sstable.bufWriter = bufio.NewWriterSize(sstFile, config.ShunyaConfigs.SSTWriteBufferSize)

	return sstable
}

func (sstable *SSTable) Flush(memtable memtable.Memtable, fileName string) {
	// sstFile, err := os.OpenFile(path.Join(config.ShunyaConfigs.SSTableDir, fileName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	// if err != nil {
	// 	fmt.Println("open new sstable file err", err)
	// }
	// sstable.bufWriter.Write()
	//TODO: implement

}

func (sstable *SSTable) ReadHeader() {

}

func (sstable *SSTable) ReadFooter() {

}
