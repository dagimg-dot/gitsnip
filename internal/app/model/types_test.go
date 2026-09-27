package model

import "testing"

func TestParseMethod(t *testing.T) {
	cases := []struct {
		raw  string
		want Method
	}{
		{"", MethodAuto},
		{"auto", MethodAuto},
		{"sparse", MethodSparse},
		{"API", MethodAPI},
		{"  api  ", MethodAPI},
	}
	for _, tc := range cases {
		if got, err := ParseMethod(tc.raw); err != nil || got != tc.want {
			t.Errorf("ParseMethod(%q) = %q, %v", tc.raw, got, err)
		}
	}
	if _, err := ParseMethod("git"); err == nil {
		t.Error(`ParseMethod("git") should fail`)
	}
}
