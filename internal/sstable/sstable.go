package sstable

import (
	"bufio"
	"fmt"
	"os"
	"path"

	"github.com/ssg2526/shunya/config"
	"github.com/ssg2526/shunya/internal/memtable"
)

type SSTable struct {
	sstFile      *os.File
	bufWriter    *bufio.Writer
	headerOffset int
	footerOffset int
	dataOffset   int
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
	sstFile, err := os.OpenFile(path.Join(config.ShunyaConfigs.SSTableDir, fileName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("open new sstable file err", err)
	}
	//TODO: marshall memtable

}

func (sstable *SSTable) ReadHeader() {

}

func (sstable *SSTable) ReadFooter() {

}
