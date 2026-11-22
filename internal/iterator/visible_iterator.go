package iterator

import (
	"bytes"

	"github.com/ssg2526/shunya/internal/constants"
)

type VisibleIterator struct {
	inner       Iterator
	snapshotLsn constants.LsnType
}

func NewVisibleIterator(inner Iterator, lsn constants.LsnType) *VisibleIterator {
	return &VisibleIterator{
		inner:       inner,
		snapshotLsn: lsn,
	}
}

func (vIt *VisibleIterator) Valid() bool {
	return vIt != nil && vIt.inner.Valid()
}

func (vIt *VisibleIterator) SeekGE(key []byte) {
	vIt.inner.SeekGE(key, vIt.snapshotLsn)
	vIt.settleVisibility(nil)
}

func (vIt *VisibleIterator) Next() {
	if !vIt.Valid() {
		return
	}
	currKey := vIt.Key().Key
	vIt.inner.Next()
	vIt.settleVisibility(currKey)
}

func (vIt *VisibleIterator) First() {
	vIt.inner.First()
	vIt.settleVisibility(nil)
}

func (vIt *VisibleIterator) Key() constants.InternalKey {
	return vIt.inner.Key()
}

func (vIt *VisibleIterator) Value() []byte {
	return vIt.inner.Value()
}

func (vIt *VisibleIterator) Close() {
	if vIt != nil {
		vIt.inner.Close()
	}
}

func (vIt *VisibleIterator) settleVisibility(currKey []byte) {
	for vIt.Valid() {
		k := vIt.Key()
		switch {
		case k.Lsn > vIt.snapshotLsn:
		case bytes.Equal(k.Key, currKey):
		case k.EntryType == constants.DelEntry:
			currKey = k.Key
		default:
			return
		}
		vIt.inner.Next()
	}
}
