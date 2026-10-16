# Releasing

A release is a tag v<major>.<minor>.<patch> on a commit of main. The release lane in release.yml publishes it only when every gate below holds; none of them is optional.

## Before the tag

1. The commit is on main and its CI run on push succeeded: make ci on Linux, macOS and Windows, the acceptance job, release-config and the plugin validation.
2. The dated live record of the candidate is in docs/live/claude-darwin.json, as agent-contracts.md describes: one macOS session of the pinned host with the allowed control, the denied write, the host timeout and the missing binary, naming the digest of CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -buildvcs=false ./cmd/golaw run with Go 1.27.0, the go-version of release.yml. The gate rebuilds the candidate with that toolchain, so a record made with another Go release names another digest, and any toolchain bump, in release.yml or in go.mod, needs a new live record. A commit that only adds the record keeps that digest, so the record is committed first and the tag goes on that commit.
3. CHANGELOG.md dates every rule change of the release with its reason.
4. The README and docs/setup.md install the new tag, and agents/SKILL.md names the rules page at the same tag, with its plugin copy; cicheck holds the three to one version.

## What the lane runs

1. The gate requires the tag to be reachable from main with a successful CI push run for the same commit, then runs go run ./tools/acceptance -release with the pinned host. Every required row must pass and SKIP never counts as PASS, so a missing or stale live record stops the release there.
2. The publication job checks out that commit, builds the immutable candidate and scans the same commit with it. The first release has no earlier verified binary, so the candidate built from the tagged commit is its own scanner; later releases keep the step.
3. goreleaser prepares a signed draft: six binaries, checksums.txt, its Sigstore bundle and one SBOM per binary.
4. cosign verifies the bundle against the exact identity of release.yml at the tag, and every binary receives a provenance attestation.
5. Only then is the draft published.

## After the release

Verify a downloaded binary as setup.md describes before running it. Keep the previous validated combination in the compatibility history of agent-contracts.md, and record a new live session for the next candidate.
