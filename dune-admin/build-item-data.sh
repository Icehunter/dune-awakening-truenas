#!/usr/bin/env bash
set -euo pipefail

RAW_PATH="${1:-item-data-raw.json}"
OUT_PATH="${2:-item-data.json}"

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required" >&2
  exit 1
fi

if [ ! -f "$RAW_PATH" ]; then
  echo "Input file not found: $RAW_PATH" >&2
  echo "Run fetch-item-data.js in the browser on dune.gaming.tools to generate it." >&2
  exit 1
fi

echo "Parsing $RAW_PATH..."

jq '
  {
    default_stack_max: 1,
    default_volume: 0,
    items: (
      [
        to_entries[] |
        .key as $id |
        .value as $item |
        select($item | type == "array" and length > 1) |
        ($item[0]) as $s |
        select($s | type == "object") |
        {
          key: $id,
          value: {
            name:      (if $s.name         | type == "number" then $item[$s.name]         else null end),
            stack_max: (if $s.maxStackSize  | type == "number" then $item[$s.maxStackSize] else 1    end // 1),
            volume:    (if $s.volume        | type == "number" then $item[$s.volume]       else 0    end // 0),
            tier:      (if $s.tier          | type == "number" then $item[$s.tier]         else null end),
            rarity:    (if $s.rarity        | type == "number" then $item[$s.rarity]       else null end)
          }
        }
      ] | from_entries
    )
  }
' "$RAW_PATH" > "$OUT_PATH"

echo "Wrote $OUT_PATH"
