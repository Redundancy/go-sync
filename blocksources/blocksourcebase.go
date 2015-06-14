package blocksources

import (
	"errors"
	"fmt"
	"sort"

	"github.com/Redundancy/go-sync/patcher"
)

// BlockSourceRequester does synchronous requests on a remote source of blocks
// concurrency is handled by the BlockSourceBase. This provides a simple way
// of implementing a particular
type BlockSourceRequester interface {
	// This method is called on multiple goroutines, and must
	// support simultaneous requests
	DoRequest(startOffset int64, endOffset int64) (data []byte, err error)

	// If an error raised by DoRequest should cause BlockSourceBase
	// to give up, return true
	IsFatal(err error) bool
}

// A BlockSourceOffsetResolver resolves a blockID to a start offset and and end offset in a file
// it also handles splitting up ranges of blocks into multiple requests, allowing requests to be split down to the
// block size, and handling of compressed blocks (given a resolver that can work out the correct range to query for,
// and a BlockSourceRequester that will decompress the result into a full sized block)
type BlockSourceOffsetResolver interface {
	GetBlockStartOffset(blockID uint) int64
	GetBlockEndOffset(blockID uint) int64
	SplitBlockRangeToDesiredSize(startBlockID, endBlockID uint) []QueuedRequest
}

// Checks blocks against their expected checksum
type BlockVerifier interface {
	VerifyBlockRange(startBlockID uint, data []byte) bool
}

func NewBlockSourceBase(
	requester BlockSourceRequester,
	resolver BlockSourceOffsetResolver,
	verifier BlockVerifier,
	concurrentRequestCount int,
	concurrentBytes int64,
) *BlockSourceBase {

	b := &BlockSourceBase{
		Requester:           requester,
		BlockSourceResolver: resolver,
		Verifier:            verifier,
		ConcurrentRequests:  concurrentRequestCount,
		ConcurrentBytes:     concurrentBytes,
		exitChannel:         make(chan bool),
		errorChannel:        make(chan error),
		responseChannel:     make(chan patcher.BlockReponse),
		requestChannel:      make(chan patcher.MissingBlockSpan),
	}

	go b.loop()

	return b
}

// BlockSourceBase provides an implementation of blocksource
// that takes care of everything except for the actual asyncronous request
// this makes blocksources easier and faster to build reliably
// BlockSourceBase implements patcher.BlockSource, and if it's good enough,
// perhaps nobody else ever will have to.
type BlockSourceBase struct {
	Requester           BlockSourceRequester
	BlockSourceResolver BlockSourceOffsetResolver
	Verifier            BlockVerifier

	// The number of requests that BlockSourceBase may service at once
	ConcurrentRequests int

	// The number of bytes that BlockSourceBase may have in-flight
	// (requested + pending delivery)
	ConcurrentBytes int64

	hasQuit         bool
	exitChannel     chan bool
	errorChannel    chan error
	responseChannel chan patcher.BlockReponse
	requestChannel  chan patcher.MissingBlockSpan

	bytesRequested int64
}

const (
	STATE_RUNNING = iota
	STATE_EXITING
)

func (s *BlockSourceBase) ReadBytes() int64 {
	return s.bytesRequested
}

func (s *BlockSourceBase) RequestBlocks(block patcher.MissingBlockSpan) error {
	s.requestChannel <- block
	return nil
}

func (s *BlockSourceBase) GetResultChannel() <-chan patcher.BlockReponse {
	return s.responseChannel
}

// If the block source encounters an unsurmountable problem
func (s *BlockSourceBase) EncounteredError() <-chan error {
	return s.errorChannel
}

var BlockSourceAlreadyClosedError = errors.New("Block source was already closed")

func (s *BlockSourceBase) Close() (err error) {
	// if it has already been closed, just recover
	// however, let the caller know
	defer func() {
		if recover() != nil {
			err = BlockSourceAlreadyClosedError
		}
	}()

	if !s.hasQuit {
		s.exitChannel <- true
	}

	return
}

