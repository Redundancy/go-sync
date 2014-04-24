package readers

import (
	"io"
)

// Injects the second reader into the first at an offset
func InjectedReader(
	offsetFromStart int64,
	base io.Reader,
	inject io.Reader,
) io.Reader {
	return io.MultiReader(
		io.LimitReader(base, offsetFromStart),
		inject,
		base,
	)
}
