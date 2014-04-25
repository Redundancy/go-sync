package circularbuffer

/*
C2 is a circular buffer intended to allow you to write a block of data of up to 'blocksize', and retrieve the
data evicted by that operation, without allocating any extra slice storage

This requires that it keep at least blocksize*2 data around. In fact, it doubles that again in order to
guarantee that both of these bits of information can always be obtained in a single contiguous block of memory.

Other than the cost of the extra memory (4xblocksize), this means that it requires 2 writes for every byte stored.
*/
type C2 struct {
	lastWritten  int
	totalWritten int
	blocksize    int
	a, b         doubleSizeBuffer
}

type doubleSizeBuffer struct {
	baseOffset int
	head       int
	buffer     []byte
}

func MakeC2Buffer(blockSize int) *C2 {
	return &C2{
		blocksize: blockSize,
		a: doubleSizeBuffer{
			baseOffset: 0,
			buffer:     make([]byte, blockSize*2),
		},
		b: doubleSizeBuffer{
			baseOffset: blockSize,
			head:       blockSize,
			buffer:     make([]byte, blockSize*2),
		},
	}
}

func (c *C2) Reset() {
	c.a.Reset()
	c.b.Reset()
	c.lastWritten = 0
	c.totalWritten = 0
}

// Write new data
func (c *C2) Write(b []byte) {
	c.a.Write(b)
	c.b.Write(b)
	c.lastWritten = len(b)
	c.totalWritten += c.lastWritten
}

// get the current buffer contents of block
func (c *C2) GetBlock() []byte {
	// figure out which buffer has it stored contiguously
	bufferToRead := c.a
	if c.b.head > c.a.head {
		bufferToRead = c.b
	}

	getSize := c.blocksize
	if c.totalWritten < c.blocksize {
		getSize = c.totalWritten
	}

	return bufferToRead.buffer[bufferToRead.head-getSize : bufferToRead.head]
}

// get the data that was evicted by the last write
func (c *C2) Evicted() []byte {
	bufferToRead := c.a
	if c.b.head < c.a.head {
		bufferToRead = c.b
	}

	return bufferToRead.buffer[bufferToRead.head+c.blocksize-c.lastWritten : bufferToRead.head+c.blocksize]
}

func (buff *doubleSizeBuffer) Reset() {
	buff.head = buff.baseOffset
}

func (buff *doubleSizeBuffer) Write(by []byte) {
	remaining := by

	for len(remaining) > 0 {
		remaining_len := len(remaining)
		availableSpace := len(buff.buffer) - buff.head
		writeThisTime := remaining_len

		if writeThisTime > availableSpace {
			writeThisTime = availableSpace
		}

		copy(
			buff.buffer[buff.head:buff.head+writeThisTime], // to
			by,
		)

		buff.head += writeThisTime

		if buff.head == len(buff.buffer) {
			buff.head = 0
		}

		remaining = remaining[writeThisTime:]
	}
}
