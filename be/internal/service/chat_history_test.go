package service

import "testing"

func TestEstimateChatTokens(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		text string
		want int
	}{
		{name: "empty", text: "", want: 1},
		{name: "short ascii", text: "hi", want: 1},
		{name: "four ascii", text: "abcd", want: 1},
		{name: "five ascii", text: "abcde", want: 2},
		{name: "cjk", text: "你好世界", want: 4},
		{name: "mixed", text: "abc你好", want: 3},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := estimateChatTokens(testCase.text); got != testCase.want {
				t.Fatalf("estimateChatTokens(%q) = %d, want %d", testCase.text, got, testCase.want)
			}
		})
	}
}

func TestChatTokenUsageClampsRemaining(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		used        int
		wantRemain  int
		wantReached bool
	}{
		{name: "empty", used: 0, wantRemain: 50_000, wantReached: false},
		{name: "below", used: 49_999, wantRemain: 1, wantReached: false},
		{name: "at limit", used: 50_000, wantRemain: 0, wantReached: true},
		{name: "over", used: 50_001, wantRemain: 0, wantReached: true},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			usage := chatTokenUsage(testCase.used)
			if usage.Used != testCase.used || usage.Limit != 50_000 {
				t.Fatalf("unexpected usage %+v for used %d", usage, testCase.used)
			}
			if usage.Remaining != testCase.wantRemain {
				t.Fatalf("Remaining = %d, want %d", usage.Remaining, testCase.wantRemain)
			}
			if usage.LimitReached != testCase.wantReached {
				t.Fatalf("LimitReached = %v, want %v", usage.LimitReached, testCase.wantReached)
			}
		})
	}
}
