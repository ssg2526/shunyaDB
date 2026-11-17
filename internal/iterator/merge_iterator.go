package iterator

import (
	"sync"

	"github.com/ssg2526/shunya/internal/constants"
)

type MergeIterator struct {
	minheap   *MinHeap
	iterators []Iterator
}

func NewMergeIterator(iterators []Iterator) Iterator {

	return &MergeIterator{
		iterators: iterators,
	}
}

func (mIt *MergeIterator) Valid() bool {
	if mIt.minheap == nil || mIt.minheap.Size <= 0 || !mIt.minheap.Peek().Valid() {
		return false
	}
	return true
}

func (mIt *MergeIterator) SeekGE(key []byte, lsn constants.LsnType) {

	var wg sync.WaitGroup

	for _, iterator := range mIt.iterators {
		wg.Go(func() {
			iterator.SeekGE(key, lsn)
		})
	}

	wg.Wait()
	mIt.minheap = NewMinHeapFromArray(mIt.iterators)
}

func (mIt *MergeIterator) First() {
	var wg sync.WaitGroup

	for _, iterator := range mIt.iterators {
		wg.Go(func() {
			iterator.First()
		})
	}

	wg.Wait()
	mIt.minheap = NewMinHeapFromArray(mIt.iterators)
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
	for _, iterator := range mIt.iterators {
		if iterator != nil {
			iterator.Close()
		}
	}

	mIt.iterators = nil
	mIt.minheap = nil
}