func (s *BlockSourceBase) loop() {
	defer func() {
		s.hasQuit = true
		close(s.exitChannel)
		close(s.errorChannel)
		close(s.requestChannel)
		close(s.responseChannel)
	}()

	state := STATE_RUNNING
	inflightRequests := 0
	//inflightBytes := int64(0)
	pendingErrors := &errorWatcher{errorChannel: s.errorChannel}
	pendingResponse := &pendingResponseHelper{responseChannel: s.responseChannel}
	resultChan := make(chan asyncResult)
	defer close(resultChan)

	requestQueue := make(QueuedRequestList, 0, s.ConcurrentRequests*2)

	// enable us to order responses for the active requests, lowest to highest
	requestOrdering := make(UintSlice, 0, s.ConcurrentRequests)
	responseOrdering := make(PendingResponses, 0, s.ConcurrentRequests)

	for state == STATE_RUNNING || inflightRequests > 0 || pendingErrors.Err() != nil {

		// Start any pending work that we can
		for inflightRequests < s.ConcurrentRequests && len(requestQueue) > 0 {
			inflightRequests += 1

			nextRequest := requestQueue[len(requestQueue)-1]

			requestOrdering = append(requestOrdering, nextRequest.StartBlockID)
			sort.Sort(sort.Reverse(requestOrdering))
			go func() {
				resolver := s.BlockSourceResolver

				startOffset := resolver.GetBlockStartOffset(
					nextRequest.StartBlockID,
				)

				endOffset := resolver.GetBlockEndOffset(
					nextRequest.EndBlockID,
				)

				result, err := s.Requester.DoRequest(
					startOffset,
					endOffset,
				)

				resultChan <- asyncResult{
					startBlockID: nextRequest.StartBlockID,
					endBlockID:   nextRequest.EndBlockID,
					data:         result,
					err:          err,
				}
			}()

			// remove dispatched request
			requestQueue = requestQueue[:len(requestQueue)-1]
		}

		select {
		case newRequest := <-s.requestChannel:
			requestQueue = append(
				requestQueue,
				s.BlockSourceResolver.SplitBlockRangeToDesiredSize(
					newRequest.StartBlock,
					newRequest.EndBlock,
				)...,
			)

			sort.Sort(sort.Reverse(requestQueue))

		case result := <-resultChan:
			inflightRequests -= 1

			if result.err != nil {
				pendingErrors.setError(result.err)
				pendingResponse.clear()
				state = STATE_EXITING
				break
			}

			s.bytesRequested += int64(len(result.data))

			if s.Verifier != nil && !s.Verifier.VerifyBlockRange(result.startBlockID, result.data) {
				pendingErrors.setError(
					fmt.Errorf(
						"The returned block range (%v-%v) did not match the expected checksum for the blocks",
						result.startBlockID, result.endBlockID,
					),
				)
				pendingResponse.clear()
				state = STATE_EXITING
				break
			}

			responseOrdering = append(responseOrdering,
				patcher.BlockReponse{
					StartBlock: result.startBlockID,
					Data:       result.data,
				},
			)

			// sort high to low
			sort.Sort(sort.Reverse(responseOrdering))

			// if we just got the lowest requested block, we can set
			// the response. Otherwise, wait.
			lowestRequest := requestOrdering[len(requestOrdering)-1]

			if lowestRequest == result.startBlockID {
				lowestResponse := responseOrdering[len(responseOrdering)-1]
				pendingResponse.clear()
				pendingResponse.setResponse(&lowestResponse)
			}

		case pendingResponse.sendIfPending() <- pendingResponse.Response():
			pendingResponse.clear()
			responseOrdering = responseOrdering[:len(responseOrdering)-1]
			requestOrdering = requestOrdering[:len(requestOrdering)-1]

			// check if there's another response to enqueue
			if len(responseOrdering) > 0 {
				lowestResponse := responseOrdering[len(responseOrdering)-1]
				lowestRequest := requestOrdering[len(requestOrdering)-1]

				if lowestRequest == lowestResponse.StartBlock {
					pendingResponse.setResponse(&lowestResponse)

				}
			}

		case pendingErrors.sendIfSet() <- pendingErrors.Err():
			pendingErrors.clear()

		case <-s.exitChannel:
			state = STATE_EXITING
		}
	}
}
