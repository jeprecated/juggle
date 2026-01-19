# Release Process

Juggle uses GoReleaser to build and publish releases.

## How Releases Work

1. **Tag a version**: Push a git tag matching `v*` (e.g., `v1.2.3`)
2. **GitHub Actions triggers**: The `.github/workflows/release.yml` workflow runs automatically
3. **Tests run first**: All tests must pass before building
4. **GoReleaser builds**: Creates binaries for:
   - Linux (amd64, arm64)
   - macOS/Darwin (amd64, arm64)
   - Windows (amd64, arm64)
5. **Draft release created**: The release is created as a **draft** (not published)
6. **Review and publish**: Manually review the draft release on GitHub and publish when ready

## Configuration

- **Workflow**: `.github/workflows/release.yml`
- **GoReleaser config**: `.goreleaser.yaml`
- **Draft mode**: `draft: true` in `.goreleaser.yaml` (line 54)

## Creating a Release

```bash
# Ensure you're on main branch and up to date
jj co main
jj git fetch

# Create and push a version tag
jj git tag v1.2.3
jj git push --tags

# Or with git:
git tag v1.2.3
git push origin v1.2.3
```

The GitHub Actions workflow will automatically:
- Run tests
- Build all platform binaries
- Create archives (tar.gz for Linux/macOS, zip for Windows)
- Generate checksums
- Create a draft release with all artifacts
- Update Homebrew tap and Scoop bucket (when published)

## Publishing the Draft

1. Go to https://github.com/ohare93/juggle/releases
2. Find the draft release
3. Review the changelog and artifacts
4. Click "Publish release"

This publishes the release and triggers:
- Homebrew tap update (ohare93/homebrew-tap)
- Scoop bucket update (ohare93/scoop)

## Why Draft Releases?

Draft releases allow manual review before publishing:
- Verify build artifacts are correct
- Review auto-generated changelog
- Add release notes or highlights
- Check for any build warnings or issues

To auto-publish releases immediately (not recommended), change `draft: true` to `draft: false` in `.goreleaser.yaml`.
