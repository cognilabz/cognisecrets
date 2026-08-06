#!/usr/bin/env bash
set -euo pipefail

tag="${1:-}"
image="${2:-}"
previous_tag="${3:-}"

if [[ ! "${tag}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  printf 'usage: %s vX.Y.Z ghcr.io/cognilabz/cognisecrets:vX.Y.Z [previous-tag]\n' "$0" >&2
  exit 1
fi

if [[ -z "${image}" ]]; then
  printf 'image is required\n' >&2
  exit 1
fi

if [[ -n "${previous_tag}" && ! "${previous_tag}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  printf 'previous tag must be a semantic tag like v0.2.5\n' >&2
  exit 1
fi

mkdir -p docs/release-notes

release_date="$(date -u +%F)"
notes_file="docs/release-notes/${tag}.md"

range_description="initial release"
log_range=()
if [[ -n "${previous_tag}" ]] && git rev-parse -q --verify "refs/tags/${previous_tag}" >/dev/null; then
  range_description="${previous_tag}..${tag}"
  log_range=("${previous_tag}..HEAD")
fi

changes="$(git log --reverse --no-merges --format='- %s (%h)' "${log_range[@]}" 2>/dev/null || true)"
if [[ -z "${changes}" ]]; then
  changes="- Rendered release manifests for ${tag}."
fi

cat >"${notes_file}" <<EOF_NOTES
# ${tag} Release Notes

## Status

\`${tag}\` is a CogniSecrets release generated from git history and the rendered install manifest.

Release date: ${release_date}

Release image:

\`\`\`text
${image}
\`\`\`

History range:

\`\`\`text
${range_description}
\`\`\`

## Changes

${changes}

## API And Compatibility

The served API remains:

\`\`\`text
cognilabz.com/v1
\`\`\`

Review the change list above for user-visible behavior changes. Incompatible changes must be called out explicitly before publication.

## Migration

No migration is generated automatically for this release. If a release includes a behavior or manifest compatibility change, maintainers should document it before triggering the release workflow.

## Known Limitations

- CogniSecrets does not provide encryption, rotation, generation, external provider access, or key-level authorization.
- Published-image smoke testing should be performed after tag publication.

## Conformance

The GitHub release workflow runs generated-file verification, unit tests, manifest rendering verification, manager build, whitespace checks, image publication, and release manifest publication.

The local \`kind\` E2E suite is not run by the GitHub release workflow; maintainers should run \`make release-gate\` before publication when E2E confirmation is required.
EOF_NOTES

tmp_readme="$(mktemp)"
awk -v tag="${tag}" '
  BEGIN {
    inserted = 0
    skipping = 0
  }
  /^[0-9]+\. \[v[0-9]+\.[0-9]+\.[0-9]+ release notes\]\(docs\/release-notes\/v[0-9]+\.[0-9]+\.[0-9]+\.md\)$/ {
    if (!inserted) {
      match($0, /^[0-9]+/)
      number = substr($0, RSTART, RLENGTH)
      printf "%s. [%s release notes](docs/release-notes/%s.md)\n", number, tag, tag
      inserted = 1
    }
    skipping = 1
    next
  }
  {
    skipping = 0
    print
    if (!inserted && $0 ~ /^[0-9]+\. \[Operations\]\(docs\/12-operations\.md\)$/) {
      match($0, /^[0-9]+/)
      number = substr($0, RSTART, RLENGTH)
      printf "%d. [%s release notes](docs/release-notes/%s.md)\n", number + 1, tag, tag
      inserted = 1
    }
  }
  END {
    if (!inserted) {
      printf "\n## Release Notes\n\n- [%s release notes](docs/release-notes/%s.md)\n", tag, tag
    }
  }
' README.md >"${tmp_readme}"
mv "${tmp_readme}" README.md

printf 'generated %s and updated README.md\n' "${notes_file}"
