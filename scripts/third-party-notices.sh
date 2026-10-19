#!/usr/bin/env bash
# Writes frontend/src/assets/third-party.json: every Go module linked into the
# app and every frontend package bundled into it, with its license, read from
# the package's own LICENSE file / package.json. Shown on About › License.
# Run after changing dependencies:  make notices
set -euo pipefail
cd "$(dirname "$0")/.."
out=frontend/src/assets/third-party.json
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

classify() { # LICENSE text on stdin → SPDX id
  local t
  t=$(head -c 4000 | tr '\n' ' ')
  case "$t" in
    *"Apache License"*) echo "Apache-2.0" ;;
    *"Mozilla Public License"*) echo "MPL-2.0" ;;
    *"Permission is hereby granted, free of charge"*) echo "MIT" ;;
    *"Redistribution and use in source and binary forms"*)
      if grep -q "Neither the name" <<<"$t"; then echo "BSD-3-Clause"; else echo "BSD-2-Clause"; fi ;;
    *"Permission to use, copy, modify, and/or distribute"*) echo "ISC" ;;
    *) echo "UNKNOWN" ;;
  esac
}

{
  echo "go|Go standard library|$(go env GOVERSION)|BSD-3-Clause"
  go list -deps -f '{{if .Module}}{{.Module.Path}}|{{.Module.Version}}|{{.Module.Dir}}{{end}}' . | sort -u | grep -v '^mango3s' |
    while IFS='|' read -r mod ver dir; do
      f=$(ls "$dir" 2>/dev/null | grep -i -E '^(licen[cs]e|copying)' | head -1 || true)
      lic=UNKNOWN
      [ -n "$f" ] && lic=$(classify <"$dir/$f")
      echo "go|$mod|$ver|$lic"
    done
  (cd frontend && npm ls --omit=dev --all --parseable 2>/dev/null | tail -n +2 | sort -u) |
    while read -r d; do
      node -e 'const p=require(process.argv[1]+"/package.json");const l=typeof p.license==="string"?p.license:(p.license&&p.license.type)||"UNKNOWN";console.log("js|"+p.name+"|"+p.version+"|"+l)' "frontend/$d" 2>/dev/null ||
        node -e 'const p=require(process.argv[1]+"/package.json");const l=typeof p.license==="string"?p.license:(p.license&&p.license.type)||"UNKNOWN";console.log("js|"+p.name+"|"+p.version+"|"+l)' "$d"
    done
} | sort -t'|' -k1,1 -k2,2 -u >"$tmp"

if grep -q '|UNKNOWN$' "$tmp"; then
  echo "Licenses that could not be identified (check them by hand):" >&2
  grep '|UNKNOWN$' "$tmp" >&2
fi
node -e '
const lines=require("fs").readFileSync(process.argv[1],"utf8").trim().split("\n")
const items=lines.map(l=>{const [kind,name,version,license]=l.split("|");return {kind,name,version,license}})
require("fs").writeFileSync(process.argv[2], JSON.stringify(items,null,1)+"\n")
console.log(items.length+" components written to "+process.argv[2])
' "$tmp" "$out"
