package model

import "testing"

func TestParseMethod(t *testing.T) {
	cases := map[string]Method{"": MethodAuto, "auto": MethodAuto, "sparse": MethodSparse, "API": MethodAPI, " api ": MethodAPI}
	for raw, want := range cases {
		if got, err := ParseMethod(raw); err != nil || got != want {
			t.Errorf("ParseMethod(%q) = %q, %v", raw, got, err)
		}
	}
	if _, err := ParseMethod("git"); err == nil {
		t.Error(`ParseMethod("git") should fail`)
	}
}
