# Release Process

## Creating a Network Release

Use the [Create Release](../.github/workflows/create-release.yml) GitHub
Actions workflow to create releases. **Do not create releases manually via
the GitHub UI** — manual releases can cause `celestia version` to report an
incorrect version.

### Steps

1. Go to [Actions > Create Release](../../actions/workflows/create-release.yml)
1. Click **Run workflow**
1. Fill in the inputs:
   - **Version**: The version number without the `v` prefix (e.g., `0.30.2`)
   - **Network**: Select `mocha`, `arabica`, or `mainnet`
   - **Release branch**: The target release branch (e.g., `release/v0.30.x`)
1. Click **Run workflow**

The workflow will:

1. Create an empty commit on the release branch: `chore: release v0.30.2-mocha`
1. Tag the commit as `v0.30.2-mocha`
1. Push the commit and tag
1. Create a GitHub release with auto-generated release notes
1. Build binaries via goreleaser and attach them to the release
1. Generate and upload the OpenRPC spec

### Creating Multiple Network Releases

Run the workflow once per network. Each run creates a separate commit, so
each network tag points to a unique commit. This ensures `celestia version`
reports the correct version including the network tag.

### Why Not the GitHub UI?

When multiple tags (e.g., `v0.30.2-mocha` and `v0.30.2-arabica`) point to
the same commit, `git describe` and `git name-rev` arbitrarily pick one.
This causes `celestia version` to report the wrong version for users who
build from source with `make install`. The workflow avoids this by ensuring
each tag has its own commit.

### Fallback

If the workflow is unavailable, you can use the `CI and Release` workflow's
manual dispatch:

1. Create the release manually via the GitHub UI
1. Go to [Actions > CI and Release](../../actions/workflows/ci_release.yml)
1. Click **Run workflow** with the tag name (e.g., `v0.30.2-mocha`)

Note: this fallback does **not** create a separate commit, so the version
ambiguity issue will still apply for `make install` builds.

### Building From Source (Old Releases)

For releases where multiple tags share a commit, specify the version
explicitly:

```shell
make install VERSION=v0.30.2-mocha
```
