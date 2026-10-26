package identity

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGraphicChallengeUsesIndependentIDsAndValidAnswers(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}

	for kind := range 5 {
		for range 30 {
			id, question, answer, err := newGraphicChallenge(kind)
			require.NoError(t, err)

			raw, err := hex.DecodeString(id)
			require.NoError(t, err, "invalid challenge ID: %q", id)
			require.Len(t, raw, 32, "invalid challenge ID: %q", id)
			require.False(t, seen[id], "invalid challenge ID: %q", id)

			seen[id] = true

			if kind >= 2 {
				assertGraphicStringChallenge(t, kind, question, answer)
			} else {
				assertGraphicMathChallenge(t, question, answer)
			}
		}
	}
}

func assertGraphicStringChallenge(t *testing.T, kind int, question, answer string) {
	t.Helper()

	source := graphicCaptchaAlphanumericSource
	if kind == 3 {
		source = graphicCaptchaDigitSource
	}

	if kind == 4 {
		source = graphicCaptchaLetterSource
	}

	require.Len(t, answer, graphicCaptchaLength, "invalid string challenge")
	require.Equal(t, question, answer, "invalid string challenge")

	for _, char := range answer {
		require.True(t, strings.ContainsRune(source, char), "unexpected answer character")
	}
}

func assertGraphicMathChallenge(t *testing.T, question, answer string) {
	t.Helper()

	var (
		a, b int
		op   rune
	)
	{
		_, err := fmt.Sscanf(question, "%d%c%d=?", &a, &op, &b)
		require.NoError(t, err)
	}

	result := a + b
	if op == '-' {
		result = a - b
	}

	if op == 'x' {
		result = a * b
	}

	require.Equal(t, answer, strconv.Itoa(result), "question=%s answer=%s", question, answer)
}
