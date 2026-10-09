# mihomo-config-builder

`mihomo-config-builder` (`mcb`) is a deterministic config generator for Mihomo-compatible clients.

Input:
- one or more subscriptions (URL or local file)
- subscription URLs may return Mihomo YAML, node URI lines, or base64-encoded node URI lines
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
GOPROXY=off GOSUMDB=off CGO_ENABLED=0 go build -mod=vendor -o bin/mcb ./cmd/mcb
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
```

`subscriptions[].url` accepts:
- Mihomo/Clash YAML subscriptions
- plain node URI line subscriptions
- base64-encoded node URI line subscriptions

```yaml

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
  # optional: override remote subscription UA
  userAgent: Clash-verge/v2.0.0
  # optional: fetch through a specific proxy, e.g. local clash/mihomo mixed port
  # proxyURL: http://127.0.0.1:7890

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

`nodes.txt` supports mixed input:
- node URI lines (one per line, `#` full-line comment supported)
- subscription URL lines (`http://` / `https://`, auto fetch + parse)
- subscription URL can set node-name suffix via URL fragment, e.g. `https://example.com/sub?token=***#HK` (nodes become `原名 🇭🇰HK`)

Node URI schemes currently supported:
- `ss://`
- `vmess://`
- `trojan://`
- `vless://`
- `hysteria2://` / `hy2://`
- `anytls://`
- `socks5://` / `socks://` / `sock5://`
- `http://` / `https://`

## Reliability and Security

- timeout/retry/concurrency configurable in profile
- remote fetch can override `fetch.userAgent` and `fetch.proxyURL` for subscription providers that gate by client identity or network path
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

- Current release notes: `docs/releases/v0.3.2.md`
- Release checklist: `docs/releases/checklist.md`

## Offline reliability and node presentation (0.3.0)

Public Go dependencies and their licenses are included in `vendor/`. Builds use
`GOPROXY=off GOSUMDB=off -mod=vendor`; the executable has no Go/Node runtime or
package downloads. Nix still needs the compiler and other build tools on a new
machine. Update vendored modules deliberately with `go mod tidy && go mod vendor`.

```yaml
fetch:
  cacheDir: ./private-source-cache
  offline: false
naming:
  enable: true
  metadataFile: ./metadata.json
  includeNetwork: false
```

Paths above are relative to the profile. Successful remote responses are parsed
before they atomically replace the private cache (directory 0700, files 0600).
HTTP failures and invalid responses use the last accepted response. `--offline`
never fetches a remote source; a cache miss is an error. Cached subscriptions
contain credentials: keep the cache out of Git and the Nix store. Caches have no
expiry to support recovery; an offline build can contain outdated nodes. Remove
the relevant cache when revoking a subscription. Config output is atomic and 0600.
The caller may grant a dedicated daemon group read access afterwards.

Names use flag, country and city, with a two-digit number when needed.
Peers are ordered by their internal identity, so source reordering preserves numbers.
Adding or removing peers can renumber a location; integrations should restore
selections by endpoint identity. Protocol and guessed route labels are omitted. Both manual links and subscription
nodes pass through the same naming step. Different nodes with the same original
label survive merging. Group, rule, dialer and DNS references follow renames;
overrides that target an original node name must account for naming before merging.
JS hooks receive the normalized names and must preserve references if changing them.

Builds read only local metadata. Enrichment is an explicit background operation:

```bash
mcb metadata refresh -f config.yaml --cache metadata.json --proxy-url http://127.0.0.1:7892
mcb build -c profile.yaml -o config.yaml --offline --metadata-output node-metadata.json
```

The default lookup is ipwho.is, with seven-day freshness and bounded concurrency
and timeouts. Private IPs are skipped; failed lookups retain previous metadata.
Missing metadata gives a `🌐 未定位` name without blocking builds. The location and
network identify the **server IP**, which can be a relay entry rather than the
proxy exit. ASN does not establish CN2, 9929 or other route characteristics.

`node-metadata.json` maps final node names to display metadata only (country,
city, server IP, ASN, organization, network). It contains no passwords or URLs.
A dashboard can read this optional sidecar and render `network` as a separate
badge beside the protocol, keeping node names compact. Original names containing
`优化` also export `optimized: true` for a separate optimization badge, even when
IP metadata is unavailable. This is a source-provided label, not a measured route
quality claim. A private `x-mcb-optimized` configuration extension preserves it
across renaming and later exports without changing endpoint identity. `mcb metadata export`
exports a sidecar for an existing config without fetching or renaming anything.
