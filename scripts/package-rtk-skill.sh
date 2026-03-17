#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
source_skill_root="$repo_root/.codex/skills/rtk"
target_skill_root="${1:-$source_skill_root}"
version="${VERSION:-dev}"
commit="${COMMIT:-$(git -C "$repo_root" rev-parse --short HEAD 2>/dev/null || echo unknown)}"
date_utc="${DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
ldflags="-X 'main.version=$version' -X 'main.commit=$commit' -X 'main.date=$date_utc'"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
  x86_64) arch=amd64 ;;
  aarch64) arch=arm64 ;;
esac

mkdir -p "$target_skill_root/scripts" "$target_skill_root/ui"
go build -trimpath -ldflags "$ldflags" -o "$target_skill_root/scripts/rtk-${os}-${arch}" "$repo_root/cmd/rtk"
if [[ ! -d "$repo_root/ui/node_modules" ]]; then
  npm ci --prefix "$repo_root/ui"
fi
npm run build --prefix "$repo_root/ui"
rm -rf "$target_skill_root/ui/dist"
cp -R "$repo_root/ui/dist" "$target_skill_root/ui/dist"

source_skill_abs=$(cd "$source_skill_root" && pwd)
target_skill_abs=$(cd "$target_skill_root" && pwd)
if [[ "$source_skill_abs" != "$target_skill_abs" ]]; then
  cp "$source_skill_root/SKILL.md" "$target_skill_root/SKILL.md"
  cp "$source_skill_root/scripts/rtk" "$target_skill_root/scripts/rtk"
fi

chmod +x "$target_skill_root/scripts/rtk"
chmod +x "$target_skill_root/scripts/rtk-${os}-${arch}"
printf 'packaged rtk skill at %s\n' "$target_skill_root"
