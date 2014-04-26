/*
Provides a circular byte buffer of a given size

Writing to the buffer will return a []byte of the evicted bytes
Getting the buffer will return a []byte of the current contents

Note that the implementation actually needs to be optimized for two seperate use cases:

1) During index generation, we reliably write BLOCK_SIZE chunks every time.
In this case, we don't care about evicted bytes, and we never need to look at the buffer - so don't use any storage in this case

2) While comparing, we frequently write a single byte and need to know the evicted byte.
We infrequently may write a whole block and not need to know the evicted bytes. We also infrequently need the whole buffer.

This is the case that the C2 buffer is optimized for, as it will not do any allocation during this
*/
package circularbuffer

/*
DEPRECATED

This is a basic circular buffer of bytes. Once you have written the full buffer size
you start to overwrite old values. In order to allow it to be used for a rolling checksum,
it also returns the bytes that are evicted by writing a new value.

It is however, pretty allocation heavy, and not particularly optimized for the actual usecases
that we need. I keep it around for the moment as a comparative benchmark.
*/
type CircularBuffer struct {
	buffer      []byte
	head        int
	startOffset int
}

// DEPRECATED
func NewCircularBuffer(size int64) *CircularBuffer {
	return &CircularBuffer{
		buffer:      make([]byte, 0, size),
		startOffset: 0,
		head:        0,
	}
}

// True once the circular buffer has been filled to capacity and further
// writes of even a single byte will cause older bytes to be evicted
func (buff *CircularBuffer) IsFull() bool {
	return len(buff.buffer) == cap(buff.buffer)
}

// io.Writer
func (buff *CircularBuffer) Write(p []byte) (n int, err error) {
	buff.WriteEvicted(p)
	return n, nil
}

// Write new data into the buffer, but return any data evicted from the buffer
// this is particularly for dealing with rolling checksums, where new data is typically
// added to the checksum, and old data needs to be subtracted
func (buff *CircularBuffer) WriteEvicted(p []byte) []byte {
	c := cap(buff.buffer)
	l := len(buff.buffer)
	p_len := len(p)

	var remaining []byte

	if p_len >= c {
		// bigger than available anyway
		old, newBuffer := buff.buffer, make([]byte, c, c)

		//copy()
		for i, b := range p[p_len-c:] {
			newBuffer[i] = b
		}

		buff.buffer = newBuffer
		buff.head = 0

		if l == 0 {
			return nil
		} else {
			return old
		}

	} else if len(p) < c-l {
		// has available capacity, nothing evicted
		buff.buffer = append(buff.buffer, p...)
		return nil
	} else if l != c {
		// fill available first
		buff.buffer = append(buff.buffer, p[:c-l]...)
		remaining = p[c-l:]
	} else {
		// already full
		remaining = p
	}

	overwritten := make([]byte, len(remaining))

	// TODO: use copy
	for i, b := range remaining {
		overwritten[i] = buff.buffer[buff.head]
		buff.buffer[buff.head] = b
		buff.head = (buff.head + 1) % c
	}

	return overwritten
}

// Get the contents of the buffer, from oldest to newest bytes
// This will return the buffer itself if possible, otherwise
// it will have to allocate and copy data into a new block of memory
func (b *CircularBuffer) Get() []byte {
	if b.head == 0 {
		return b.buffer
	} else {
		l := len(b.buffer)
		c := make([]byte, l)

		for i, _ := range c {
			c[i] = b.buffer[(b.head+i)%l]
		}

		return c
	}
}

// Alias for Get()
func (b *CircularBuffer) GetLastBlock() []byte {
	return b.Get()
}
