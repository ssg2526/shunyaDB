// iterator for scanning the keys in paginated way
package iterator

type Iterator interface {
	Seek(key []byte)
	Next()
	Valid() bool
	Key() []byte
	Value() []byte
}
