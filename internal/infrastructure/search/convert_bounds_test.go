package search

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNumericProjectionRejectsOverflowAndNonFiniteValues(t *testing.T) {
	t.Parallel()

	for _, value := range []any{int(-1), int64(-1), float64(-1), math.NaN(), math.Inf(1), math.Inf(-1), json.Number("18446744073709551616")} {
		require.Zero(t, toUint(value), "unsigned projection of %v", value)
	}

	for _, value := range []any{uint64(math.MaxInt64) + 1, json.Number("9223372036854775808"), math.NaN(), math.Inf(1), math.Inf(-1), float64(math.MaxInt64)} {
		require.Zero(t, toInt64(value), "signed projection of %v", value)
	}

	require.EqualValues(t, math.MaxInt64, toInt64(uint64(math.MaxInt64)))
	require.EqualValues(t, 12, toInt64(float64(12.9)))
	require.EqualValues(t, 12, toUint(float64(12.9)))
	require.Zero(t, toInt(uint64(math.MaxInt)+1))
}
