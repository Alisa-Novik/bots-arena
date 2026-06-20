#!/usr/bin/env bash
set -euo pipefail

target_bots="${TARGET_BOTS:-100000}"
ticks="${TICKS:-300}"
warmup_ticks="${WARMUP_TICKS:-20}"
min_ticks_per_second="${MIN_TICKS_PER_SECOND:-10}"
seed="${SEED:-42}"
bin="${GOLAB_BIN:-./bin/golab}"

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required" >&2
  exit 2
fi

if [[ ! -x "$bin" ]]; then
  mkdir -p "$(dirname "$bin")"
  go build -o "$bin" ./cmd/golab
fi

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

"$bin" scale-test \
  --seed "$seed" \
  --target-bots "$target_bots" \
  --ticks "$ticks" \
  --warmup-ticks "$warmup_ticks" \
  --pretty >"$tmp"

cat "$tmp"

if ! jq -e \
  --argjson target "$target_bots" \
  --argjson ticks "$ticks" \
  --argjson min_tps "$min_ticks_per_second" '
    .initial_live_bots == $target
    and .target_bots == $target
    and .ticks == $ticks
    and .logic_ticks_per_second >= $min_tps
  ' "$tmp" >/dev/null; then
  echo "scale validation failed: expected target=$target_bots ticks=$ticks min_tps=$min_ticks_per_second" >&2
  exit 1
fi
