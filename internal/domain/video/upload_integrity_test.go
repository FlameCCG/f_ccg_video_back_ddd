package video

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChunkSetIgnoresOutOfRangeExtras(t *testing.T) {
	t.Parallel()

	valid := func(s string) error { _, err := strconv.Atoi(s); return err }
	{
		err := ValidateChunkSet([]string{"0", "1", "900"}, 2, valid)
		require.NoError(t, err)
	}

	{
		err := ValidateChunkSet([]string{"0", "900"}, 2, valid)
		require.Error(t, err, "missing required chunk accepted")
	}
}
