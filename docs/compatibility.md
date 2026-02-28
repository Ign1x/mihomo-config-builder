# Compatibility Notes

Target clients:
- Mihomo core
- Mihomo Party
- Clash Verge Rev

## MVP + v1 Compatibility Scope

The generator emits standard Mihomo YAML with emphasis on these sections:
- `proxies`
- `proxy-groups`
- `rules`
- `dns`
- `rule-providers` (merge/update paths)

This subset is intentionally conservative and broadly supported by mainstream Mihomo GUI clients.

## Design choices for cross-client stability

- Deterministic output for reproducibility and debugging.
- Rule ordering preserved and explicit for policy predictability.
- Fake-IP warning hints to avoid silent breakage with gaming/platform domains.
- Minimal assumptions about client-specific UI metadata.

## v1 additions

- Embedded JS hook runtime with fixed API (`mcbTransform(config, ctx)`), no external runtime dependency.
- Built-in rule templates:
  - `cn-direct`
  - `steam-direct-enhanced`

## Known limits

- `mcb diff` remains intentionally simplified (change detection + external diff hint).
- Validation is schema/semantic-lite; not a full Mihomo parser.

## Validation strategy

`mcb validate` catches:
- Missing required top-level sections.
- Invalid shape for proxies/proxy-groups/rules.

`mcb build` warning examples:
- Missing recommended fake-ip filter entries for common conflict domains.
