// iterator for scanning the keys in paginated way
package iterator

import (
	constants "github.com/ssg2526/shunya/internal/constants"
)

type Iterator interface {
	Seek(key []byte)
	Next()
	Valid() bool
	Key() []byte
	Value() []byte
}

type Version struct {
	Value     []byte
	Lsn       constants.LsnType
	EntryType constants.EntryType
}

type VersionedIterator interface {
	Valid() bool
	Next()
	Key() []byte
	Versions() []Version
}
