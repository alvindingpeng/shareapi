package service

import "testing"

func TestModelsMatch(t *testing.T) {
	cases := []struct {
		claimed string
		echoed  string
		want    bool
	}{
		{"gpt-4o", "gpt-4o", true},
		{"gpt-4o", "gpt-4o-2024-05-13", true},   // dated variant
		{"gpt-4o-mini", "gpt-4o-mini-2024-07-18", true},
		{"gpt-4o", "gpt-4o-mini", false},       // dilution: mini served as full
		{"gpt-4o-mini", "gpt-4o", false},       // reverse should not happen either
		{"claude-3-5-sonnet", "claude-3-5-haiku", false},
		{"", "gpt-4o", false},
		{"gpt-4o", "", false},
	}
	for _, tc := range cases {
		if got := modelsMatch(tc.claimed, tc.echoed); got != tc.want {
			t.Errorf("modelsMatch(%q, %q) = %v, want %v", tc.claimed, tc.echoed, got, tc.want)
		}
	}
}

func TestIdentityMatchesModel(t *testing.T) {
	cases := []struct {
		response string
		claimed  string
		want     bool
	}{
		{"I am GPT-4, a large language model developed by OpenAI.", "gpt-4", true},
		{"I am Claude, an AI assistant created by Anthropic.", "gpt-4", false},
		{"I am Muse, created by Anthropic.", "claude-3-opus", true},
		{"I am a large language model from OpenAI.", "claude-3-opus", false},
		{"I am Gemini, developed by Google.", "gemini-pro", true},
		{"I am DeepSeek, an AI assistant.", "deepseek-chat", true},
		// Unknown model family: can't judge, treat as consistent.
		{"I am some custom model.", "my-custom-model", true},
	}
	for _, tc := range cases {
		if got := identityMatchesModel(tc.response, tc.claimed); got != tc.want {
			t.Errorf("identityMatchesModel(%q, %q) = %v, want %v", tc.response, tc.claimed, got, tc.want)
		}
	}
}
