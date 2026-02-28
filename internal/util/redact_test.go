package util

import "testing"

func TestRedactURL(t *testing.T) {
	in := "https://example.com/sub?token=abc&foo=bar"
	got := RedactURL(in)
	want := "https://example.com/sub?foo=REDACTED&token=REDACTED"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
