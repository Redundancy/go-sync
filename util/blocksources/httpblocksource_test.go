package blocksources

import (
	"bytes"
	"fmt"
	"github.com/Redundancy/go-sync/patcher"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"
)

var PORT = 8000

var TEST_CONTENT = []byte("This is test content used for evaluation of the unit tests")
var content = bytes.NewReader(TEST_CONTENT)
var LOCAL_URL = ""

func handler(w http.ResponseWriter, req *http.Request) {
	http.ServeContent(w, req, "", time.Now(), content)
}

var PARTIAL_CONTENT = []byte("abcdef")
var partialContent = bytes.NewReader(PARTIAL_CONTENT)

func partialContentHandler(w http.ResponseWriter, req *http.Request) {
	http.ServeContent(w, req, "", time.Now(), partialContent)
}

// set up a http server locally that will respond predictably to ranged requests
func init() {
	s := http.NewServeMux()
	s.HandleFunc("/", handler)
	s.HandleFunc("/partial", partialContentHandler)
	s.Handle("/404", http.NotFoundHandler())

	go func() {
		for {
			p := fmt.Sprintf(":%v", PORT)
			LOCAL_URL = "http://localhost" + p

			err := http.ListenAndServe(
				p,
				s,
			)

			if err != nil {
				PORT += 1
			}
		}
	}()

}

// ensure that a ranged request is implemented correctly
func TestRangedRequest(t *testing.T) {
	// URL can be anything that supports HTTP 1.1 and has enough content to support an offset request
	const (
		URL                      = "http://farm3.static.flickr.com/2390/2253727548_a413c88ab3_s.jpg"
		START_OFFSET             = 5
		END_OFFSET               = 10
		EXPECTED_RESPONSE_LENGTH = END_OFFSET - START_OFFSET + 1
	)

	standardResponse, err := http.Get(URL)

	if err != nil {
		t.Fatal(err)
	}

	defer standardResponse.Body.Close()

	if standardResponse.StatusCode > 299 {
		t.Fatal("Status:" + standardResponse.Status)
	}

	acceptableRanges := standardResponse.Header.Get("Accept-Ranges")

	if acceptableRanges == "none" {
		t.Fatal("Server does not accept ranged requests")
	} else if acceptableRanges == "" {
		t.Log("Server has not responded with the 'Accept-Ranges' header")
	} else {
		t.Logf("Accept-Ranges=%v", acceptableRanges)
	}

	rangedResponse, err := rangedRequest(
		http.DefaultClient,
		URL,
		START_OFFSET,
		END_OFFSET,
	)

	if err != nil {
		t.Fatal(err)
	}

	rangedResponseBody := rangedResponse.Body
	defer rangedResponseBody.Close()

	l := io.LimitReader(rangedResponseBody, 200)
	buffer := make([]byte, 200)

	n, err := io.ReadFull(l, buffer)
	t.Logf("Read %v bytes", n)

	if err != nil && err != io.ErrUnexpectedEOF {
		t.Fatal(err)
	}

	if n != EXPECTED_RESPONSE_LENGTH {
		t.Fatalf(
			"Unexpected response length: %v vs %v",
			n,
			EXPECTED_RESPONSE_LENGTH)
	}

	return
}

func TestRangedRequestErrorsWhenNotSupported(t *testing.T) {
	const URL = "http://google.com"

	_, err := rangedRequest(
		http.DefaultClient,
		URL,
		0,
		100,
	)

	if err == nil {
		t.Fatal(URL + " does not support ranged requests (or didn't!)")
	}
}

func TestNoResponseError(t *testing.T) {
	const URL = "http://foo.bar/"

	_, err := rangedRequest(
		http.DefaultClient,
		URL,
		0,
		100,
	)

	switch err.(type) {
	case *url.Error:
	default:
		t.Fatalf("%#v", err)
	}
}

func TestHandler(t *testing.T) {
	resp, err := http.Get(LOCAL_URL)

	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		t.Fatal(resp.Status)
	}
}

