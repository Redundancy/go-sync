package blocksources

import (
	"io"
)

const (
	from_start = 0
)

type ReadSeeker interface {
	Read(b []byte) (n int, err error)
	Seek(offset int64, whence int) (int64, error)
}

func NewReadSeekerBlockSource(
	r ReadSeeker,
	resolver BlockSourceOffsetResolver,
) *BlockSourceBase {
	return NewBlockSourceBase(
		&ReadSeekerRequester{
			rs: r,
		},
		resolver,
		nil, // TODO: No verifier!
		1,
		8*MB,
	)
}

type ReadSeekerRequester struct {
	rs ReadSeeker
}

func (r *ReadSeekerRequester) DoRequest(startOffset int64, endOffset int64) (data []byte, err error) {
	read_length := endOffset - startOffset
	buffer := make([]byte, read_length)

	if _, err = r.rs.Seek(startOffset, from_start); err != nil {
		return
	}

	n, err := io.ReadFull(r.rs, buffer)

	if err != nil && err != io.ErrUnexpectedEOF {
		return
	}

	return buffer[:n], nil
}

func (r *ReadSeekerRequester) IsFatal(err error) bool {
	return true
}
