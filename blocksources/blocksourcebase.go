package blocksources

import (
	"github.com/Redundancy/go-sync/patcher"
	"sort"
)

type BlockSourceRequester interface {
	// This method is called on multiple goroutines, and must
	// support simultaneous requests
	DoRequest(startOffset int64, endOffset int64) (data []byte, err error)

	// If an error raised by DoRequest should cause BlockSourceBase
	// to give up, return true
	IsFatal(err error) bool
}

func NewBlockSourceBase(
	requester BlockSourceRequester,
	concurrentRequestCount int,
	concurrentBytes int64,
) *BlockSourceBase {

	b := &BlockSourceBase{
		Requester:          requester,
		ConcurrentRequests: concurrentRequestCount,
		ConcurrentBytes:    concurrentBytes,
		exitChannel:        make(chan bool),
		errorChannel:       make(chan error),
		responseChannel:    make(chan patcher.BlockReponse),
		requestChannel:     make(chan patcher.MissingBlockSpan),
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
	Requester BlockSourceRequester

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

func (s *BlockSourceBase) RequestBlock(block patcher.MissingBlockSpan) error {
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

func (s *BlockSourceBase) Close() {
	// TODO: race condition
	if !s.hasQuit {
		s.exitChannel <- true
	}
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

	requestQueue := make(queuedRequestList, 0, s.ConcurrentRequests*2)

	// enable us to order responses for the active requests, lowest to highest
	requestOrdering := make(UintSlice, 0, s.ConcurrentRequests)
	responseOrdering := make(PendingResponses, 0, s.ConcurrentRequests)

	for state == STATE_RUNNING || inflightRequests > 0 || pendingErrors.Err() != nil {

		// Start any pending work that we can
		for inflightRequests < s.ConcurrentRequests && len(requestQueue) > 0 {
			inflightRequests += 1

			nextRequest := requestQueue[len(requestQueue)-1]

			requestOrdering = append(requestOrdering, nextRequest.startBlockID)
			sort.Sort(sort.Reverse(requestOrdering))
			go func() {
				result, err := s.Requester.DoRequest(
					nextRequest.startOffset,
					nextRequest.endOffset,
				)

				resultChan <- asyncResult{
					blockID: nextRequest.startBlockID,
					data:    result,
					err:     err,
				}
			}()

			// remove dispatched request
			requestQueue = requestQueue[:len(requestQueue)-1]
		}

		select {
		case newRequest := <-s.requestChannel:
			// TODO: limit size of individual requests, so that we can
			// ensure that we use multiple concurrent connections, even when there
			// is only a single continuous missing block

			requestQueue = append(requestQueue, queuedRequest{
				startBlockID: newRequest.StartBlock,
				startOffset:  int64(newRequest.StartBlock) * newRequest.BlockSize,
				endOffset:    int64(newRequest.EndBlock+1) * newRequest.BlockSize,
			})

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
			// TODO: Confirm checksum

			responseOrdering = append(responseOrdering,
				patcher.BlockReponse{
					StartBlock: result.blockID,
					Data:       result.data,
				},
			)

			// sort high to low
			sort.Sort(sort.Reverse(responseOrdering))

			// if we just got the lowest requested block, we can set
			// the response. Otherwise, wait.
			lowestRequest := requestOrdering[len(requestOrdering)-1]

			if lowestRequest == result.blockID {
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
