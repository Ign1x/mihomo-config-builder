package configfile

import (
	"strings"
	"testing"
)

func TestMarshalDeterministic(t *testing.T) {
	cfg := map[string]any{
		"b": 1,
		"a": map[string]any{"z": 1, "y": 2},
	}
	out1, err := MarshalYAML(cfg, true, true)
	if err != nil {
		t.Fatalf("marshal1: %v", err)
	}
	out2, err := MarshalYAML(cfg, true, true)
	if err != nil {
		t.Fatalf("marshal2: %v", err)
	}
	if string(out1) != string(out2) {
		t.Fatalf("output not deterministic")
	}
	if !strings.Contains(string(out1), "a:") || !strings.Contains(string(out1), "b:") {
		t.Fatalf("unexpected output")
	}
}

func TestMarshalYAMLNonASCIIAndLiteralUnicodeEscapes(t *testing.T) {
	cfg := map[string]any{
		"name": "节点-香港",
		"note": "keep literal \\u4e2d and \\U0001F600 text",
	}

	out, err := MarshalYAML(cfg, true, true)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "节点-香港") {
		t.Fatalf("non-ascii text should be preserved: %q", s)
	}
	if !strings.Contains(s, "\\u4e2d") {
		t.Fatalf("literal short unicode escape should remain literal: %q", s)
	}
	if !strings.Contains(s, "\\U0001F600") {
		t.Fatalf("literal long unicode escape should remain literal: %q", s)
	}
}
