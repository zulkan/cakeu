package service

import "goskeleton/internal/model"

// estimateChatTokens approximates the conversation budget of a message:
// roughly four ASCII characters per token, one token per non-ASCII rune,
// with a minimum of one token.
func estimateChatTokens(text string) int {
	ascii := 0
	nonASCII := 0

	for _, r := range text {
		if r < 0x80 {
			ascii++
			continue
		}
		nonASCII++
	}

	tokens := (ascii+3)/4 + nonASCII
	if tokens < 1 {
		return 1
	}

	return tokens
}

func chatTokenUsage(used int) model.ChatTokenUsage {
	remaining := chatTokenLimit - used
	if remaining < 0 {
		remaining = 0
	}

	return model.ChatTokenUsage{
		Used:         used,
		Limit:        chatTokenLimit,
		Remaining:    remaining,
		LimitReached: used >= chatTokenLimit,
	}
}
