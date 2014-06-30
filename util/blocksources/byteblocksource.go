package blocksources

import (
	"github.com/Redundancy/go-sync/patcher"
)

func NewByteBlockSource(data []byte) *ByteBlockSource {
	s := &ByteBlockSource{
		data:         data,
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
type ByteBlockSource struct {
	data           []byte
	errorChan      chan error
	responseChan   chan patcher.BlockReponse
	requestChan    chan patcher.MissingBlockSpan
	requestedBytes int64
}

func (s *ByteBlockSource) ReadBytes() int64 {
	return s.requestedBytes
}

func (s *ByteBlockSource) RequestBlock(block patcher.MissingBlockSpan) error {
	s.requestChan <- block
	return nil
}

func (s *ByteBlockSource) GetResultChannel() <-chan patcher.BlockReponse {
	return s.responseChan
}

// If the block source encounters an unsurmountable problem
func (s *ByteBlockSource) EncounteredError() <-chan error {
	return s.errorChan
}

func (s *ByteBlockSource) loop() {
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

			if endOffset > int64(len(s.data)) {
				endOffset = int64(len(s.data))
			}

			pendingResponses = append(
				pendingResponses,
				patcher.BlockReponse{
					StartBlock: request.StartBlock,
					Data:       s.data[startOffset:endOffset],
				},
			)

			firstResponse = pendingResponses[0]
			responseChan = s.responseChan
			s.requestedBytes += endOffset - startOffset

		case responseChan <- firstResponse:
			// take 1
			pendingResponses = pendingResponses[1:]
			if len(pendingResponses) == 0 {
				responseChan = nil
			} else {
				firstResponse = pendingResponses[0]
			}

		case errorChan <- currentError:
			// no errors!
		}
	}
}
