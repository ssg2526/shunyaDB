package skiplist

import "github.com/ssg2526/shunya/internal/iterator"

type FlushIterator struct {
	node *Node
}

func (skipList *Skiplist) NewFlushIterator() iterator.VersionedIterator {
	return &FlushIterator{
		node: skipList.head.lvlPtrs[0],
	}
}

func (it *FlushIterator) Valid() bool {
	return it != nil && it.node != nil
}

func (it *FlushIterator) Next() {
	if it.Valid() {
		it.node = it.node.lvlPtrs[0]
	}
}

func (it *FlushIterator) Key() []byte {
	if it.Valid() {
		return it.node.key
	}
	return nil
}

func (it *FlushIterator) Versions() []iterator.Version {
	if !it.Valid() {
		return nil
	}
	versions := make([]iterator.Version, len(it.node.versions))
	for _, v := range it.node.versions {
		versions = append(versions, iterator.Version{Value: v.value, Lsn: v.lsn, EntryType: v.entryType})
	}
	return versions
}
