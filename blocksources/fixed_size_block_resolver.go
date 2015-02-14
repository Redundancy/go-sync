package blocksources

type FixedSizeBlockResolver struct {
	BlockSize             uint64
	MaxDesiredRequestSize uint64
}

func (r *FixedSizeBlockResolver) GetBlockStartOffset(blockID uint) int64 {
	return int64(uint64(blockID) * r.BlockSize)
}

func (r *FixedSizeBlockResolver) GetBlockEndOffset(blockID uint) int64 {
	// TODO: should really take into account the maximum size of the file and potential partial blocks at the end
	return int64(uint64(blockID+1) * r.BlockSize)
}

// Split blocks into chunks of the desired size, or less. This implementation assumes a fixed block size at the source.
func (r *FixedSizeBlockResolver) SplitBlockRangeToDesiredSize(startBlockID, endBlockID uint) []QueuedRequest {

	if r.MaxDesiredRequestSize == 0 {
		return []QueuedRequest{
			QueuedRequest{
				StartBlockID: startBlockID,
				EndBlockID:   endBlockID,
			},
		}
	}

	maxSize := r.MaxDesiredRequestSize
	if r.MaxDesiredRequestSize < r.BlockSize {
		maxSize = r.BlockSize
	}

	// how many blocks is the desired size?
	blockCountPerRequest := uint(maxSize / r.BlockSize)

	requests := make([]QueuedRequest, 0, (endBlockID-startBlockID)/blockCountPerRequest+1)
	currentBlockID := startBlockID

	for {
		maxEndBlock := currentBlockID + blockCountPerRequest

		if maxEndBlock > endBlockID {
			requests = append(
				requests,
				QueuedRequest{
					StartBlockID: currentBlockID,
					EndBlockID:   endBlockID,
				},
			)

			return requests
		} else {
			requests = append(
				requests,
				QueuedRequest{
					StartBlockID: currentBlockID,
					EndBlockID:   maxEndBlock - 1,
				},
			)

			currentBlockID = maxEndBlock
		}
	}
}
