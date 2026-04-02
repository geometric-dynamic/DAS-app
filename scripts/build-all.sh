#!/usr/bin/env bash

set -u

APP_NAME="DAS-Console"
RAW_OUTPUT_BASENAME="${APP_NAME}"
DIST_DIR="dist"
DEFAULT_TARGETS="linux/amd64,windows/amd64"

targets="$DEFAULT_TARGETS"
nsis=0
clean_dist=1

usage() {
  cat <<'USAGE'
Usage:
  bash scripts/build-all.sh [--targets <os/arch,...>] [--nsis] [--no-clean]

Options:
  --targets   Comma-separated Wails platforms, e.g. linux/amd64,windows/amd64
  --nsis      Generate NSIS installer for Windows targets
  --no-clean  Keep existing files in dist/
  -h, --help  Show this help
USAGE
}

log() {
  printf '[build-all] %s\n' "$*"
}

warn() {
  printf '[build-all] WARN: %s\n' "$*" >&2
}

err() {
  printf '[build-all] ERROR: %s\n' "$*" >&2
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    err "Missing required command: $1"
    exit 1
  fi
}

detect_version() {
  local version
  if version=$(git describe --tags --exact-match 2>/dev/null); then
    printf '%s' "$version"
  else
    printf 'dev'
  fi
}

normalize_target_name() {
  local os="$1"
  local arch="$2"
  local version="$3"
  printf '%s_%s_%s_%s' "$APP_NAME" "$version" "$os" "$arch"
}

copy_if_exists() {
  local src="$1"
  local dest="$2"
  if [ -e "$src" ]; then
    cp -R "$src" "$dest"
    return 0
  fi
  return 1
}

is_windows_binary() {
  local candidate="$1"
  if [ ! -f "$candidate" ]; then
    return 1
  fi

  local file_output
  file_output=$(file "$candidate" 2>/dev/null || true)
  case "$file_output" in
    *"PE32"*|*"MS Windows"*)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

build_target() {
  local target="$1"
  local version="$2"
  local os="${target%%/*}"
  local arch="${target##*/}"
  local normalized
  normalized=$(normalize_target_name "$os" "$arch" "$version")

  log "Building target: ${target}"

  local build_cmd=(wails build -platform "$target" -o "$RAW_OUTPUT_BASENAME" -clean -nocolour)
  if [ "$nsis" -eq 1 ] && [ "$os" = "windows" ]; then
    build_cmd+=( -nsis )
  fi

  if ! "${build_cmd[@]}"; then
    warn "Build failed for ${target}. This usually means the current machine lacks the required native or cross compilation toolchain."
    return 1
  fi

  local copied_any=0

  case "$os" in
    windows)
      if copy_if_exists "build/bin/${RAW_OUTPUT_BASENAME}.exe" "${DIST_DIR}/${normalized}.exe"; then
        log "Created ${DIST_DIR}/${normalized}.exe"
        copied_any=1
      elif copy_if_exists "build/bin/${RAW_OUTPUT_BASENAME}" "${DIST_DIR}/${normalized}.exe"; then
        log "Created ${DIST_DIR}/${normalized}.exe"
        copied_any=1
      elif is_windows_binary "build/bin/${RAW_OUTPUT_BASENAME}"; then
        cp "build/bin/${RAW_OUTPUT_BASENAME}" "${DIST_DIR}/${normalized}.exe"
        log "Created ${DIST_DIR}/${normalized}.exe"
        copied_any=1
      fi
      if [ "$nsis" -eq 1 ]; then
        local installer="build/bin/${RAW_OUTPUT_BASENAME}-${arch}-installer.exe"
        if copy_if_exists "$installer" "${DIST_DIR}/${normalized}_installer.exe"; then
          log "Created ${DIST_DIR}/${normalized}_installer.exe"
          copied_any=1
        fi
      fi
      ;;
    darwin)
      if copy_if_exists "build/bin/${RAW_OUTPUT_BASENAME}.app" "${DIST_DIR}/${normalized}.app"; then
        log "Created ${DIST_DIR}/${normalized}.app"
        copied_any=1
      elif copy_if_exists "build/bin/${RAW_OUTPUT_BASENAME}" "${DIST_DIR}/${normalized}"; then
        log "Created ${DIST_DIR}/${normalized}"
        copied_any=1
      fi
      ;;
    *)
      if copy_if_exists "build/bin/${RAW_OUTPUT_BASENAME}" "${DIST_DIR}/${normalized}"; then
        log "Created ${DIST_DIR}/${normalized}"
        copied_any=1
      fi
      ;;
  esac

  if [ "$copied_any" -eq 0 ]; then
    warn "Build succeeded for ${target}, but no known output artifact was found in build/bin/."
    return 1
  fi

  return 0
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --targets)
      if [ "$#" -lt 2 ]; then
        err "--targets requires a value"
        exit 1
      fi
      targets="$2"
      shift 2
      ;;
    --nsis)
      nsis=1
      shift
      ;;
    --no-clean)
      clean_dist=0
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      err "Unknown argument: $1"
      usage
      exit 1
      ;;
  esac
done

require_cmd git
require_cmd wails

version="$(detect_version)"

mkdir -p "$DIST_DIR"
if [ "$clean_dist" -eq 1 ]; then
  find "$DIST_DIR" -mindepth 1 -maxdepth 1 -exec rm -rf {} +
fi

log "Resolved version: ${version}"
log "Target list: ${targets}"

IFS=',' read -r -a target_list <<< "$targets"

success_count=0
failure_count=0

for target in "${target_list[@]}"; do
  if build_target "$target" "$version"; then
    success_count=$((success_count + 1))
  else
    failure_count=$((failure_count + 1))
  fi
done

log "Finished. success=${success_count}, failed=${failure_count}"
log "Normalized artifacts directory: ${DIST_DIR}/"

if [ "$failure_count" -gt 0 ]; then
  exit 1
fi
