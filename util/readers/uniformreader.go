package readers

import (
	"io"
)

// Reads a continuous stream of bytes with the same value, up to length
type uniformReader struct {
	value  byte
	length int
	read   int
}

func (r *uniformReader) Read(p []byte) (n int, err error) {
	destinationLength := len(p)
	readable := r.length - r.read
	read := destinationLength

	if readable < destinationLength {
		read = readable
	}

	if read == 0 {
		return 0, io.EOF
	}

	for i := 0; i < read; i++ {
		p[i] = r.value
	}

	var result error = nil
	if read == readable {
		result = io.EOF
	}

	r.read += read

	return read, result
}

func ZeroReader(length int) io.Reader {
	return &uniformReader{
		value:  0,
		length: length,
		read:   0,
	}
}

func OneReader(length int) io.Reader {
	return &uniformReader{
		value:  1,
		length: length,
		read:   0,
	}
}
