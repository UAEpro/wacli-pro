# Release

## GitHub Release Artifacts

`wacli-pro` uses GoReleaser (`.goreleaser.yaml` for macOS, `.goreleaser-linux-windows.yaml` for Linux/Windows) and the GitHub Actions workflow `.github/workflows/release.yml`.

To cut a release:

1. Update `version` in `cmd/wacli-pro/root.go` and add a `CHANGELOG.md` entry.
2. Tag and push:
   - `git tag vX.Y.Z`
   - `git push origin vX.Y.Z`
3. Wait for the GitHub Actions "Release" workflow to build and attach the artifacts.

The publish step is idempotent: if a release for the tag already exists (e.g. created manually), the workflow uploads/replaces the artifacts on it instead of failing.

To re-release an existing tag, run the "Release" workflow manually (workflow_dispatch) and pass the tag:

```bash
gh workflow run release.yml -f tag=vX.Y.Z
```

Artifacts:

- `wacli-pro-macos-universal.tar.gz` (universal arm64+amd64 binary)
- `wacli-pro-linux-amd64.tar.gz`
- `wacli-pro-linux-arm64.tar.gz`
- `wacli-pro-windows-amd64.zip`
- `checksums.txt`

The install script (`scripts/install.sh`) downloads these names from the latest release, so keep the GoReleaser `name_template` in sync with it.
