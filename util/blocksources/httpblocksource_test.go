package blocksources

import (
	"bytes"
	"fmt"
	"github.com/Redundancy/go-sync/patcher"
	"net/http"
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
// NB: Doing this will prevent deadlocks from being caught!
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
