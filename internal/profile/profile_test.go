package profile

import (
	"strings"
	"testing"
)

func TestReadProfile(t *testing.T) {
	body := `
subscriptions:
  - file: ./a.yaml
template: ./base.yaml
overrides:
  patches:
    - type: yaml-merge
      patch:
        mode: rule
output:
  deterministic: true
  sortKeys: true
  keepComments: false
fetch:
  timeoutSeconds: 10
  retries: 2
  concurrency: 2
  ignoreFailed: true
policy:
  gamePlatformDirect: [steam]
hooks:
  js:
    files: [./hooks/a.js]
    timeoutMs: 500
ruleTemplates: [cn-direct]
`
	p, err := Read(strings.NewReader(body))
	if err != nil {
		t.Fatalf("read profile: %v", err)
	}
	if len(p.Subscriptions) != 1 {
		t.Fatalf("unexpected subscriptions len: %d", len(p.Subscriptions))
	}
	if p.Fetch.TimeoutSeconds != 10 || p.Fetch.Retries != 2 || p.Fetch.Concurrency != 2 {
		t.Fatalf("unexpected fetch config: %+v", p.Fetch)
	}
	if p.Hooks.JS.TimeoutMs != 500 {
		t.Fatalf("unexpected hooks timeout: %d", p.Hooks.JS.TimeoutMs)
	}
	if len(p.RuleTemplates) != 1 || p.RuleTemplates[0] != "cn-direct" {
		t.Fatalf("unexpected rule templates")
	}
}

func TestReadProfileWithNodesFile(t *testing.T) {
	body := `
subscriptions:
  - nodesFile: ./nodes.txt
`
	p, err := Read(strings.NewReader(body))
	if err != nil {
		t.Fatalf("read profile: %v", err)
	}
	if len(p.Subscriptions) != 1 {
		t.Fatalf("unexpected subscriptions len: %d", len(p.Subscriptions))
	}
	if p.Subscriptions[0].NodesFile != "./nodes.txt" {
		t.Fatalf("unexpected nodesFile: %q", p.Subscriptions[0].NodesFile)
	}
}

func TestValidateRequiresInput(t *testing.T) {
	_, err := Read(strings.NewReader(`output: { deterministic: true, sortKeys: true, keepComments: false }`))
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateHookTimeout(t *testing.T) {
	_, err := Read(strings.NewReader(`
subscriptions:
  - file: ./a.yaml
hooks:
  js:
    timeoutMs: -1
`))
	if err == nil {
		t.Fatalf("expected error for negative timeout")
	}
}
