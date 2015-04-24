package blocksources

import (
	"bytes"
	"github.com/Redundancy/go-sync/patcher"

	//"runtime"
	"testing"
	"time"
)

//-----------------------------------------------------------------------------
type erroringRequester struct{}
type testError struct{}

func (e *testError) Error() string {
	return "test"
}

func (e *erroringRequester) DoRequest(startOffset int64, endOffset int64) (data []byte, err error) {
	return nil, &testError{}
}

func (e *erroringRequester) IsFatal(err error) bool {
	return true
}

//-----------------------------------------------------------------------------
type FunctionRequester func(a, b int64) ([]byte, error)

func (f FunctionRequester) DoRequest(startOffset int64, endOffset int64) (data []byte, err error) {
	return f(startOffset, endOffset)
}

func (f FunctionRequester) IsFatal(err error) bool {
	return true
}

//-----------------------------------------------------------------------------

func init() {
	//if runtime.GOMAXPROCS(0) == 1 {
	//runtime.GOMAXPROCS(4)
	//}
}

func TestRangeSlice(t *testing.T) {
	a := []int{0, 1, 2, 3, 4}
	b := a[:len(a)-1]

	if len(b) != len(a)-1 {
		t.Errorf("b is wrong length, only supposed to remove one item: %v %v", a, b)
	}
}

func TestCreateAndCloseBlockSourceBase(t *testing.T) {
	b := NewBlockSourceBase(nil, nil, nil, 1, 1024)
	b.Close()

	// TODO: Race condition here. Can Close() block?
	if !b.hasQuit {
		t.Fatal("Block source base did not exit")
	}
}

func TestErrorWatcher(t *testing.T) {
	e := errorWatcher{errorChannel: make(chan error)}

	if e.sendIfSet() != nil {
		t.Errorf("Channel should be nil when created")
	}

	e.setError(&testError{})

	if e.sendIfSet() == nil {
		t.Errorf("Channel should be non-nil when error is set")
	}
	if e.Err() == nil {
		t.Errorf("Error should not be nil when set")
	}
}

func TestBlockSourceBaseError(t *testing.T) {
	b := NewBlockSourceBase(
		&erroringRequester{},
		MakeNullFixedSizeResolver(4),
		nil,
		1,
		1024,
	)
	defer b.Close()

	b.RequestBlocks(patcher.MissingBlockSpan{
		BlockSize:  4,
		StartBlock: 1,
		EndBlock:   1,
	})

	select {
	case <-time.After(time.Second):
		t.Fatal("Timed out waiting for error")
	case <-b.EncounteredError():
	}

}

func TestBlockSourceRequest(t *testing.T) {
	expected := []byte("test")

	b := NewBlockSourceBase(
		FunctionRequester(func(start, end int64) (data []byte, err error) {
			return expected, nil
		}),
		MakeNullFixedSizeResolver(4),
		nil,
		1,
		1024,
	)
	defer b.Close()

	b.RequestBlocks(patcher.MissingBlockSpan{
		BlockSize:  4,
		StartBlock: 1,
		EndBlock:   1,
	})

	result := <-b.GetResultChannel()

	if result.StartBlock != 1 {
		t.Errorf("Unexpected start block in result: %v", result.StartBlock)
	}
	if bytes.Compare(result.Data, expected) != 0 {
		t.Errorf("Unexpected data in result: %v", result.Data)
	}
}

func TestConcurrentBlockRequests(t *testing.T) {
	content := []byte("test")

	b := NewBlockSourceBase(
		FunctionRequester(func(start, end int64) (data []byte, err error) {
			return content[start:end], nil
		}),
		MakeNullFixedSizeResolver(2),
		nil,
		2,
		1024,
	)
	defer b.Close()

	b.RequestBlocks(patcher.MissingBlockSpan{
		BlockSize:  2,
		StartBlock: 0,
		EndBlock:   0,
	})

	b.RequestBlocks(patcher.MissingBlockSpan{
		BlockSize:  2,
		StartBlock: 1,
		EndBlock:   1,
	})

	for i := uint(0); i < 2; i++ {
		select {
		case r := <-b.GetResultChannel():
			if r.StartBlock != i {
				t.Errorf("Wrong start block: %v", r.StartBlock)
			}
			if bytes.Compare(r.Data, content[i*2:(i+1)*2]) != 0 {
				t.Errorf("Unexpected result content for result %v: %v", i+1, string(r.Data))
			}
		case <-time.After(time.Second):
			t.Fatal("Timed out on request", i+1)
		}
	}
}

func TestOutOfOrderRequestCompletion(t *testing.T) {
	content := []byte("test")

	channeler := []chan bool{
		make(chan bool),
		make(chan bool),
	}

	b := NewBlockSourceBase(
		FunctionRequester(func(start, end int64) (data []byte, err error) {
			// read from the channel based on the start
			<-(channeler[start])
			return content[start:end], nil
		}),
		MakeNullFixedSizeResolver(1),
		nil,
		2,
		1024,
	)
	defer b.Close()

	b.RequestBlocks(patcher.MissingBlockSpan{
		BlockSize:  1,
		StartBlock: 0,
		EndBlock:   0,
	})

	b.RequestBlocks(patcher.MissingBlockSpan{
		BlockSize:  1,
		StartBlock: 1,
		EndBlock:   1,
	})

	// finish the second request
	channeler[1] <- true

	select {
	case <-b.GetResultChannel():
		t.Error("Should not deliver any blocks yet")
	case <-time.After(time.Second):
	}

	// once the first block completes, we're ready to send both
	channeler[0] <- true

	for i := uint(0); i < 2; i++ {
		select {
		case r := <-b.GetResultChannel():
			if r.StartBlock != i {
				t.Errorf(
					"Wrong start block: %v on result %v",
					r.StartBlock,
					i+1,
				)
			}
		case <-time.After(time.Second):
			t.Fatal("Timed out on request", i+1)
		}
	}
}

func TestRequestCountLimiting(t *testing.T) {
	counter := make(chan int)
	waiter := make(chan bool)
	const (
		MAX_CONCURRENCY = 2
		REQUESTS        = 4
	)
	call_counter := 0

	b := NewBlockSourceBase(
		FunctionRequester(func(start, end int64) (data []byte, err error) {
			counter <- 1
			call_counter += 1
			<-waiter
			counter <- -1
			return []byte{0, 0}, nil
		}),
		MakeNullFixedSizeResolver(1),
		nil,
		MAX_CONCURRENCY,
		1024,
	)
	defer b.Close()

	count := 0
	max := 0

	go func() {
		for {
			change, ok := <-counter

			if !ok {
				break
			}

			count += change

			if count > max {
				max = count
			}
		}
	}()

	for i := 0; i < REQUESTS; i++ {
		b.RequestBlocks(patcher.MissingBlockSpan{
			BlockSize:  1,
			StartBlock: uint(i),
			EndBlock:   uint(i),
		})
	}

	for i := 0; i < REQUESTS; i++ {
		waiter <- true
	}

	close(counter)
	close(waiter)

	if max > MAX_CONCURRENCY {
		t.Errorf("Maximum requests in flight was greater than the requested concurrency: %v", max)
	}
	if call_counter != REQUESTS {
		t.Errorf("Total number of requests is not expected: %v", call_counter)
	}
}
