# Release Management

This document describes how Zeus releases are built, versioned, and published.

---

## Pipeline Overview

```
1. Run bump-version workflow (GitHub Actions UI)
        ↓
2. New git tag is created and pushed (e.g. v0.0.8-alpha)
        ↓
3. release.yml triggers on the tag push
        ↓
4. macOS runner builds Boehm GC → Zig runtime → Zeus binary (version baked in)
        ↓
5. Tarball + SHA256 checksum uploaded as artifacts
        ↓
6. GitHub Release created with the tarball
        ↓
7. homebrew-zeus tap repo formula updated automatically
        ↓
8. Users install via: brew tap ameerthehacker/zeus && brew install zeus
```

---

## Cutting a Release

### Normal flow (automated)

1. Go to **Actions → Bump Version** in the GitHub UI
2. Click **Run workflow**
3. Choose:
   - **bump**: `patch` / `minor` / `major` (which part of `X.Y.Z` to increment)
   - **pre_release** *(optional)*: a suffix like `alpha` or `beta`; leave blank for a stable release

**Examples:**

| Current tag | bump | pre_release | Result |
|-------------|------|-------------|--------|
| `v0.0.7-alpha` | `patch` | `alpha` | `v0.0.8-alpha` |
| `v0.0.7-alpha` | `minor` | *(empty)* | `v0.1.0` |
| `v1.2.3` | `major` | `beta` | `v2.0.0-beta` |

The bump workflow creates and pushes the tag. The release workflow fires automatically — no further action required.

### Manual fallback

If you need to skip the bump helper (e.g. a hotfix with a specific version):

```bash
git tag v0.0.8-alpha
git push origin v0.0.8-alpha
```

The release workflow triggers on any `v*` tag push.

---

## How Version Gets Into the Binary

The release workflow injects the version string at compile time via Go's `-ldflags`:

```bash
go build -tags llvm19 \
  -ldflags "-X 'github.com/ameerthehacker/zeus/cmd.Version=${VERSION}'" \
  -o bin/zeus zeus.go
```

The variable `cmd.Version` in `cmd/version.go` defaults to `"dev"` for local builds:

```go
var Version = "dev"
```

So:
- `go build` locally → `zeus --version` prints `zeus dev`
- CI release build → `zeus --version` prints `zeus 0.0.8-alpha`

---

## Homebrew Tap (`homebrew-zeus`)

The formula lives in a **separate repository** (`ameerthehacker/homebrew-zeus`), not in the main zeus repo. This means `brew tap ameerthehacker/zeus` only clones that small repo — not the entire zeus codebase.

The release workflow automatically updates `Formula/zeus.rb` in that repo with the new version and SHA256 checksum after every successful release.

### One-time setup (already done)

1. Created repo `ameerthehacker/homebrew-zeus` with `Formula/zeus.rb`
2. Created a fine-grained PAT with **Contents: Read and write** on `homebrew-zeus`
3. Stored the PAT as secret `HOMEBREW_TAP_TOKEN` in the main zeus repo settings

### User installation

```bash
brew tap ameerthehacker/zeus
brew install zeus
```

---

## Build Steps in CI

The `build` job in `release.yml` runs on `macos-15` (ARM64) and performs these steps in order:

| Step | What it does |
|------|-------------|
| Install LLVM@19 | Required for the Go CGO build |
| Install Zig 0.13.0 | Builds the Zeus runtime |
| **Build Boehm GC** | `cmake -B build && make -C build` — downloads bdwgc v8.2.12 + libatomic_ops via FetchContent into `third_party/bdwgc/` |
| Build Zig runtime | `zig build -Doptimize=ReleaseSmall` inside `runtime/` |
| Build Zeus binary | `go build` with CGO flags pointing to LLVM and version ldflags |
| Package | Creates `zeus-{VERSION}-darwin-{ARCH}.tar.gz` + `.sha256` |

The Boehm GC step mirrors what `make always` does locally and must run before the Zig runtime build because the runtime links against `libgc`.

---

## Artifacts

Each release produces:

```
zeus-{VERSION}-darwin-arm64.tar.gz
  └── zeus-{VERSION}-darwin-arm64/
        ├── bin/zeus          ← compiler binary (version embedded)
        ├── runtime/zig-out/  ← compiled Zeus runtime
        └── lib/std/          ← Zeus standard library
```

Intel Macs use the ARM64 binary via Rosetta 2.

---

## Prerelease Detection

The release workflow automatically marks the GitHub Release as a **pre-release** if the version string contains a `-` (e.g. `0.0.8-alpha`). Stable versions (`1.0.0`) are marked as full releases.

---

## Troubleshooting

**`HOMEBREW_TAP_TOKEN` secret expired or missing**
- Create a new fine-grained PAT with `Contents: Read and write` on `ameerthehacker/homebrew-zeus`
- Update the secret in zeus repo → Settings → Secrets and variables → Actions

**cmake step fails**
- Check that `CMakeLists.txt` FetchContent URLs are reachable from the runner
- The step downloads `bdwgc v8.2.12` and `libatomic_ops v7.8.2` from GitHub

**Tag already exists**
- The bump workflow will fail if the computed tag already exists
- Either delete the tag (`git push origin :v0.0.8-alpha`) or push a manual tag with a different version

**Release workflow doesn't fire after tagging**
- Verify the tag matches the `v*` glob in `release.yml`
- Tags pushed by `GITHUB_TOKEN` do not trigger other workflows by default — the bump workflow uses `contents: write` permission which does trigger downstream workflows (this is the standard GitHub behavior for tags)