func TestHttpBlockSource(t *testing.T) {
	b := NewHttpBlockSource(LOCAL_URL+"/", 2)

	err := b.RequestBlock(patcher.MissingBlockSpan{
		BlockSize:  4,
		StartBlock: 0,
		EndBlock:   0,
	})

	if err != nil {
		t.Fatal(err)
	}

	results := b.GetResultChannel()

	select {
	case r := <-results:
		if bytes.Compare(r.Data, TEST_CONTENT[:4]) != 0 {
			t.Errorf("Data differed from expected content: \"%v\"", string(r.Data))
		}
	case e := <-b.EncounteredError():
		t.Fatal(e)
	case <-time.After(time.Second):
		t.Fatal("Waited a second for the response, timeout.")
	}
}

func TestHttpBlockSource404(t *testing.T) {
	b := NewHttpBlockSource(LOCAL_URL+"/404", 2)

	b.RequestBlock(patcher.MissingBlockSpan{
		BlockSize:  4,
		StartBlock: 0,
		EndBlock:   0,
	})

	results := b.GetResultChannel()

	select {
	case <-results:
		t.Fatal("Should not have gotten a result")
	case e := <-b.EncounteredError():
		if e == nil {
			t.Fatal("Error was nil!")
		} else if e != UrlNotFoundError {
			t.Errorf("Unexpected error type: %v", e)
		}
	case <-time.After(time.Second):
		t.Fatal("Waited a second for the response, timeout.")
	}
}

func TestHttpBlockSourceOffsetBlockRequest(t *testing.T) {
	b := NewHttpBlockSource(LOCAL_URL+"/", 2)

	b.RequestBlock(patcher.MissingBlockSpan{
		BlockSize:  4,
		StartBlock: 1,
		EndBlock:   3,
	})

	select {
	case result := <-b.GetResultChannel():
		if result.StartBlock != 1 {
			t.Errorf(
				"Unexpected result start block: %v",
				result.StartBlock,
			)
		}
	case <-time.After(time.Second):
		t.Fatalf("Timeout waiting for result")
	}
}

func TestMultipleRequestOrdering(t *testing.T) {
	b := NewHttpBlockSource(LOCAL_URL+"/", 2)

	b.RequestBlock(patcher.MissingBlockSpan{
		BlockSize:  4,
		StartBlock: 1,
		EndBlock:   3,
	})

	b.RequestBlock(patcher.MissingBlockSpan{
		BlockSize:  4,
		StartBlock: 0,
		EndBlock:   0,
	})

	time.Sleep(time.Millisecond * 600)

	results := b.GetResultChannel()

	EXPECTED_START_BLOCKS := []uint{
		0,
		1,
	}

	for i := 0; i < 2; i++ {
		select {
		case result := <-results:
			if result.StartBlock != EXPECTED_START_BLOCKS[i] {
				t.Errorf(
					"Unexpected result start block (%v): %v",
					i,
					result.StartBlock,
				)
			}
		case <-time.After(time.Second):
			t.Fatalf("Timeout waiting for result %v", i+1)
		}
	}

}

func TestHttpBlockSourcePartialContentRequest(t *testing.T) {
	b := NewHttpBlockSource(LOCAL_URL+"/partial", 2)

	b.RequestBlock(patcher.MissingBlockSpan{
		BlockSize:  4,
		StartBlock: 1,
		EndBlock:   1,
	})

	select {
	case result := <-b.GetResultChannel():
		if result.StartBlock != 1 {
			t.Errorf(
				"Unexpected result start block: %v",
				result.StartBlock,
			)
		}
		if len(result.Data) != 2 {
			t.Errorf(
				"Unexpected data length: \"%v\"",
				string(result.Data),
			)
		}
		if string(result.Data) != "ef" {
			t.Errorf(
				"Unexpected result \"%v\"",
				string(result.Data),
			)
		}
	case err := <-b.EncounteredError():
		t.Fatal(err)
	case <-time.After(time.Second):
		t.Fatalf("Timeout waiting for result")
	}
}
