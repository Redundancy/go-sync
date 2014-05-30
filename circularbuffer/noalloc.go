package circularbuffer

/*
C2 is a circular buffer intended to allow you to write a block of data of up to 'blocksize', and retrieve the
data evicted by that operation, without allocating any extra slice storage

This requires that it keep at least blocksize*2 data around. In fact, it doubles that again in order to
guarantee that both of these bits of information can always be obtained in a single contiguous block of memory.

Other than the cost of the extra memory (4xblocksize), this means that it requires 2 writes for every byte stored.
*/
type C2 struct {
	// used to know how much was evicted
	lastWritten int

	// total number of written bytes
	// used to track if the buffer has been filled, but goes above blocksize
	totalWritten int

	// quick access to the circular buffer size
	blocksize int

	// double internal buffer storage
	a, b doubleSizeBuffer
}

type doubleSizeBuffer struct {
	// used to reset the head pointer
	baseOffset int

	// index of the next byte to be written
	head int

	// buffer
	buffer []byte
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

func (c *C2) getBlockBuffer() *doubleSizeBuffer {
	bufferToRead := &c.a
	if c.b.head > c.a.head {
		bufferToRead = &c.b
	}

	return bufferToRead
}

// the total written, up to the blocksize
func (c *C2) maxWritten() int {
	if c.totalWritten < c.blocksize {
		return c.totalWritten
	}

	return c.blocksize
}

func (c *C2) Len() int {
	return c.maxWritten()
}

func (c *C2) Empty() bool {
	return c.totalWritten == 0
}

// Shortens the content of the circular buffer
// and returns the content removed
func (c *C2) Truncate(byteCount int) (evicted []byte) {
	max := c.maxWritten()

	if byteCount > max {
		byteCount = max
	}

	bufferToRead := c.getBlockBuffer()
	start := bufferToRead.head - max

	c.totalWritten = c.maxWritten() - byteCount
	return bufferToRead.buffer[start : start+byteCount]
}

// get the current buffer contents of block
func (c *C2) GetBlock() []byte {
	// figure out which buffer has it stored contiguously
	bufferToRead := c.getBlockBuffer()
	start := bufferToRead.head - c.maxWritten()

	return bufferToRead.buffer[start:bufferToRead.head]
}

// get the data that was evicted by the last write
func (c *C2) Evicted() []byte {
	if c.totalWritten <= c.blocksize {
		return nil
	}

	bufferToRead := c.a
	if c.b.head < c.a.head {
		bufferToRead = c.b
	}

	bufferStart := bufferToRead.head + c.blocksize
	readLength := c.lastWritten

	// if the buffer wasn't full, we don't read the full length
	if c.totalWritten-c.lastWritten < c.blocksize {
		readLength -= c.lastWritten - c.totalWritten + c.blocksize
	}

	return bufferToRead.buffer[bufferStart-readLength : bufferStart]
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
