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
