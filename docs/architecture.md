# Stage 1: Architecture and Data Flow

## Text Architecture Diagram

```text
profile.yaml
  |
  | parse + validate profile
  v
Input Loader
  |- template loader (file/url, optional)
  |- subscriptions loader (multi-source, concurrent, retry, timeout)
  v
Normalizer
  |- YAML decode to map[string]any
  |- type normalization
  v
Merge Engine
  |- append/dedupe proxies, groups, rules
  |- merge providers maps
  v
Override Engine
  |- yaml-merge patches
  |- json-patch patches
  |- strategy patch (declarative rule ops)
  v
Rule Template Engine
  |- cn-direct
  |- steam-direct-enhanced
  v
Policy Renderer
  |- gamePlatformDirect => prepend DIRECT rules
  |- fake-ip-filter enrichment
  v
JS Hook Engine
  |- mcbTransform(config, ctx)
  |- embedded runtime (no external node dependency)
  v
Validator
  |- shape checks
  |- semantic warnings
  v
Renderer
  |- stable key ordering
  |- deterministic YAML output
  v
config.yaml
```

## Directory Layout

```text
cmd/mcb/                 # CLI entrypoint
internal/cli/            # build/validate/diff command handling
internal/build/          # end-to-end pipeline orchestration
internal/profile/        # profile schema + parsing + defaults
internal/source/         # file/url fetching, retry/timeout/concurrency
internal/merge/          # subscription merge logic
internal/override/       # yaml-merge / json-patch / strategy overrides
internal/ruletemplate/   # built-in rule templates
internal/render/         # policy-level transforms (game direct, fake-ip)
internal/hook/           # embedded JS hook runtime and API bridge
internal/validate/       # config checks and warnings
internal/configfile/     # YAML decode/normalize/deterministic encode
internal/logging/        # info/warn/error logging
internal/util/           # helpers (redaction)
examples/                # runnable sample profiles
nix/                     # nix module + home-manager module
.github/workflows/       # CI
testdata/integration/    # snapshot integration fixtures
docs/                    # research, architecture, compatibility, migration
```

## Key Decisions and Trade-offs

1. Language: Go
- Pros: single binary, easy Nix packaging, fast startup, strong YAML tooling.
- Trade-off: dynamic YAML manipulation is less ergonomic than JS ecosystem.

2. Override mechanism first uses declarative patches
- YAML merge + JSON patch cover most maintainable use cases.
- Trade-off: declarative model has limits for highly dynamic transforms.

3. v1 JS hook is embedded with fixed API
- No external runtime dependency, stable interface for automation.
- Trade-off: only synchronous transform function is supported.

4. Deterministic output prioritized
- Stable map ordering and fixed indentation for reproducible builds.
- Trade-off: comments are dropped to keep renderer simple/stable.

5. Compatibility-first validator
- Lightweight schema + warning checks for practical failures.
- Trade-off: not a full strict Mihomo semantic validator.

6. Fault tolerance controllable by profile
- `ignoreFailed` lets automation keep running with partial subscription outages.
- Trade-off: operator must inspect warnings to avoid unnoticed quality drop.
