package gosync

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"net/http"
	"time"

	"github.com/Redundancy/go-sync/blocksources"
	"github.com/Redundancy/go-sync/comparer"
	"github.com/Redundancy/go-sync/filechecksum"
	"github.com/Redundancy/go-sync/indexbuilder"
	"github.com/Redundancy/go-sync/patcher"
	"github.com/Redundancy/go-sync/patcher/sequential"
)

// due to short example strings, use a very small block size
// using one this small in practice would increase your file transfer!
const BLOCK_SIZE = 4

// This is the "file" as described by the authoritive version
const REFERENCE = "The quick brown fox jumped over the lazy dog"

// This is what we have locally. Not too far off, but not correct.
const LOCAL_VERSION = "The qwik brown fox jumped 0v3r the lazy"

var content = bytes.NewReader([]byte(REFERENCE))
var LOCAL_URL = ""
var PORT = 8000

func handler(w http.ResponseWriter, req *http.Request) {
	http.ServeContent(w, req, "", time.Now(), content)
}

// set up a http server locally that will respond predictably to ranged requests
func setupServer() {
	s := http.NewServeMux()
	s.HandleFunc("/content", handler)

	go func() {
		for {
			p := fmt.Sprintf(":%v", PORT)
			LOCAL_URL = "http://localhost" + p

			err := http.ListenAndServe(
				p,
				s,
			)

			if err != nil {
				PORT++
			}
		}
	}()
}

// This is exceedingly similar to the module Example, but uses the http blocksource and a local http server
func Example_httpBlockSource() {
	setupServer()

	generator := filechecksum.NewFileChecksumGenerator(BLOCK_SIZE)
	_, referenceFileIndex, checksumLookup, err := indexbuilder.BuildIndexFromString(generator, REFERENCE)

	if err != nil {
		return
	}

	compare := &comparer.Comparer{}

	// This will result in a stream of blocks that match in the local version
	// to those in the reference
	// We could do this on two goroutines simultaneously, if we used two identical generators
	matchStream := compare.StartFindMatchingBlocks(
		bytes.NewBufferString(LOCAL_VERSION),
		0,
		generator,
		referenceFileIndex,
	)

	merger := &comparer.MatchMerger{}
	merger.StartMergeResultStream(matchStream, BLOCK_SIZE)

	matchingBlockRanges := merger.GetMergedBlocks()
	missingBlockRanges := matchingBlockRanges.GetMissingBlocks(uint(referenceFileIndex.BlockCount) - 1)

	patchedFile := bytes.NewBuffer(make([]byte, 0, len(REFERENCE)))
	remoteReferenceSource := blocksources.NewHttpBlockSource(
		LOCAL_URL+"/content",
		2,
		blocksources.MakeNullFixedSizeResolver(BLOCK_SIZE),
		&filechecksum.HashVerifier{
			Hash:                md5.New(),
			BlockSize:           BLOCK_SIZE,
			BlockChecksumGetter: checksumLookup,
		},
	)

	err = sequential.SequentialPatcher(
		bytes.NewReader([]byte(LOCAL_VERSION)),
		remoteReferenceSource,
		ToPatcherMissingSpan(missingBlockRanges, BLOCK_SIZE),
		ToPatcherFoundSpan(matchingBlockRanges, BLOCK_SIZE),
		1024,
		patchedFile,
	)

	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("Patched content: \"%v\"\n", patchedFile.String())
	fmt.Printf("Downloaded Bytes: %v\n", remoteReferenceSource.ReadBytes())

	// Output:
	// Patched content: "The quick brown fox jumped over the lazy dog"
	// Downloaded Bytes: 16
}

func ToPatcherFoundSpan(sl comparer.BlockSpanList, blockSize int64) []patcher.FoundBlockSpan {
	result := make([]patcher.FoundBlockSpan, len(sl))

	for i, v := range sl {
		result[i].StartBlock = v.StartBlock
		result[i].EndBlock = v.EndBlock
		result[i].MatchOffset = v.ComparisonStartOffset
		result[i].BlockSize = blockSize
	}

	return result
}

func ToPatcherMissingSpan(sl comparer.BlockSpanList, blockSize int64) []patcher.MissingBlockSpan {
	result := make([]patcher.MissingBlockSpan, len(sl))

	for i, v := range sl {
		result[i].StartBlock = v.StartBlock
		result[i].EndBlock = v.EndBlock
		result[i].BlockSize = blockSize
	}

	return result
}
