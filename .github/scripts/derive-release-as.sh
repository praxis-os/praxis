#!/usr/bin/env bash

set -euo pipefail

manifest_file="${MANIFEST_FILE:-.release-please-manifest.json}"
package_path="${PACKAGE_PATH:-.}"
git_range="${GIT_RANGE:-}"

escape_regex() {
  printf '%s' "$1" | sed 's/[][(){}.^$*+?|\/]/\\&/g'
}

write_output() {
  local key="$1"
  local value="$2"

  if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
    printf '%s=%s\n' "$key" "$value" >>"$GITHUB_OUTPUT"
  fi
}

manifest_key="$(escape_regex "$package_path")"
current_version="$(
  sed -En "s/^[[:space:]]*\"${manifest_key}\"[[:space:]]*:[[:space:]]*\"([^\"]+)\".*/\\1/p" "$manifest_file" |
    head -n1
)"

if [[ -z "$current_version" ]]; then
  echo "Could not determine current version for package path '$package_path' from $manifest_file" >&2
  exit 1
fi

if [[ ! "$current_version" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
  echo "Unsupported semantic version in $manifest_file: $current_version" >&2
  exit 1
fi

major="${BASH_REMATCH[1]}"
minor="${BASH_REMATCH[2]}"
has_release_as="false"

if (( major >= 1 )); then
  write_output "has_release_as" "$has_release_as"
  echo "No release override: current version $current_version is already >= 1.0.0" >&2
  exit 0
fi

if [[ -z "$git_range" ]]; then
  last_tag=""
  manifest_tag="v${current_version}"
  if git rev-parse -q --verify "refs/tags/${manifest_tag}" >/dev/null; then
    last_tag="$manifest_tag"
  else
    last_tag="$(git describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null || true)"
  fi
  if [[ -n "$last_tag" ]]; then
    git_range="${last_tag}..HEAD"
  else
    git_range="HEAD"
  fi
fi

log_subjects="$(git log --no-merges --format='%s' "$git_range")"
log_bodies="$(git log --no-merges --format='%b' "$git_range")"

has_feature_commit=false
has_breaking_subject=false
has_breaking_footer=false

if grep -Eiq '^(feat|feature)(\([^)]+\))?(!)?: ' <<<"$log_subjects"; then
  has_feature_commit=true
fi

if grep -Eiq '^[[:alpha:]][[:alnum:]-]*(\([^)]+\))?!: ' <<<"$log_subjects"; then
  has_breaking_subject=true
fi

if grep -Eiq '^BREAKING[ -]CHANGE: ' <<<"$log_bodies"; then
  has_breaking_footer=true
fi

if [[ "$has_feature_commit" != "true" && "$has_breaking_subject" != "true" && "$has_breaking_footer" != "true" ]]; then
  write_output "has_release_as" "$has_release_as"
  echo "No release override: no pre-v1 feature/breaking commits found in $git_range" >&2
  exit 0
fi

if (( minor % 2 == 0 )); then
  next_minor=$((minor + 1))
else
  next_minor=$((minor + 2))
fi

release_as="${major}.${next_minor}.0"
has_release_as="true"

write_output "has_release_as" "$has_release_as"
write_output "release_as" "$release_as"

echo "Derived release override $release_as from current version $current_version" >&2
printf '%s\n' "$release_as"
