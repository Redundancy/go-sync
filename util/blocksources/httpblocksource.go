package blocksources

import (
	"errors"
	"fmt"
	"github.com/Redundancy/go-sync/patcher"
	"io/ioutil"
	"net/http"
	"sort"
	"time"
)

/*
func NewLimitedNetConnection(bytesPerSecond int64) *LimitedNetConnection {
	c := &LimitedNetConnection{}
	c.Reader = flowcontrol.NewReader(c.Conn, bytesPerSecond)

	return nil
}

type LimitedNetConnection struct {
	net.Conn
	*flowcontrol.Reader
}
*/

func NewHttpBlockSource(
	url string,
	concurrentRequests int,
	//bytesPerSecondLimit int64,
) *HttpBlockSource {

	s := &HttpBlockSource{
		url:                url,
		concurrentRequests: concurrentRequests,
		errorChan:          make(chan error),
		responseChan:       make(chan patcher.BlockReponse),
		requestChan:        make(chan patcher.MissingBlockSpan),
	}

	// Rate limiting not implmented yet
	/*if bytesPerSecondLimit > 0 {
		s.client = &http.Client{}
		//s.client.Transport = NewLimitedNetConnection(bytesPerSecondLimit)
	} else {
	*/
	s.client = http.DefaultClient
	//}

	go s.loop()

	return s
}

type HttpBlockSource struct {
	url                string
	concurrentRequests int
	errorChan          chan error
	responseChan       chan patcher.BlockReponse
	httpResponseChan   chan httpResponse
	requestChan        chan patcher.MissingBlockSpan
	requestedBytes     int64
	client             *http.Client
}

type httpResponse struct {
	err     error
	blockID uint
	reponse *http.Response
}

func (s *HttpBlockSource) ReadBytes() int64 {
	return s.requestedBytes
}

var TimeoutError = errors.New("Request timed out")

func (s *HttpBlockSource) RequestBlock(block patcher.MissingBlockSpan) error {
	// Reqest channel may be blocked
	// TODO: this does not deal with the case where s.requestChan has been closed
	// which would panic
	select {
	case s.requestChan <- block:
		return nil
	case <-time.After(time.Second):
		return TimeoutError
	}

}

func (s *HttpBlockSource) GetResultChannel() <-chan patcher.BlockReponse {
	return s.responseChan
}

// If the block source encounters an unsurmountable problem
func (s *HttpBlockSource) EncounteredError() <-chan error {
	return s.errorChan
}

type PendingResponses []patcher.BlockReponse

func (r PendingResponses) Len() int {
	return len(r)
}

func (r PendingResponses) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func (r PendingResponses) Less(i, j int) bool {
	return r[i].StartBlock < r[j].StartBlock
}

func (s *HttpBlockSource) loop() {
	defer close(s.errorChan)
	defer close(s.responseChan)
	inflightRequests := 0

	// Set to nil when there is nothing to send
	var responseChan chan patcher.BlockReponse = nil
	var errorChan chan error = nil
	var currentError error = nil
	var responses = make(PendingResponses, 0, s.concurrentRequests)
	var nextReponse patcher.BlockReponse

	requestChan := s.requestChan
	s.httpResponseChan = make(chan httpResponse, s.concurrentRequests)

	fatalError := false

	//ServiceLoop:
	for !fatalError || inflightRequests > 0 {
		select {
		case request := <-requestChan:
			startOffset := int64(request.StartBlock) * request.BlockSize
			// for the last block, may be past the end
			endOffset := int64(request.EndBlock+1)*request.BlockSize - 1

			// fire off an http request
			s.startRequest(
				startOffset, endOffset,
				request.StartBlock,
			)

			inflightRequests += 1
			if inflightRequests == s.concurrentRequests {
				requestChan = nil
			}

		case responseChan <- nextReponse:
			s.requestedBytes += int64(len(nextReponse.Data))
			responses = responses[:len(responses)-1]

			if len(responses) == 0 {
				responseChan = nil
			} else {
				nextReponse = responses[len(responses)-1]
			}

		case response := <-s.httpResponseChan:
			var data []byte
			var err error

			if response.err != nil {
				// for the moment, we assume that no error is recoverable
				// TODO: figure out strategies for various possible recoverable
				// errors
				fatalError = true
				errorChan = s.errorChan
				currentError = response.err

				// Prevent any more requests from being filled
				requestChan = nil
				continue

			} else if data, err = ioutil.ReadAll(response.reponse.Body); err != nil {
				fatalError = true
				errorChan = s.errorChan
				currentError = err

				response.reponse.Body.Close()

				// Prevent any more requests from being filled
				requestChan = nil
				continue
			}
			defer response.reponse.Body.Close()

			inflightRequests -= 1

			if !fatalError && requestChan == nil {
				// clearly we were full, but now have a free request slot
				requestChan = s.requestChan
			}

			responses = append(
				responses,
				patcher.BlockReponse{
					Data:       data,
					StartBlock: response.blockID,
				},
			)

			sort.Sort(sort.Reverse(responses))
			nextReponse = responses[len(responses)-1]

			if responseChan == nil {
				responseChan = s.responseChan
			}

		case errorChan <- currentError:
			errorChan = nil
			currentError = nil
		}
	}
}

func (s *HttpBlockSource) startRequest(
	startOffset, endOffset int64,
	blockID uint,
) {
	go func() {
		req, err := rangedRequest(
			s.client,
			s.url,
			startOffset,
			endOffset,
		)

		s.httpResponseChan <- httpResponse{err, blockID, req}
	}()
}

var RangedRequestNotSupportedError = errors.New("Ranged request not supported (Server did not respond with 206 Status)")
var UrlNotFoundError = errors.New("404 Error on URL")

func rangedRequest(
	client *http.Client,
	url string,
	startOffset int64,
	endOffset int64,
) (*http.Response, error) {
	rangedRequest, err := http.NewRequest("GET", url, nil)

	if err != nil {
		return nil, err
	}

	rangeSpecifier := fmt.Sprintf("bytes=%v-%v", startOffset, endOffset)
	rangedRequest.ProtoAtLeast(1, 1)
	rangedRequest.Header.Add("Range", rangeSpecifier)
	rangedResponse, err := client.Do(rangedRequest)

	if err != nil {
		return nil, err
	}

	if rangedResponse.StatusCode == 404 {
		return nil, UrlNotFoundError
	} else if rangedResponse.StatusCode != 206 {
		rangedResponse.Body.Close()
		return nil, RangedRequestNotSupportedError
	} else {
		return rangedResponse, nil
	}
}
