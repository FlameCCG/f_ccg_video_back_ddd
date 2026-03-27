package ai

// InputPolicy 限制单条输入和最终上下文，历史窗口裁剪不能替代这些硬上限。
// 字符预算是跨模型的保守保护，不声称等同于提供方的精确 token 数。
type InputPolicy struct{}

func (InputPolicy) Limits(messageChars, contextChars int) (int, int) {
	if messageChars <= 0 {
		messageChars = 24000
	}
	if contextChars <= 0 {
		contextChars = 48000
	}
	return messageChars, contextChars
}

func (p InputPolicy) ValidateMessage(chars, configured int) error {
	limit, _ := p.Limits(configured, 0)
	if chars > limit {
		return ErrInputTooLong
	}
	return nil
}

func (p InputPolicy) ValidateContext(chars, configured int) error {
	_, limit := p.Limits(0, configured)
	if chars > limit {
		return ErrContextTooLong
	}
	return nil
}
