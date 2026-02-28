# Migration Guide: Sub-Store -> mcb

This migration keeps your workflow deterministic and Nix-friendly.

## Concept Mapping

- Sub-Store input subscriptions -> `subscriptions[]`
- Sub-Store template/base -> `template`
- Sub-Store transformations -> `overrides.patches` + `overrides.files`
- Script-based transforms -> `hooks.js.files`
- Common policy scripts -> `ruleTemplates` and `policy.gamePlatformDirect`

## Step-by-step

1. Start with your current exported Clash/Mihomo YAML as a local subscription file.
2. Create `profile.yaml` with the same sources and minimal output options.
3. Port static transformations to `yaml-merge` patches first.
4. Port structural edits to `json-patch` where exact path mutation is needed.
5. Replace common game-direct custom scripts with:
   - `ruleTemplates: [steam-direct-enhanced]`
   - `policy.gamePlatformDirect: [steam]`
6. Keep remaining custom logic in JS hooks (`mcbTransform`).
7. Run `mcb build` and compare with previous output using `mcb diff` + external diff.
8. Add `mcb validate` to CI and system timer automation.

## Example patch translation

- "Add DIRECT rule at top" -> strategy patch with `target: rules`, `action: prepend`.
- "Inject dns.fake-ip-filter entries" -> `yaml-merge` patch under `dns`.

## Caveats

- JS hooks run in embedded runtime with synchronous API only.
- Keep hook logic deterministic to preserve reproducible outputs.
