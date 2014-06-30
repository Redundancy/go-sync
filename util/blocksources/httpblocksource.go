package blocksources

import (
	"errors"
	"fmt"
	"github.com/Redundancy/go-sync/patcher"
	"io/ioutil"
	"net/http"
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

func (s *HttpBlockSource) loop() {
	defer close(s.errorChan)
	defer close(s.responseChan)
	//pendingResponses := make([]patcher.BlockReponse, 0, 10)
	inflightRequests := 0

	// Set to nil when there is nothing to send
	var responseChan chan patcher.BlockReponse = nil
	var errorChan chan error = nil
	var currentError error = nil
	var firstResponse patcher.BlockReponse
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
			)

			inflightRequests += 1
			if inflightRequests == s.concurrentRequests {
				requestChan = nil
			}

		case responseChan <- firstResponse:
			responseChan = nil
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
			} else if data, err = ioutil.ReadAll(response.reponse.Body); err != nil {
				fatalError = true
				errorChan = s.errorChan
				currentError = err

				// Prevent any more requests from being filled
				requestChan = nil
			}

			inflightRequests -= 1

			if !fatalError && requestChan == nil {
				// clearly we were full, but now have a free request slot
				requestChan = s.requestChan
			}

			firstResponse = patcher.BlockReponse{
				Data: data,
			}
			if responseChan == nil {
				responseChan = s.responseChan
			}

		case errorChan <- currentError:
			errorChan = nil
			currentError = nil
		}
	}
}

func (s *HttpBlockSource) startRequest(startOffset, endOffset int64) {
	go func() {
		req, err := rangedRequest(
			s.client,
			s.url,
			startOffset,
			endOffset,
		)

		s.httpResponseChan <- httpResponse{err, req}
	}()
}

var RangedRequestNotSupportedError = errors.New("Ranged request not supported (Server did not respond with 206 Status)")
var UrlNotFoundError = errors.New("404 Error on URL")

func rangedRequest(client *http.Client, url string, startOffset int64, endOffset int64) (*http.Response, error) {
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
