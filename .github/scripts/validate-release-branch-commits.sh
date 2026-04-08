#!/usr/bin/env bash

set -euo pipefail

base_ref="${1:-origin/main}"

git fetch --no-tags origin main

commit_range="${base_ref}..HEAD"
commit_shas="$(git rev-list --reverse "${commit_range}")"

if [[ -z "${commit_shas}" ]]; then
  echo "No commits found in ${commit_range}; skipping release-branch commit validation"
  exit 0
fi

conventional_header='^(build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test)(\([[:alnum:]/._-]+\))?(!)?: .+'
invalid=0

while IFS= read -r sha; do
  header="$(git show -s --format='%s' "${sha}")"
  if [[ ! "${header}" =~ ${conventional_header} ]]; then
    printf 'Non-conventional commit header: %s %s\n' "${sha}" "${header}" >&2
    invalid=1
  fi
done <<<"${commit_shas}"

if (( invalid != 0 )); then
  exit 1
fi

echo "Validated conventional commit headers for ${commit_range}"
