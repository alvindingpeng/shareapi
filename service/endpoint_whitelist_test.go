package service

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
)

func TestIsOfficialEndpoint(t *testing.T) {
	cases := []struct {
		typ      int
		baseURL  string
		official bool
		known    bool
	}{
		{constant.ChannelTypeOpenAI, "https://api.openai.com/v1", true, true},
		{constant.ChannelTypeOpenAI, "https://evil-relay.example.com/v1", false, true},
		{constant.ChannelTypeGemini, "https://generativelanguage.googleapis.com", true, true},
		{constant.ChannelTypeGemini, "https://proxy.example.com", false, true},
		{constant.ChannelTypeXai, "https://api.x.ai/v1", true, true},
		{constant.ChannelTypeDeepSeek, "https://api.deepseek.com", true, true},
		// Unknown type: no whitelist.
		{999, "https://api.openai.com/v1", false, false},
		// Empty base URL.
		{constant.ChannelTypeOpenAI, "", false, true},
		// Port-stripped match.
		{constant.ChannelTypeOpenAI, "https://api.openai.com:443/v1", true, true},
	}
	for _, tc := range cases {
		official, known := IsOfficialEndpoint(tc.typ, tc.baseURL)
		if official != tc.official || known != tc.known {
			t.Errorf("IsOfficialEndpoint(%d, %q) = (%v, %v), want (%v, %v)",
				tc.typ, tc.baseURL, official, known, tc.official, tc.known)
		}
	}
}
