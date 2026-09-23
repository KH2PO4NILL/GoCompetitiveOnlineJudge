#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
for language in cpp20 go; do
  for stage in compiler runtime; do
    docker build --target "$stage" -t "gcoj/judge-${language}:1.2.0-${stage}" -f "$root/judge-worker/languages/$language/Dockerfile" "$root/judge-worker/languages"
  done
done
