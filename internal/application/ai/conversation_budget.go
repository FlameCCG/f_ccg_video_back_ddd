package ai

import (
	"unicode/utf8"

	"f_ccg_video_back_ddd/internal/config"
	aidomain "f_ccg_video_back_ddd/internal/domain/ai"
)

func (c conversation) validateInput(cfg *config.AI) error {
	for _, fact := range c.historyFacts() {
		if err := (aidomain.InputPolicy{}).ValidateMessage(fact.TextRunes, cfg.InputMaxChars); err != nil {
			return err
		}
	}
	return nil
}

func (c conversation) validateContext(cfg *config.AI) error {
	chars := 0
	for _, message := range c.modelMessages() {
		chars += utf8.RuneCountInString(message.Content)
	}
	return (aidomain.InputPolicy{}).ValidateContext(chars, cfg.ContextMaxChars)
}
