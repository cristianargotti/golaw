set -euo pipefail
test "$GITHUB_REF_TYPE" = tag
case "$GITHUB_REF" in
  refs/tags/v*) ;;
  *) exit 1 ;;
esac
tag_sha=$(git rev-parse 'HEAD^{commit}')
test "$(git rev-parse "${GITHUB_REF}^{commit}")" = "$tag_sha"
git merge-base --is-ancestor "$tag_sha" refs/remotes/origin/main
gh api --method GET \
  "repos/${GITHUB_REPOSITORY}/actions/workflows/ci.yml/runs" \
  -f head_sha="$tag_sha" -f branch=main -f event=push \
  -f per_page=100 > "$RUNNER_TEMP/ci-runs.json"
jq -e --arg sha "$tag_sha" \
  -f tools/cicheck/release-query.jq "$RUNNER_TEMP/ci-runs.json"
printf 'sha=%s\n' "$tag_sha" >> "$GITHUB_OUTPUT"
