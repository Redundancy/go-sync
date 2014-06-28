package blocksources

import (
	"io"
	"net/http"
	"testing"
)

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

	t.Fatalf("%#v", err)
}
