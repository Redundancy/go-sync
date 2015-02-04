package blocksources

type FixedSizeBlockResolver struct {
	BlockSize             uint64
	MaxDesiredRequestSize uint64
}

func (r *FixedSizeBlockResolver) GetBlockStartOffset(blockID uint) int64 {
	return int64(uint64(blockID) * r.BlockSize)
}

func (r *FixedSizeBlockResolver) GetBlockEndOffset(blockID uint) int64 {
	return int64(uint64(blockID+1) * r.BlockSize)
}

func (r *FixedSizeBlockResolver) SplitBlockRangeToDesiredSize(startBlockID, endBlockID uint) []queuedRequest {

	if r.MaxDesiredRequestSize == 0 {
		return []queuedRequest{
			queuedRequest{
				startBlockID: startBlockID,
				endBlockID:   endBlockID,
			},
		}
	}

	maxSize := r.MaxDesiredRequestSize
	if r.MaxDesiredRequestSize < r.BlockSize {
		maxSize = r.BlockSize
	}

	// how many blocks is the desired size?
	blockCountPerRequest := uint(maxSize / r.BlockSize)

	requests := make([]queuedRequest, 0, (endBlockID-startBlockID)/blockCountPerRequest+1)
	currentBlockID := startBlockID

	for {
		maxEndBlock := currentBlockID + blockCountPerRequest

		if maxEndBlock > endBlockID {
			requests = append(
				requests,
				queuedRequest{
					startBlockID: currentBlockID,
					endBlockID:   endBlockID,
				},
			)

			return requests
		} else {
			requests = append(
				requests,
				queuedRequest{
					startBlockID: currentBlockID,
					endBlockID:   maxEndBlock - 1,
				},
			)

			currentBlockID = maxEndBlock
		}
	}
}
