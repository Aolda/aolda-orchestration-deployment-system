#!/usr/bin/env bash

set -u

status=0

check_cmd() {
  local cmd="$1"
  if command -v "$cmd" >/dev/null 2>&1; then
    printf '[ok] %s found\n' "$cmd"
  else
    printf '[warn] %s not found\n' "$cmd"
    status=1
  fi
}

check_file() {
  local path="$1"
  if [ -f "$path" ]; then
    printf '[ok] %s exists\n' "$path"
  else
    printf '[warn] %s is missing\n' "$path"
    status=1
  fi
}

check_env() {
  local name="$1"
  if [ -n "${!name:-}" ]; then
    printf '[ok] %s is set\n' "$name"
  else
    printf '[warn] %s is not set\n' "$name"
    status=1
  fi
}

check_optional_file() {
  local path="$1"
  if [ -f "$path" ]; then
    printf '[ok] optional %s exists\n' "$path"
  else
    printf '[info] optional %s is not configured\n' "$path"
  fi
}

check_optional_env() {
  local name="$1"
  if [ -n "${!name:-}" ]; then
    printf '[ok] optional %s is set\n' "$name"
  else
    printf '[info] optional %s is not set\n' "$name"
  fi
}

printf 'Harness setup check for AODS\n'

check_cmd git
check_cmd make
check_cmd go
check_cmd node
check_cmd npm

if command -v codex >/dev/null 2>&1; then
  printf '[ok] codex found\n'
elif command -v claude >/dev/null 2>&1; then
  printf '[ok] claude found\n'
else
  printf '[warn] neither codex nor claude was found\n'
  status=1
fi

check_optional_file config/mcporter.json
check_optional_env GCAL_CALENDAR_ID

if [ "$status" -eq 0 ]; then
  printf 'Harness setup baseline looks ready.\n'
else
  printf 'Harness setup baseline is incomplete.\n'
fi

exit "$status"
