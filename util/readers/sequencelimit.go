package readers

import (
	"io"
)

// read from 'readers' in sequence up to a limit of 'size'
func SequenceLimit(size int64, readers ...io.Reader) io.Reader {
	return io.LimitReader(
		io.MultiReader(readers...),
		size)
}
