#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
target_repo_root=${1:-}

if [[ -z "$target_repo_root" ]]; then
  echo "usage: $0 /absolute/path/to/agent-skills" >&2
  exit 1
fi

target_repo_root=$(cd "$target_repo_root" && pwd)
marketplace_json="$target_repo_root/.claude-plugin/marketplace.json"
plugin_root="$target_repo_root/plugins/rtk"
skill_root="$plugin_root/skills/rtk"

if [[ ! -f "$marketplace_json" ]]; then
  echo "missing marketplace manifest: $marketplace_json" >&2
  exit 1
fi

version="${VERSION:-}"
if [[ -z "$version" ]]; then
  tag=$(git -C "$repo_root" describe --tags --exact-match 2>/dev/null || true)
  if [[ -n "$tag" ]]; then
    version="${tag#v}"
  else
    echo "VERSION is required unless roundtable-kernel HEAD is tagged with v*" >&2
    exit 1
  fi
fi

mkdir -p "$plugin_root/.claude-plugin"
"$repo_root/scripts/package-rtk-skill.sh" "$skill_root"

cat >"$plugin_root/.claude-plugin/plugin.json" <<EOF
{
  "name": "rtk",
  "version": "$version",
  "description": "Roundtable Kernel runtime skill with bundled CLI and UI",
  "author": {
    "name": "yansircc"
  },
  "repository": "https://github.com/yansircc/roundtable-kernel",
  "license": "MIT",
  "keywords": ["rtk", "roundtable", "workflow", "debate", "cli"]
}
EOF

tmp_json=$(mktemp)
jq \
  --arg version "$version" \
  '
    .plugins = (
        ((.plugins // []) | map(select(.name != "rtk")))
        + [
          {
            "name": "rtk",
            "source": "./plugins/rtk",
            "description": "Roundtable Kernel runtime skill with bundled CLI and UI",
            "version": $version,
            "author": {
              "name": "yansircc"
            }
          }
        ]
        | sort_by(.name)
      )
  ' \
  "$marketplace_json" >"$tmp_json"
mv "$tmp_json" "$marketplace_json"

printf 'synced rtk plugin into %s\n' "$target_repo_root"
