#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
for language in cpp20 go; do
  for stage in compiler runtime; do
    docker build --target "$stage" -t "gcoj/judge-${language}:1.1.0-${stage}" "$root/judge-worker/languages/$language"
  done
done
