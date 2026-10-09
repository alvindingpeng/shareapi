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
