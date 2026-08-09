# gotunnel Release Plan — Path to v1.0

**Date:** 2025-04-25  
**Current Version:** v0.2.0  
**Target:** v1.0.0  
**Repo:** github.com/v1truv1us/gotunnel

---

## Current State Assessment

### Project Overview
gotunnel is a Go-based CLI tool that creates secure local HTTP/HTTPS tunnels via `.local` domains for development. It features a built-in reverse proxy, mDNS discovery, DNS server, self-signed certificate management, and OpenTelemetry/Sentry observability. ~5,600 lines of Go across 10 internal packages.

### Architecture
- **cmd/gotunnel** — CLI entrypoint (urfave/cli v2)
- **internal/tunnel** — Tunnel manager, reverse proxy, hosts file management
- **internal/proxy** — Proxy manager (builtin/nginx/caddy modes)
- **internal/cert** — Self-signed TLS certificate generation
- **internal/dnsserver** — DNS server for `.local` domain resolution
- **internal/mdns** — mDNS service discovery
- **internal/observability** — OpenTelemetry + Sentry integration
- **internal/logging** — Structured logging (slog-based)
- **internal/privilege** — Root/admin privilege detection
- **internal/state** — Tunnel state management

### Current Tags
`v0.1.0-beta` → `v0.1.1-beta` → `v0.1.2-beta` → `v0.1.3-beta` → `v0.2.0`

### Open Issues
0 open issues on the v1truv1us/gotunnel repo.

---

## What Works ✅

1. **Core tunneling** — Start/stop/list tunnels, reverse proxy to local app, `.local` domain resolution
2. **TLS/HTTPS** — Self-signed cert generation, TLS 1.2+ with modern cipher suites
3. **Built-in proxy** — Reverse proxy mode without external dependencies
4. **mDNS discovery** — Network-wide `.local` domain access via mDNS
5. **DNS server** — Custom DNS server for domain resolution
6. **Observability** — Full OpenTelemetry traces + metrics, Sentry error tracking, Prometheus endpoint
7. **Structured logging** — slog-based with configurable levels/formats
8. **Tests pass** — All 10 packages pass (`go test ./...` ✅)
9. **Builds clean** — `go build ./cmd/gotunnel` succeeds on arm64 macOS
10. **GoReleaser config** — Proper cross-platform build config (linux/darwin/windows × amd64/arm64)
11. **Homebrew tap exists** — `v1truv1us/homebrew-tap` repo exists and is active
12. **CI/CD pipeline** — Comprehensive GitHub Actions (test, lint, security, build, Docker, release)
13. **Release workflow** — Tag-triggered GoReleaser with Homebrew tap auto-publish
14. **Docker** — Dockerfile + docker-compose.yml with monitoring stack (Prometheus + Grafana)
15. **Configuration** — Example YAML config with full observability options
16. **Packaging scaffolding** — Directories for chocolatey, debian, homebrew, scoop, winget
17. **Docs site** — Astro-based docs site exists in `docs-site/`
18. **Security docs** — SECURITY.md, CONTRIBUTING.md, code of conduct templates
19. **Code signing** — macOS code signing workflow in CI (with certificate import)

---

## What's Broken / Missing 🔴

### Critical — Org Reference Inconsistencies
The project has been moved between GitHub orgs/names and references are **scattered across 3 identities**:

| Reference | Used In |
|-----------|---------|
| `v1truv1us/gotunnel` | go.mod, GoReleaser ldflags, CI YAML, winget manifests, install.sh, AUR PKGBUILD, Docker references, APT example in README |
| `v1truv1us/gotunnel` | GoReleaser `brews.repository.owner`, release workflow trigger |
| `v1truv1us/gotunnel` | `gotunnel.rb` (root-level formula), README badge URLs, Homebrew tap references, Scoop bucket reference |

**This is the #1 blocker.** Users can't install from any single consistent source.

### Specific Broken Items

