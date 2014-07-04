package blocksources

import (
	"github.com/Redundancy/go-sync/patcher"
	"io"
)

const (
	from_start = 0
)

type ReadSeeker interface {
	Read(b []byte) (n int, err error)
	Seek(offset int64, whence int) (int64, error)
}

func NewReadSeekerBlockSource(r ReadSeeker) *ReadSeekerBlockSource {
	s := &ReadSeekerBlockSource{
		data:         r,
		errorChan:    make(chan error),
		responseChan: make(chan patcher.BlockReponse),
		requestChan:  make(chan patcher.MissingBlockSpan),
	}

	go s.loop()

	return s
}

/*
ByteBlockSource is provided largely for convenience in testing and
as a very simple example.
*/
type ReadSeekerBlockSource struct {
	data           ReadSeeker
	errorChan      chan error
	responseChan   chan patcher.BlockReponse
	requestChan    chan patcher.MissingBlockSpan
	requestedBytes int64
}

func (s *ReadSeekerBlockSource) ReadBytes() int64 {
	return s.requestedBytes
}

func (s *ReadSeekerBlockSource) RequestBlock(block patcher.MissingBlockSpan) error {
	s.requestChan <- block
	return nil
}

func (s *ReadSeekerBlockSource) GetResultChannel() <-chan patcher.BlockReponse {
	return s.responseChan
}

// If the block source encounters an unsurmountable problem
func (s *ReadSeekerBlockSource) EncounteredError() <-chan error {
	return s.errorChan
}

func (s *ReadSeekerBlockSource) loop() {
	defer close(s.errorChan)
	defer close(s.responseChan)
	pendingResponses := make([]patcher.BlockReponse, 0, 10)

	// Set to nil when there is nothing to send
	var responseChan chan patcher.BlockReponse = nil
	var errorChan chan error = nil
	var currentError error = nil
	var firstResponse patcher.BlockReponse

	for {
		select {
		case request := <-s.requestChan:
			startOffset := int64(request.StartBlock) * request.BlockSize
			endOffset := int64(request.EndBlock+1) * request.BlockSize
			read_length := endOffset - startOffset

			buffer := make([]byte, read_length)

			if _, err := s.data.Seek(startOffset, from_start); err != nil {
				currentError = err
				errorChan = s.errorChan
				continue
			}

			n, err := io.ReadFull(s.data, buffer)

			if err != nil && err != io.ErrUnexpectedEOF {
				currentError = err
				errorChan = s.errorChan
				continue
			}

			pendingResponses = append(
				pendingResponses,
				patcher.BlockReponse{
					StartBlock: request.StartBlock,
					Data:       buffer[:n],
				},
			)

			firstResponse = pendingResponses[0]
			responseChan = s.responseChan
			s.requestedBytes += int64(n)

		case responseChan <- firstResponse:
			// take 1
			pendingResponses = pendingResponses[1:]
			if len(pendingResponses) == 0 {
				responseChan = nil
			} else {
				firstResponse = pendingResponses[0]
			}

		case errorChan <- currentError:
			currentError = nil
			errorChan = nil
		}
	}
}
