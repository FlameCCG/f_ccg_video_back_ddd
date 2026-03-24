package ai

import "slices"

// HistoryMessage 只表达历史选择所需的角色与文本成本，不包含模型 SDK 或协议类型。
type HistoryMessage struct {
	Present, System, User bool
	TextRunes             int
}
type HistoryPolicy struct{}

// QueryImages 选择本轮用于检索的图片，沿用最多三张的政策。
func (HistoryPolicy) QueryImages(images []string) []string {
	count := min(len(images), 3)
	return slices.Clone(images[:count])
}

// AnalysisPositions 先取最后若干条原始消息，再排除缺失与系统消息。
// 截取顺序不能改成过滤后截取，否则会改变意图分析使用的历史范围。
func (HistoryPolicy) AnalysisPositions(facts []HistoryMessage, maximum int) []int {
	start := 0
	if maximum > 0 && len(facts) > maximum {
		start = len(facts) - maximum
	}
	positions := make([]int, 0, len(facts)-start)
	for i := start; i < len(facts); i++ {
		if facts[i].Present && !facts[i].System {
			positions = append(positions, i)
		}
	}
	return positions
}

func (HistoryPolicy) Limits(turns, chars int) (int, int) {
	if turns <= 0 {
		turns = 20
	}
	if chars <= 0 {
		chars = 24000
	}
	return turns, chars
}

// Select 保留系统消息，按条数和字符预算从后向前取历史，再丢弃开头非用户消息。
// 最新消息即使超出预算仍先保留，沿用已有选择顺序。
func (p HistoryPolicy) Select(facts []HistoryMessage, turns, chars int) []int {
	turns, chars = p.Limits(turns, chars)
	system := make([]int, 0, len(facts))
	messages := make([]int, 0, len(facts))
	for i, f := range facts {
		if !f.Present {
			continue
		}
		if f.System {
			system = append(system, i)
		} else {
			messages = append(messages, i)
		}
	}
	if len(messages) > turns {
		messages = messages[len(messages)-turns:]
	}
	budget, start := chars, len(messages)
	for i := len(messages) - 1; i >= 0; i-- {
		cost := facts[messages[i]].TextRunes
		if budget-cost < 0 && i != len(messages)-1 {
			break
		}
		budget -= cost
		start = i
	}
	messages = messages[start:]
	for len(messages) > 0 && !facts[messages[0]].User {
		messages = messages[1:]
	}
	return append(system, messages...)
}
