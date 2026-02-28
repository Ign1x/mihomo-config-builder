package configfile

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func ReadYAMLFile(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read yaml file: %w", err)
	}
	return DecodeYAMLBytes(b)
}

func DecodeYAMLBytes(data []byte) (map[string]any, error) {
	var v any
	if err := yaml.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("decode yaml: %w", err)
	}
	norm, err := Normalize(v)
	if err != nil {
		return nil, err
	}
	m, ok := norm.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("yaml root must be mapping")
	}
	return m, nil
}

func DecodeYAMLBytesAny(data []byte) (any, error) {
	var v any
	if err := yaml.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("decode yaml: %w", err)
	}
	return Normalize(v)
}

func MarshalYAML(v any, deterministic bool, sortKeys bool) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	defer enc.Close()

	norm, err := Normalize(v)
	if err != nil {
		return nil, err
	}
	node, err := ToNode(norm, deterministic || sortKeys)
	if err != nil {
		return nil, err
	}
	if err := enc.Encode(node); err != nil {
		return nil, fmt.Errorf("encode yaml: %w", err)
	}
	return buf.Bytes(), nil
}
