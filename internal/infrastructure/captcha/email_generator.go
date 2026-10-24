package captcha

import (
	"crypto/rand"
	"encoding/hex"
	app "f_ccg_video_back_ddd/internal/application/identity"
	"math/big"
)

type EmailGenerator struct{}

var _ app.EmailCaptchaGenerator = (*EmailGenerator)(nil)

func NewEmailGenerator() *EmailGenerator { return &EmailGenerator{} }

// Generate uses rejection sampling from the operating system CSPRNG.
// Empty results fail closed at the application boundary if entropy is unavailable.
func (*EmailGenerator) Generate(length int, charset string) (id, code string) {
	if length <= 0 || charset == "" {
		return "", ""
	}
	var rawID [32]byte
	if _, err := rand.Read(rawID[:]); err != nil {
		return "", ""
	}
	alphabet := []rune(charset)
	answer := make([]rune, length)
	for i := range answer {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", ""
		}
		answer[i] = alphabet[n.Int64()]
	}
	return hex.EncodeToString(rawID[:]), string(answer)
}
