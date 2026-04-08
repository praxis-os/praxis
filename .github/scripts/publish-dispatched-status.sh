#!/usr/bin/env bash

set -euo pipefail

: "${GH_TOKEN:?GH_TOKEN is required}"
: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"
: "${GITHUB_RUN_ID:?GITHUB_RUN_ID is required}"
: "${STATUS_CONTEXT:?STATUS_CONTEXT is required}"
: "${STATUS_STATE:?STATUS_STATE is required}"

status_description="${STATUS_DESCRIPTION:-CI ${STATUS_STATE}}"
target_url="${STATUS_TARGET_URL:-${GITHUB_SERVER_URL:-https://github.com}/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}}"
head_sha="$(git rev-parse HEAD)"

gh api \
  --method POST \
  -H "Accept: application/vnd.github+json" \
  "repos/${GITHUB_REPOSITORY}/statuses/${head_sha}" \
  -f state="${STATUS_STATE}" \
  -f context="${STATUS_CONTEXT}" \
  -f description="${status_description}" \
  -f target_url="${target_url}" >/dev/null

echo "Published ${STATUS_STATE} status for ${STATUS_CONTEXT} on ${head_sha}"
