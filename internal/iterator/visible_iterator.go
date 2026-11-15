package iterator

import "github.com/ssg2526/shunya/internal/constants"

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
	vIt.findNextVisible()
}

func (vIt *VisibleIterator) Next() {
	vIt.findNextVisible()
}

func (vIt *VisibleIterator) First() {
	vIt.inner.First()
	vIt.findNextVisible()
}

func (vIt *VisibleIterator) Key() constants.InternalKey {
	return vIt.inner.Key()
}

func (vIt *VisibleIterator) Value() []byte {
	return vIt.inner.Value()
}

func (vIt *VisibleIterator) Close() {

}

func (vIt *VisibleIterator) findNextVisible() {
	//TODO: implement correctly
	for vIt.Valid() {
		if vIt.Key().Lsn > vIt.snapshotLsn {
			vIt.inner.Next()
			continue
		}
		return
	}
}
