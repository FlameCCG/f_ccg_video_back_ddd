package identity

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func TestGraphicChallengeUsesIndependentIDsAndValidAnswers(t *testing.T) {
	seen := map[string]bool{}
	for kind := 0; kind <= 4; kind++ {
		for range 30 {
			id, question, answer, err := newGraphicChallenge(kind)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := hex.DecodeString(id)
			if err != nil || len(raw) != 32 || seen[id] {
				t.Fatalf("invalid challenge ID: %q", id)
			}
			seen[id] = true
			if kind >= 2 {
				source := graphicCaptchaAlphanumericSource
				if kind == 3 {
					source = graphicCaptchaDigitSource
				}
				if kind == 4 {
					source = graphicCaptchaLetterSource
				}
				if len(answer) != graphicCaptchaLength || question != answer {
					t.Fatal("invalid string challenge")
				}
				for _, char := range answer {
					if !strings.ContainsRune(source, char) {
						t.Fatal("unexpected answer character")
					}
				}
			} else {
				var a, b int
				var op rune
				if _, err := fmt.Sscanf(question, "%d%c%d=?", &a, &op, &b); err != nil {
					t.Fatal(err)
				}
				result := a + b
				if op == '-' {
					result = a - b
				}
				if op == 'x' {
					result = a * b
				}
				if answer != strconv.Itoa(result) {
					t.Fatalf("question=%s answer=%s", question, answer)
				}
			}
		}
	}
}
