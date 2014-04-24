package readers

import (
	"encoding/binary"
	"io"
)

const nonRepeatingModulo = 87178291199
const nonRepeatingIncrement = 17180131327

// *should* produce a non-repeating sequence of bytes in a deterministic fashion
// use io.LimitReader to limit it to a specific length
type nonRepeatingSequenceReader struct {
	value int
}

func NewNonRepeatingSequence(i int) io.Reader {
	return &nonRepeatingSequenceReader{i}
}

func NewSizedNonRepeatingSequence(i int, s int64) io.Reader {
	return io.LimitReader(NewNonRepeatingSequence(i), s)
}

func (r *nonRepeatingSequenceReader) Read(p []byte) (n int, err error) {
	lenp := len(p)
	b := []byte{1, 2, 3, 4}

	for i := 0; i < lenp; i++ {
		binary.LittleEndian.PutUint32(b, uint32(r.value))
		p[i] = b[0]
		r.value = (r.value + nonRepeatingIncrement) % nonRepeatingModulo
	}
	return lenp, nil
}
