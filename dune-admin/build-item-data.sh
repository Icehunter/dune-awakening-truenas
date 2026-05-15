#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-https://cdn-hosted.gaming.tools/dune/data/en}"
ENTITIES_PATH="${1:-entities.d.json}"
OUT_PATH="${2:-item-data.json}"
UA="${UA:-Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36}"
CF_CLEARANCE="${CF_CLEARANCE:-}"
ITEM_VERSION_PARAM="${ITEM_VERSION_PARAM:-}"
ENTITIES_VERSION_PARAM="${ENTITIES_VERSION_PARAM:-}"
ITEMS_DIR="${ITEMS_DIR:-}"

CURL_HEADERS=(
  -H "accept: application/json"
  -H "accept-language: en-CA,en;q=0.8"
  -H "cache-control: no-cache"
  -H "dnt: 1"
  -H "origin: https://dune.gaming.tools"
  -H "pragma: no-cache"
  -H "priority: u=1, i"
  -H "referer: https://dune.gaming.tools/"
  -H "sec-ch-ua: \"Chromium\";v=\"148\", \"Brave\";v=\"148\", \"Not/A)Brand\";v=\"99\""
  -H "sec-ch-ua-mobile: ?0"
  -H "sec-ch-ua-platform: \"macOS\""
  -H "sec-fetch-dest: empty"
  -H "sec-fetch-mode: cors"
  -H "sec-fetch-site: same-site"
  -H "sec-gpc: 1"
)

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required" >&2
  exit 1
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

if [ ! -f "$ENTITIES_PATH" ]; then
  echo "Downloading entities index..."
  entities_url="$BASE_URL/entities.d.json"
  if [ -n "$ENTITIES_VERSION_PARAM" ]; then
    entities_url="${entities_url}?version=${ENTITIES_VERSION_PARAM}"
  fi
  if [ -n "$CF_CLEARANCE" ]; then
    curl -fsSL --compressed -A "$UA" "${CURL_HEADERS[@]}" \
      -H "cookie: cf_clearance=${CF_CLEARANCE}" \
      -o "$ENTITIES_PATH" "$entities_url"
  else
    curl -fsSL --compressed -A "$UA" "${CURL_HEADERS[@]}" \
      -o "$ENTITIES_PATH" "$entities_url"
  fi
fi

echo "Extracting item ids + volume..."
jq -r '
  def deref($p; $v):
    if ($v|type)=="number" then $p[$v] else $v end;
  . as $p
  | $p[0][] as $i
  | ($p[$i]) as $o
  | select($o|type=="object")
  | select($o.id? != null)
  | [deref($p; $o.id), (deref($p; ($o.volume? // empty)) // empty)] | @tsv
' "$ENTITIES_PATH" > "$tmp_dir/volumes.tsv"

mkdir -p "$tmp_dir/items"

if [ -n "$ITEMS_DIR" ]; then
  echo "Using local item files from $ITEMS_DIR..."
else
  echo "Downloading item files..."
fi
FAIL_FAST="${FAIL_FAST:-1}"
while IFS=$'\t' read -r item_id _volume; do
  if [ -n "$ITEMS_DIR" ]; then
    item_path="$ITEMS_DIR/${item_id}.d.json"
  else
    item_path="$tmp_dir/items/${item_id}.d.json"
  fi
  if [ -s "$item_path" ]; then
    continue
  fi
  if [ -n "$ITEMS_DIR" ]; then
    echo "Missing local item file: $item_path" >&2
    if [ "$FAIL_FAST" -eq 1 ]; then
      exit 1
    fi
    echo "$item_id" >> "$tmp_dir/failures.txt"
    continue
  fi
  item_url="$BASE_URL/items/${item_id}.d.json"
  if [ -n "$ITEM_VERSION_PARAM" ]; then
    item_url="${item_url}?version=${ITEM_VERSION_PARAM}"
  fi
  if [ -n "$CF_CLEARANCE" ]; then
    if ! curl -fsSL --compressed -A "$UA" "${CURL_HEADERS[@]}" \
      -H "cookie: cf_clearance=${CF_CLEARANCE}" \
      -o "$item_path" "$item_url"; then
      echo "Download failed: $item_url" >&2
      if [ "$FAIL_FAST" -eq 1 ]; then
        exit 1
      fi
      echo "$item_id" >> "$tmp_dir/failures.txt"
    fi
  else
    if ! curl -fsSL --compressed -A "$UA" "${CURL_HEADERS[@]}" \
      -o "$item_path" "$item_url"; then
      echo "Download failed: $item_url" >&2
      if [ "$FAIL_FAST" -eq 1 ]; then
        exit 1
      fi
      echo "$item_id" >> "$tmp_dir/failures.txt"
    fi
  fi
done < "$tmp_dir/volumes.tsv"

if [ -s "$tmp_dir/failures.txt" ]; then
  echo "Some item files failed to download. First 10:" >&2
  head -n 10 "$tmp_dir/failures.txt" >&2
fi

echo "Building item-data.json..."
{
  echo '{'
  echo '  "default_stack_max": 1,'
  echo '  "default_volume": 0,'
  echo '  "items": {'
  first=1
  while IFS=$'\t' read -r item_id volume; do
    item_path="$tmp_dir/items/${item_id}.d.json"
    stack_max=1
    if [ -s "$item_path" ]; then
      stack_max="$(jq -r '
        def resolve($p):
          if type=="number" then $p[.] | resolve($p)
          elif type=="array" then map(resolve($p))
          elif type=="object" then with_entries(.value |= resolve($p))
          else .
          end;
        def find_stack($o):
          [ $o | .. | objects | to_entries[]
            | select(.key|test("stack";"i"))
            | .value | select(type=="number") ] | max // empty;
        . as $p
        | [ $p[0][] | select($p[.]|type=="object")
            | ($p[.] | resolve($p))
            | find_stack(.) ] | map(select(. != null)) | .[0] // empty
      ' "$item_path" 2>/dev/null || true)"
      if [ -z "$stack_max" ] || [ "$stack_max" = "null" ]; then
        stack_max=1
      fi
    fi
    if [ "$first" -eq 0 ]; then
      echo ","
    fi
    first=0
    printf '    "%s": {"stack_max": %s, "volume": %s}' \
      "$item_id" "$stack_max" "${volume:-0}"
  done < "$tmp_dir/volumes.tsv"
  echo
  echo '  }'
  echo '}'
} > "$OUT_PATH"

echo "Wrote $OUT_PATH"