1. **`gotunnel.rb` (root-level)** — Points to `v1truv1us/gotunnel` with placeholder SHA256s and version `0.1.0-beta`. Unusable.
2. **README badges** — Point to `v1truv1us/gotunnel` (old org). Shields will 404 if repo doesn't exist there.
3. **README install instructions** — Mix of `v1truv1us` (Homebrew/Scoop), `v1truv1us` (APT, Docker, Go install, scripts). Confusing.
4. **`packaging/homebrew/gotunnel.rb`** — Likely has same org issues.
5. **Sentry DSN hardcoded in main.go** — `"https://2df8619717cb8316ef83612d2ec29b95@sentry.fergify.work/11"` is a real DSN baked into source. Should be env-only default.
6. **TLS config uses deprecated `PreferServerCipherSuites`** — Will generate warnings on Go 1.24+.
7. **`resolveHostname` function** — Dead code; both branches do identical thing.
8. **CI pipeline is overly complex** — 400+ line CI file does build + release + publish to 4 package managers. Should be split. Many `continue-on-error: true` masking real failures.
9. **No config file loading** — `configs/gotunnel.example.yaml` exists but CLI doesn't read it. Config parsing code is absent.
10. **Docs site is minimal** — Astro scaffold with no real content.
11. **No AUR package published** — CI generates PKGBUILD but doesn't publish to AUR.
12. **No Chocolatey package published** — CI generates nuspec but doesn't push.
13. **No Scoop bucket exists** — `v1truv1us/scoop-bucket` referenced in README but likely doesn't exist.
14. **Winget manifests use `v1truv1us` org** — Wrong org.
15. **CHANGELOG version dates are inconsistent** — `0.1.0-beta` dated 2024-08-13, `0.0.1-alpha` dated 2024-11-01 (later than beta?). Last updated note says 2025-08-23 (future).
16. **No `--version` command** — Version embedded in `--version` flag via urfave/cli, but `gotunnel.rb` test checks `--version` which may not work without proper ldflags.
17. **`copy-secrets.md` in repo root** — Internal doc committed to public repo.

---

## Feature Scope for v1.0

The goal for v1.0 is **production-quality distribution of existing functionality** — not new features. The core tunneling works well. The focus is:

1. **Fix org references** — Single canonical identity everywhere
2. **Working distribution** — Homebrew + Go install + Docker as primary channels
3. **Clean CI/CD** — Simplified, reliable pipeline
4. **Config file support** — Load from `~/.config/gotunnel/config.yaml`
5. **Remove hardcoded secrets** — Sentry DSN out of source
6. **Dead code cleanup** — Remove unused functions
7. **Docs site with real content** — Installation, usage, configuration guides
8. **Consistent versioning** — Clean CHANGELOG, proper semver

### Deferred to v1.1+
- AUR/Chocolatey/Winget/Scoop publishing (scaffolding exists, automate later)
- Web UI for tunnel management
- Load balancing / connection pooling
- API for programmatic control
- Configuration profiles

---

## Ordered Work Items

| # | Task | Est. Time | Priority |
|---|------|-----------|----------|
| 1 | **Unify org references to `v1truv1us/gotunnel`** — go.mod, all README URLs, badges, install instructions, GoReleaser ldflags, CI, packaging, scripts | 2h | 🔴 P0 |
| 2 | **Remove hardcoded Sentry DSN from main.go** — Make env-var only, remove default | 30m | 🔴 P0 |
| 3 | **Fix TLS deprecation** — Remove `PreferServerCipherSuites`, update cipher suites | 30m | 🟡 P1 |
| 4 | **Delete dead code** — `resolveHostname`, root-level `gotunnel.rb` | 30m | 🟡 P1 |
| 5 | **Delete `copy-secrets.md`** from repo root | 5m | 🔴 P0 |
| 6 | **Implement config file loading** — Read `~/.config/gotunnel/config.yaml` on startup, merge with CLI flags | 3h | 🟡 P1 |
| 7 | **Update CHANGELOG** — Fix dates, add v0.2.0 entry, add v1.0.0 planned entry | 30m | 🟡 P1 |
| 8 | **Simplify CI pipeline** — Split ci.yml into ci.yml (test+lint) and release.yml (build+distribute). Remove `continue-on-error` on test/lint jobs | 2h | 🟡 P1 |
| 9 | **Verify GoReleaser Homebrew tap publish** — Test release workflow dry-run, ensure `v1truv1us/homebrew-tap` gets updated Formula | 1h | 🔴 P0 |
| 10 | **Fix README install instructions** — Primary: Homebrew via `v1truv1us/homebrew-tap`, Go install, Docker. Remove references to non-existent Scoop bucket | 1h | 🔴 P0 |
| 11 | **Update docs-site with real content** — Quick start, CLI reference, config guide, architecture overview, troubleshooting | 4h | 🟢 P2 |
| 12 | **Add `--config` flag** — Explicit config file path override | 30m | 🟡 P1 |
| 13 | **Clean up `packaging/` directories** — Update winget manifests to `v1truv1us` org, remove placeholder SHA256s, or remove unpublishable scaffolding | 1h | 🟡 P1 |
| 14 | **Add integration test** — Start tunnel → HTTP request → verify response flows through | 2h | 🟢 P2 |
| 15 | **Write CONTRIBUTING.md update** — Reflect actual dev setup, remove boilerplate | 1h | 🟢 P2 |

