# mihomo-config-builder

`mihomo-config-builder` (`mcb`) is a deterministic config generator for Mihomo-compatible clients.

Input:
- one or more subscriptions (URL or local file)
- or one or more local `nodes.txt` sources (`nodesFile`)
- override patches (YAML merge / JSON patch)
- optional template

Output:
- generated `config.yaml`

Goals:
- reproducible output
- automation-first (NixOS/systemd)
- compatibility-first (mihomo / mihomo-party / clash-verge-rev)

## 5-minute Quick Start

### 1) Build

```bash
go build -o bin/mcb ./cmd/mcb
```

### 2) Generate config from example

```bash
./bin/mcb build -c examples/basic/profile.yaml -o config.yaml
```

### 3) Validate output

```bash
./bin/mcb validate -f config.yaml
```

### 4) Compare with previous config

```bash
./bin/mcb diff -c examples/basic/profile.yaml --against old.yaml
```

## CLI

- `mcb build -c profile.yaml -o config.yaml`
- `mcb validate -f config.yaml`
- `mcb diff -c profile.yaml --against old.yaml`

## Profile Format

```yaml
subscriptions:
  - url: https://example.com/sub?token=***
  - file: ./subscription.yaml
  - nodesFile: ./nodes.txt

template: ./base-template.yaml

overrides:
  patches:
    - type: yaml-merge
      patch:
        mode: rule
    - type: json-patch
      patch:
        - op: replace
          path: /dns/enhanced-mode
          value: fake-ip
    - type: strategy
      target: rules
      action: prepend
      value: DOMAIN-SUFFIX,steamcontent.com,DIRECT
  files:
    - ./overrides/common.yaml

ruleTemplates:
  - cn-direct
  - steam-direct-enhanced

hooks:
  js:
    files:
      - ./hooks/custom.js
    timeoutMs: 2000

output:
  deterministic: true
  sortKeys: true
  keepComments: false

fetch:
  timeoutSeconds: 15
  retries: 1
  concurrency: 4
  ignoreFailed: false

policy:
  gamePlatformDirect:
    - steam
    - epic
```

## Rule Template Library (v1)

Built-in templates:
- `cn-direct`
- `steam-direct-enhanced`

## JS Hook API (v1)

Each JS file must define:

```js
function mcbTransform(config, ctx) {
  // mutate and return config object
  return config;
}
```

`ctx` fields:
- `profilePath` (string)
- `sourceCount` (number)
- `nowUnix` (number)

The hook runtime is embedded and does not depend on external Node/Bun/Deno.

## Examples

- `examples/basic/profile.yaml`
- `examples/steam-direct/profile.yaml`
- `examples/nodes/profile.yaml`

`nodes.txt` (one node URI per line, `#` comment supported) currently supports:
- `ss://`
- `vmess://`
- `trojan://`
- `vless://`
- `hysteria2://` / `hy2://`
- `socks5://` / `socks://` / `sock5://`
- `http://` / `https://`

## Reliability and Security

- timeout/retry/concurrency configurable in profile
- optional degraded mode with `fetch.ignoreFailed`
- URL tokens are redacted in network error messages
- generated files and secret-like files are ignored via `.gitignore`

## NixOS Integration

Use flake package + module:

```nix
{
  inputs.mcb.url = "github:yourname/mihomo-config-builder";

  outputs = { self, nixpkgs, mcb, ... }: {
    nixosConfigurations.host = nixpkgs.lib.nixosSystem {
      system = "x86_64-linux";
      modules = [
        mcb.nixosModules.default
        ({ pkgs, ... }: {
          services.mihomo-config-builder = {
            enable = true;
            package = mcb.packages.x86_64-linux.default;
            profileFile = /etc/mcb/profile.yaml;
            outputFile = "/var/lib/mihomo/config.yaml";
            schedule = "*:0/30";
            mihomoServiceName = "mihomo.service";
          };
        })
      ];
    };
  };
}
```

See full examples:
- `examples/nixos/configuration.nix`
- `examples/home-manager/home.nix`

## Common Commands

- `make build`
- `make test`
- `make lint`
- `make ci`

(Equivalent commands are also provided in `justfile`.)

## CI

GitHub Actions workflow runs:
- lint (`go vet`)
- tests (`go test ./...`)

## Compatibility Notes

See `docs/compatibility.md`.

## Research and License Boundaries

See `docs/research.md` for upstream references and license-risk mitigation.

## Release Notes

- Current release notes draft: `docs/releases/v0.2.0.md`
- Release checklist: `docs/releases/checklist.md`
