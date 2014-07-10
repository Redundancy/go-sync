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

type asyncResult struct {
	blockID uint
	data    []byte
	err     error
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

	// enable us to order responses for the active requests, lowest to highest
	requestOrdering := make(UintSlice, 0, s.ConcurrentRequests)
	responseOrdering := make(PendingResponses, 0, s.ConcurrentRequests)

	for state == STATE_RUNNING || inflightRequests > 0 || pendingErrors.Err() != nil {
		select {
		case newRequest := <-s.requestChannel:
			inflightRequests += 1
			requestOrdering = append(requestOrdering, newRequest.StartBlock)
			sort.Sort(sort.Reverse(requestOrdering))

			go func() {
				result, err := s.Requester.DoRequest(
					int64(newRequest.StartBlock)*newRequest.BlockSize,
					int64(newRequest.EndBlock+1)*newRequest.BlockSize,
				)

				resultChan <- asyncResult{
					blockID: newRequest.StartBlock,
					data:    result,
					err:     err,
				}
			}()
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

			lowestResponse := responseOrdering[len(responseOrdering)-1]
			lowestRequest := requestOrdering[len(requestOrdering)-1]

			if lowestRequest == lowestResponse.StartBlock {
				pendingResponse.setResponse(&lowestResponse)
				responseOrdering = responseOrdering[:len(responseOrdering)-1]
				requestOrdering = requestOrdering[:len(requestOrdering)-1]
			}

		case pendingResponse.sendIfPending() <- pendingResponse.Response():
			pendingResponse.clear()

			// check if there's another response to enqueue
			if len(responseOrdering) > 0 {
				lowestResponse := responseOrdering[len(responseOrdering)-1]
				lowestRequest := requestOrdering[len(requestOrdering)-1]

				if lowestRequest == lowestResponse.StartBlock {
					pendingResponse.setResponse(&lowestResponse)
					responseOrdering = responseOrdering[:len(responseOrdering)-1]
					requestOrdering = requestOrdering[:len(requestOrdering)-1]
				}
			}

		case pendingErrors.sendIfSet() <- pendingErrors.Err():
			pendingErrors.clear()

		case <-s.exitChannel:
			state = STATE_EXITING
		}
	}
}

// errorWatcher is a small helper object
// sendIfSet will only return a channel if there is an error set
// so w.sendIfSet() <- w.Err() is always safe in a select statement
// even if there is no error set
type errorWatcher struct {
	errorChannel chan error
	lastError    error
}

func (w *errorWatcher) setError(e error) {
	if w.lastError != nil {
		panic("cannot set a new error when one is already set!")
	}
	w.lastError = e
}

func (w *errorWatcher) clear() {
	w.lastError = nil
}

func (w *errorWatcher) Err() error {
	return w.lastError
}

func (w *errorWatcher) sendIfSet() chan<- error {
	if w.lastError != nil {
		return w.errorChannel
	} else {
		return nil
	}
}

type pendingResponseHelper struct {
	responseChannel chan patcher.BlockReponse
	pendingResponse *patcher.BlockReponse
}

func (w *pendingResponseHelper) setResponse(r *patcher.BlockReponse) {
	if w.pendingResponse != nil {
		panic("setting a response when one is already set!")
	}
	w.pendingResponse = r
}

func (w *pendingResponseHelper) clear() {
	w.pendingResponse = nil
}

func (w *pendingResponseHelper) Response() patcher.BlockReponse {
	if w.pendingResponse == nil {
		return patcher.BlockReponse{}
	}
	return *w.pendingResponse
}

func (w *pendingResponseHelper) sendIfPending() chan<- patcher.BlockReponse {
	if w.pendingResponse != nil {
		return w.responseChannel
	} else {
		return nil
	}

}

type UintSlice []uint

func (r UintSlice) Len() int {
	return len(r)
}

func (r UintSlice) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func (r UintSlice) Less(i, j int) bool {
	return r[i] < r[j]
}
