package skiplist

import (
	"bytes"
	"math/rand"

	constants "github.com/ssg2526/shunya/internal/constants"
)

type Node struct {
	key      []byte
	versions []Version
	lvlPtrs  []*Node
}

type Version struct {
	value     []byte
	lsn       constants.LsnType
	entryType constants.EntryType
}

type Skiplist struct {
	threshold float64
	maxLevel  int
	head      *Node
	size      int
}

//TODO: Need to do handle concurrent cases for correctness

func newNode(key []byte, value []byte, lsn constants.LsnType, entryType constants.EntryType, lvl int) *Node {
	versions := []Version{{value: value, lsn: lsn, entryType: entryType}}
	return &Node{key: key, versions: versions, lvlPtrs: make([]*Node, lvl+1)}
}

func NewSkiplist(threshold float64, maxLevel int) *Skiplist {
	return &Skiplist{
		head: &Node{
			key:      nil,
			versions: []Version{{value: nil, lsn: 0}},
			lvlPtrs:  make([]*Node, maxLevel),
		},
		threshold: threshold,
		maxLevel:  maxLevel}
}

func shouldMoveToNextLvl(threshold float64) bool {
	rand_toss := rand.Float64()
	return rand_toss > threshold
}

func (skipList *Skiplist) Get(key []byte, snapshotLsn constants.LsnType) (value []byte) {
	curr := skipList.head

	for i := skipList.maxLevel - 1; i >= 0; i-- {

		for curr.lvlPtrs[i] != nil && bytes.Compare(curr.lvlPtrs[i].key, key) < 0 {
			curr = curr.lvlPtrs[i]
		}

		if curr.lvlPtrs[i] != nil && bytes.Compare(curr.lvlPtrs[i].key, key) == 0 {
			for j := len(curr.lvlPtrs[i].versions) - 1; j >= 0; j-- {
				if curr.lvlPtrs[i].versions[j].lsn <= snapshotLsn {
					if curr.lvlPtrs[i].versions[j].entryType == constants.DelEntry {
						return nil
					}
					return curr.lvlPtrs[i].versions[j].value
				}
			}
			return nil
		}
	}
	return nil
}

// TODO: need to handle concurrent writes yet - choose either single write or lock free model
func (skipList *Skiplist) Put(key []byte, value []byte, lsn constants.LsnType, entryType constants.EntryType) {
	nodeList := make([]*Node, skipList.maxLevel)

	curr := skipList.head

	for i := skipList.maxLevel - 1; i >= 0; i-- {
		for curr.lvlPtrs[i] != nil && bytes.Compare(curr.lvlPtrs[i].key, key) < 0 {
			curr = curr.lvlPtrs[i]
		}
		nodeList[i] = curr
	}

	target := nodeList[0].lvlPtrs[0]
	if target != nil && bytes.Equal(target.key, key) {
		if target.versions[len(target.versions)-1].lsn < lsn {
			target.versions = append(target.versions, Version{value: value, lsn: lsn, entryType: entryType})
			skipList.size += len(value) + 8
			return
		}
		panic("larger lsn found") // to be handled later
	}

	insertLvl := 0

	for shouldMoveToNextLvl(skipList.threshold) {
		insertLvl++
		if insertLvl >= skipList.maxLevel {
			insertLvl = skipList.maxLevel - 1
			break
		}
	}

	node := newNode(key, value, lsn, entryType, insertLvl)

	for i := len(nodeList) - 1; i >= 0; i-- {
		if i <= insertLvl {
			node.lvlPtrs[i] = nodeList[i].lvlPtrs[i]
			nodeList[i].lvlPtrs[i] = node
		}
	}
	skipList.size += len(key) + len(value) + 8 // to be improved lated
}

func (skiplist *Skiplist) Size() int {
	return skiplist.size
}

func (skiplist *Skiplist) Validate() bool {
	return true
}
