package iterator

import "github.com/ssg2526/shunya/internal/constants"

type MergeIterator struct {
	minheap *MinHeap
}

func NewMergeIterator(iterators []Iterator) Iterator {

	return &MergeIterator{
		minheap: NewMinHeapFromArray(iterators),
	}
}

func (mIt *MergeIterator) Valid() bool {
	if mIt == nil {
		return false
	}
	return true
}

func (mIt *MergeIterator) SeekGE(key []byte, lsn constants.LsnType) {

}

func (mIt *MergeIterator) First() {

}

func (mIt *MergeIterator) Next() {
	mIt.minheap.GetMin()
}

func (mIt *MergeIterator) Key() constants.InternalKey {
	return mIt.minheap.Peek().Key()
}

func (mIt *MergeIterator) Value() []byte {
	return mIt.minheap.Peek().Value()
}

func (mIt *MergeIterator) Close() {

}
