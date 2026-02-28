# Release Checklist (v0.1.0)

## 1) Version and metadata

- [ ] Confirm tag version: `v0.1.0`
- [ ] Confirm `nix/package.nix` version matches release
- [ ] Confirm release notes file exists: `docs/releases/v0.1.0.md`

## 2) Build and quality gates

- [ ] `nix shell nixpkgs#go -c go test ./...`
- [ ] `nix shell nixpkgs#go -c go vet ./...`
- [ ] `nix build .#mihomo-config-builder --no-link`
- [ ] `nix flake check`

## 3) CLI smoke tests

- [ ] `go build -o bin/mcb ./cmd/mcb`
- [ ] `./bin/mcb build -c examples/basic/profile.yaml -o /tmp/mcb-basic.yaml`
- [ ] `./bin/mcb validate -f /tmp/mcb-basic.yaml`
- [ ] `./bin/mcb build -c examples/steam-direct/profile.yaml -o /tmp/mcb-steam.yaml`
- [ ] `./bin/mcb validate -f /tmp/mcb-steam.yaml`
- [ ] `./bin/mcb diff -c examples/basic/profile.yaml --against /tmp/mcb-basic.yaml`

## 4) NixOS and module checks

- [ ] NixOS module options documented (`enable`, `package`, `profileFile`, `outputFile`, `schedule`)
- [ ] systemd oneshot/timer behavior verified
- [ ] mihomo restart hook (`ExecStartPost`) validated in docs

## 5) Release artifacts

- [ ] Git tag created: `git tag v0.1.0`
- [ ] Push tag to remote: `git push origin v0.1.0`
- [ ] GitHub Release created using notes from `docs/releases/v0.1.0.md`

## 6) Post-release

- [ ] Verify fresh install path from README in clean environment
- [ ] Validate at least one user profile migration using `docs/migration-sub-store.md`
