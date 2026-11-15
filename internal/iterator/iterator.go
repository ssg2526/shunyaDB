// iterator for scanning the keys in paginated way
package iterator

import (
	"github.com/ssg2526/shunya/internal/constants"
)

type Iterator interface {
	SeekGE(key []byte, lsn constants.LsnType)
	Next()
	First()
	Valid() bool
	Key() constants.InternalKey
	Value() []byte
	Close()
}

type Version struct {
	Value     []byte
	Lsn       constants.LsnType
	EntryType constants.EntryType
}

// type VersionedIterator interface {
// 	Valid() bool
// 	Next()
// 	Key() []byte
// 	Versions() []Version
// }
