# Research Notes

This file summarizes implementation ideas from upstream projects and documents license boundaries.

## Scope and Principle

- We reference public behavior, docs, and architecture ideas only.
- We do not copy source code from AGPL/GPL projects into this repository.
- We reimplement features from scratch with independent naming, structure, and tests.

## Project Survey

### 1) Mihomo official docs (authoritative config behavior)

References:
- https://wiki.metacubex.one/en/config/
- https://wiki.metacubex.one/en/config/general/
- https://wiki.metacubex.one/en/config/dns
- https://wiki.metacubex.one/en/config/rules
- https://wiki.metacubex.one/en/config/proxy-groups/
- https://wiki.metacubex.one/en/config/proxy-providers
- https://github.com/MetaCubeX/mihomo
- https://raw.githubusercontent.com/MetaCubeX/mihomo/Alpha/docs/config.yaml

Key takeaways:
- Top-level fields and behavior are YAML-driven, with critical sections: `proxies`, `proxy-groups`, `rules`, `dns`, provider sections.
- Rule order is priority-sensitive (top to bottom), so deterministic generation and explicit prepend/append semantics are necessary.
- `dns.enhanced-mode: fake-ip` requires careful `fake-ip-filter` handling to reduce compatibility issues for special domains and platform services.
- Compatibility for GUI clients is usually strongest when generated YAML stays close to mainstream Mihomo field usage and avoids client-private extensions.

How we apply:
- `mcb validate` checks required top-level structure and common semantic risks.
- Deterministic rendering keeps stable key order for reproducible output and clean git diff.
- Policy-level helper adds game-domain `DIRECT` rules and fake-ip filter entries.

### 2) Sub-Store (subscription processing + override concepts)

References:
- https://github.com/sub-store-org/Sub-Store
- https://raw.githubusercontent.com/sub-store-org/Sub-Store/master/LICENSE

Key takeaways:
- Subscription pipelines often combine: source fetch, parse/normalize, transform/override, and export.
- Practical override mechanisms include declarative patching and scriptable transformations.
- Multi-source aggregation and per-source fault tolerance are key for real-world operations.

How we apply:
- MVP uses declarative overrides only: YAML merge + JSON patch (+ a small strategy patch helper).
- Optional failure tolerance via `fetch.ignoreFailed` and clear error messages for network failures.
- We define stable, explicit profile format (`profile.yaml`) for non-interactive automation.

### 3) Clash Verge Rev and Mihomo Party (consumer compatibility targets)

References:
- https://github.com/clash-verge-rev/clash-verge-rev
- https://raw.githubusercontent.com/clash-verge-rev/clash-verge-rev/dev/LICENSE
- https://github.com/mihomo-party-org/clash-party
- https://raw.githubusercontent.com/mihomo-party-org/clash-party/smart_core/LICENSE

Key takeaways:
- Both clients are GUI wrappers around Mihomo core behavior and consume Mihomo-style YAML configs.
- Operational stability depends on keeping generated configs in the common supported subset.
- Avoiding ambiguous or exotic fields improves cross-client portability.

How we apply:
- Validate and warn around common pitfalls (especially fake-ip related domain behavior).
- Ensure generated YAML is standard and deterministic.
- Keep output compatible-first: no UI-specific private schema in MVP.

## License Risk and Mitigation

Observed upstream licenses:
- Sub-Store: AGPL-3.0
- Clash Verge Rev: GPL-3.0
- Mihomo Party (clash-party): GPL-3.0

Risk:
- Direct code reuse, adaptation, or close derivative translation from AGPL/GPL repositories can trigger copyleft obligations.

Mitigation used in this project:
- No code copy from upstream repositories.
- Behavior-level and documentation-level reference only.
- Fresh implementation with independent structure and naming.
- Project tests are based on our own fixtures and generated snapshots.
- This repository documents references and explicitly marks them as design inspiration only.

## Boundaries

Allowed:
- Field definitions and runtime behavior inferred from official docs.
- Public interface behavior compatibility.
- High-level architecture patterns (fetch -> merge -> override -> validate -> render).

Not allowed in this repository:
- Copy-paste from AGPL/GPL implementation files.
- Porting upstream functions line-by-line.
- Retaining upstream private API names tied to implementation details.