**Total estimated time: ~19 hours**

---

## Release Checklist

### Pre-Release
- [ ] All org references unified to `v1truv1us/gotunnel`
- [ ] `go.mod` module path is `github.com/v1truv1us/gotunnel`
- [ ] Hardcoded Sentry DSN removed from source
- [ ] `copy-secrets.md` removed from repo
- [ ] All tests pass (`go test ./...`)
- [ ] `go vet ./...` clean
- [ ] `golangci-lint run` clean (or documented exceptions)
- [ ] TLS config updated (no deprecated fields)
- [ ] Dead code removed
- [ ] Config file loading works
- [ ] CHANGELOG updated for v1.0.0
- [ ] README accurate and consistent
- [ ] GoReleaser dry-run succeeds (`goreleaser release --snapshot --clean`)
- [ ] Homebrew formula testable locally
- [ ] Docker build succeeds

### Release
- [ ] Create `v1.0.0` tag on main
- [ ] Push tag → triggers release workflow
- [ ] Verify GitHub Release created with all binaries + checksums
- [ ] Verify Docker image published to `ghcr.io/v1truv1us/gotunnel`
- [ ] Verify Homebrew tap updated (`v1truv1us/homebrew-tap`)
- [ ] Test `brew install gotunnel` works on macOS (arm64 + amd64)
- [ ] Test `go install github.com/v1truv1us/gotunnel/cmd/gotunnel@v1.0.0` works
- [ ] Smoke test: `gotunnel --proxy=builtin --no-privilege-check start --port 3000 --domain testapp --https=false`

### Post-Release
- [ ] Update docs-site with v1.0.0 content
- [ ] Publish docs-site
- [ ] Verify `gotunnel.dev` resolves and shows docs
- [ ] Announce on Twitter/X, Discord, relevant subreddits
- [ ] Submit to Awesome Go list
- [ ] Write blog post / dev.to article
- [ ] Close any v1.0 milestone issues

---

## Distribution & Marketing Plan

### Primary Distribution Channels (v1.0)

| Channel | Status | Action Needed |
|---------|--------|---------------|
| **Homebrew** (`v1truv1us/homebrew-tap`) | Tap exists, GoReleaser configured | Verify auto-publish works on release |
| **Go Install** | Works now (after go.mod fix) | Update module path |
| **Docker** (`ghcr.io/v1truv1us/gotunnel`) | CI configured | Verify image publishes |
| **GitHub Releases** | Binaries for 6 platforms | Verify release workflow |

### Secondary Channels (v1.1+)

| Channel | Status | Notes |
|---------|--------|-------|
| **Scoop** | Referenced but no bucket | Create `v1truv1s/scoop-bucket` or remove reference |
| **AUR** | PKGBUILD generated in CI | Need AUR account + SSH key for publish |
| **Chocolatey** | nuspec exists | Need Chocolatey account + API key |
| **Winget** | Manifests exist | Submit PR to microsoft/winget-pkgs |
| **DEB/RPM** | GoReleaser nfpm not configured | Add nfpm section to GoReleaser |

### Marketing

1. **GitHub** — Star the repo, add topics (`tunnel`, `local-development`, `devtools`, `go`, `https`), write good description
2. **README badges** — Ensure all badges resolve correctly post-org-fix
3. **Documentation site** — `gotunnel.dev` as canonical docs URL
4. **Community posts** — Dev.to, r/golang, r/selfhosted, Hacker News Show HN
5. **Awesome lists** — Submit to awesome-go, awesome-selfhosted
6. **Social** — Tweet/Discord announcement with GIF demo

### Competitive Positioning
- vs **ngrok**: gotunnel is self-hosted, no account needed, no external relay
- vs **localtunnel**: Go binary (no Node.js), built-in TLS cert management
- vs **nip.io/sslip.io**: Works offline, no internet required, full HTTPS support
- Key differentiator: Zero-config local `.local` domain tunnels with no external dependencies

---

## Is v0.2.0 Ready for v1.0?

**Core functionality: Yes. Distribution and polish: No.**

The tunneling engine, proxy, cert management, DNS, and observability are solid. Tests pass. The main gaps are:

1. **Org identity chaos** — Must be fixed before any public release
2. **Config file support** — Documented in README but not implemented
3. **Hardcoded secrets** — Security concern
4. **Distribution verification** — Haven't confirmed Homebrew tap actually works end-to-end

With ~19 hours of focused work on the items above, v1.0 is achievable. The codebase is clean, well-structured, and functionally complete for its stated purpose.

---

*Generated: 2025-04-25 by OpenClaw audit agent*
