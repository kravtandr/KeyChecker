package provider

import "testing"

func TestMask(t *testing.T) {
	cases := map[string]string{
		"sk-ant-api03-ABCDEF1234567890": "sk-ant...7890",
		"short":                         "***",
		"":                              "***",
	}
	for in, want := range cases {
		if got := Mask(in); got != want {
			t.Errorf("Mask(%q) = %q, want %q", in, got, want)
		}
	}
}
