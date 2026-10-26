package identity

import (
	"math"
	"time"
)

// configuredDuration converts a positive unit count without allowing duration overflow.
// Invalid configuration expires immediately rather than creating an unbounded credential.
func configuredDuration(count int64, unit time.Duration) time.Duration {
	if count <= 0 || unit <= 0 || count > math.MaxInt64/int64(unit) {
		return 0
	}

	return time.Duration(count) * unit
}
