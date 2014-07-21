package blocksources

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

const MB = 1024 * 1024

var RangedRequestNotSupportedError = errors.New("Ranged request not supported (Server did not respond with 206 Status)")
var UrlNotFoundError = errors.New("404 Error on URL")

func NewHttpBlockSource(
	url string,
	concurrentRequests int,
) *BlockSourceBase {
	return NewBlockSourceBase(&HttpRequester{
		url:    url,
		client: http.DefaultClient,
	}, concurrentRequests, 4*MB)
}

// This class provides the implementation of BlockSourceRequester for BlockSourceBase
// this simplifies creating new BlockSources that satisfy the requirements down to
// writing a request function
type HttpRequester struct {
	client *http.Client
	url    string
}

func (r *HttpRequester) DoRequest(startOffset int64, endOffset int64) (data []byte, err error) {
	rangedRequest, err := http.NewRequest("GET", r.url, nil)

	if err != nil {
		return nil, err
	}

	rangeSpecifier := fmt.Sprintf("bytes=%v-%v", startOffset, endOffset-1)
	rangedRequest.ProtoAtLeast(1, 1)
	rangedRequest.Header.Add("Range", rangeSpecifier)
	rangedResponse, err := r.client.Do(rangedRequest)

	if err != nil {
		return nil, err
	}

	defer rangedResponse.Body.Close()

	if rangedResponse.StatusCode == 404 {
		return nil, UrlNotFoundError
	} else if rangedResponse.StatusCode != 206 {
		return nil, RangedRequestNotSupportedError
	} else {
		return ioutil.ReadAll(rangedResponse.Body)
	}
}

func (r *HttpRequester) IsFatal(err error) bool {
	return true
}
