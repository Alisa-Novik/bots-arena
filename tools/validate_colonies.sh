#!/usr/bin/env bash
set -euo pipefail

ticks="${TICKS:-8000}"
interval="${INTERVAL:-25}"
top_bots="${TOP_BOTS:-1}"
seeds="${SEEDS:-1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20}"
bin="${GOLAB_BIN:-./bin/golab}"

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required" >&2
  exit 2
fi

if [[ ! -x "$bin" ]]; then
  mkdir -p "$(dirname "$bin")"
  go build -o "$bin" ./cmd/golab
fi

status=0
printf 'seed\tevents\tresource_rain\tfood_rain\tcolony_support\tsettlement_support\tfinal_live\tfinal_divisions\tfinal_active\tfinal_solo\tfinal_max_members\tfinal_max_connected\tpeak_members\tpeak_connected\tpheromone_cells\n'

for seed in $seeds; do
  tmp="$(mktemp)"
  sim_log="$(mktemp)"
  jq_log="$(mktemp)"

  if ! "$bin" gamemaster --seed "$seed" --ticks "$ticks" --interval "$interval" --top-bots "$top_bots" >"$tmp" 2>"$sim_log"; then
    echo "seed $seed: simulator command failed: $bin gamemaster --seed $seed --ticks $ticks --interval $interval --top-bots $top_bots" >&2
    if [[ -s "$sim_log" ]]; then
      echo "seed $seed: simulator stderr follows:" >&2
      sed 's/^/  /' "$sim_log" >&2
    fi
    status=1
    rm -f "$tmp" "$sim_log" "$jq_log"
    continue
  fi

  if ! row="$(
    jq -r --argjson seed "$seed" '
      def count_kind($kind): ([.events[]? | select(.kind == $kind)] | length);
      . as $root
      | ($root.final_summary // ($root.frames[-1].summary)) as $final
      | [
          $seed,
          (($root.events // []) | length),
          count_kind("resource_rain"),
          count_kind("food_rain"),
          count_kind("colony_support"),
          count_kind("settlement_support"),
          ($final.live_bots // 0),
          ($final.successful_divisions // 0),
          ($final.active_colonies // 0),
          ($final.solo_active_colonies // 0),
          ($final.max_colony_members // 0),
          ($final.max_connected_members // 0),
          ([ $root.frames[].summary.max_colony_members ] | max // 0),
          ([ $root.frames[].summary.max_connected_members ] | max // 0),
          ($final.pheromone_active_cells // 0)
        ] | @tsv
    ' "$tmp" 2>"$jq_log"
  )"; then
    echo "seed $seed: failed to parse simulator JSON from $bin" >&2
    if [[ -s "$jq_log" ]]; then
      echo "seed $seed: jq error follows:" >&2
      sed 's/^/  /' "$jq_log" >&2
    fi
    if [[ -s "$sim_log" ]]; then
      echo "seed $seed: simulator stderr follows:" >&2
      sed 's/^/  /' "$sim_log" >&2
    fi
    echo "seed $seed: simulator stdout was captured at $tmp" >&2
    status=1
    rm -f "$sim_log" "$jq_log"
    continue
  fi
  printf '%s\n' "$row"

  IFS=$'\t' read -r _ events resource_rain _ _ _ final_live final_divisions _ _ _ _ peak_members peak_connected pheromone_cells <<<"$row"
  max_resource_rain=$(( events / 4 ))
  if (( max_resource_rain < 1 )); then
    max_resource_rain=1
  fi
  if (( resource_rain > max_resource_rain )); then
    echo "seed $seed: resource_rain=$resource_rain exceeds threshold $max_resource_rain" >&2
    status=1
  fi
  if (( final_live <= 0 || final_divisions <= 0 || pheromone_cells <= 0 )); then
    echo "seed $seed: dead/sterile/pheromone-empty run" >&2
    status=1
  fi
  if (( peak_members < 2 || peak_connected < 1 )); then
    echo "seed $seed: no multi-member connected colony peak" >&2
    status=1
  fi
  rm -f "$tmp" "$sim_log" "$jq_log"
done

exit "$status"
