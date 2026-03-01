package profile

import (
	"errors"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

type Profile struct {
	Subscriptions []SourceRef      `yaml:"subscriptions" json:"subscriptions"`
	Template      string           `yaml:"template,omitempty" json:"template,omitempty"`
	Overrides     OverrideConfig   `yaml:"overrides,omitempty" json:"overrides,omitempty"`
	Output        OutputOptions    `yaml:"output,omitempty" json:"output,omitempty"`
	Fetch         FetchOptions     `yaml:"fetch,omitempty" json:"fetch,omitempty"`
	Policy        PolicyDirectives `yaml:"policy,omitempty" json:"policy,omitempty"`
	Hooks         HookConfig       `yaml:"hooks,omitempty" json:"hooks,omitempty"`
	RuleTemplates []string         `yaml:"ruleTemplates,omitempty" json:"ruleTemplates,omitempty"`
}

type SourceRef struct {
	URL       string `yaml:"url,omitempty" json:"url,omitempty"`
	File      string `yaml:"file,omitempty" json:"file,omitempty"`
	NodesFile string `yaml:"nodesFile,omitempty" json:"nodesFile,omitempty"`
}

type OverrideConfig struct {
	Patches []PatchSpec `yaml:"patches,omitempty" json:"patches,omitempty"`
	Files   []string    `yaml:"files,omitempty" json:"files,omitempty"`
}

type PatchSpec struct {
	Type    string `yaml:"type" json:"type"`
	Patch   any    `yaml:"patch,omitempty" json:"patch,omitempty"`
	Path    string `yaml:"path,omitempty" json:"path,omitempty"`
	Target  string `yaml:"target,omitempty" json:"target,omitempty"`
	Action  string `yaml:"action,omitempty" json:"action,omitempty"`
	Value   any    `yaml:"value,omitempty" json:"value,omitempty"`
	Match   string `yaml:"match,omitempty" json:"match,omitempty"`
	Enabled *bool  `yaml:"enabled,omitempty" json:"enabled,omitempty"`
}

type OutputOptions struct {
	Deterministic bool `yaml:"deterministic" json:"deterministic"`
	SortKeys      bool `yaml:"sortKeys" json:"sortKeys"`
	KeepComments  bool `yaml:"keepComments" json:"keepComments"`
}

type FetchOptions struct {
	TimeoutSeconds int  `yaml:"timeoutSeconds" json:"timeoutSeconds"`
	Retries        int  `yaml:"retries" json:"retries"`
	Concurrency    int  `yaml:"concurrency" json:"concurrency"`
	IgnoreFailed   bool `yaml:"ignoreFailed" json:"ignoreFailed"`
}

type PolicyDirectives struct {
	GamePlatformDirect []string `yaml:"gamePlatformDirect,omitempty" json:"gamePlatformDirect,omitempty"`
}

type HookConfig struct {
	JS JSHookConfig `yaml:"js,omitempty" json:"js,omitempty"`
}

type JSHookConfig struct {
	Files     []string `yaml:"files,omitempty" json:"files,omitempty"`
	TimeoutMs int      `yaml:"timeoutMs" json:"timeoutMs"`
}

func DefaultProfile() Profile {
	return Profile{
		Output: OutputOptions{
			Deterministic: true,
			SortKeys:      true,
			KeepComments:  false,
		},
		Fetch: FetchOptions{
			TimeoutSeconds: 15,
			Retries:        1,
			Concurrency:    4,
			IgnoreFailed:   false,
		},
		Hooks: HookConfig{
			JS: JSHookConfig{
				TimeoutMs: 2000,
			},
		},
	}
}

func ReadFile(path string) (Profile, error) {
	f, err := os.Open(path)
	if err != nil {
		return Profile{}, fmt.Errorf("open profile: %w", err)
	}
	defer f.Close()
	return Read(f)
}

func Read(r io.Reader) (Profile, error) {
	cfg := DefaultProfile()
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return Profile{}, fmt.Errorf("decode profile: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Profile{}, err
	}
	return cfg, nil
}

func (p Profile) Validate() error {
	if len(p.Subscriptions) == 0 && p.Template == "" {
		return errors.New("profile requires at least one subscription or a template")
	}
	for i, s := range p.Subscriptions {
		hasURL := s.URL != ""
		hasFile := s.File != ""
		hasNodesFile := s.NodesFile != ""
		count := 0
		if hasURL {
			count++
		}
		if hasFile {
			count++
		}
		if hasNodesFile {
			count++
		}
		if count != 1 {
			return fmt.Errorf("subscriptions[%d] must set exactly one of url, file or nodesFile", i)
		}
	}
	if p.Fetch.TimeoutSeconds < 0 {
		return errors.New("fetch.timeoutSeconds must be >= 0")
	}
	if p.Fetch.Retries < 0 {
		return errors.New("fetch.retries must be >= 0")
	}
	if p.Fetch.Concurrency <= 0 {
		return errors.New("fetch.concurrency must be > 0")
	}
	if p.Hooks.JS.TimeoutMs < 0 {
		return errors.New("hooks.js.timeoutMs must be >= 0")
	}
	return nil
}
