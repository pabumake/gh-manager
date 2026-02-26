#!/usr/bin/env bash
set -euo pipefail

REPO="${GHM_REPO:-pabumake/gh-manager}"
API_URL="https://api.github.com/repos/${REPO}/releases/latest"
INSTALL_DIR="/usr/local/bin"
BIN_NAME="gh-manager"
MODE="install"

if [[ "${1:-}" == "--uninstall" || "${1:-}" == "-u" ]]; then
  MODE="uninstall"
fi

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "error: required command not found: $1" >&2
    exit 1
  }
}

fetch() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- "$1"
  else
    echo "error: curl or wget is required" >&2
    exit 1
  fi
}

download_file() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$1" -o "$2"
  else
    wget -qO "$2" "$1"
  fi
}

bin_path="${INSTALL_DIR}/${BIN_NAME}"

if [[ "${MODE}" == "uninstall" ]]; then
  echo "Uninstalling ${BIN_NAME}..."
  if command -v "${BIN_NAME}" >/dev/null 2>&1; then
    "${BIN_NAME}" theme apply default >/dev/null 2>&1 || true
    "${BIN_NAME}" theme uninstall catppuccin-mocha >/dev/null 2>&1 || true
  fi
  if [[ -e "${bin_path}" ]]; then
    if [[ -w "${INSTALL_DIR}" ]]; then
      rm -f "${bin_path}"
    else
      need_cmd sudo
      sudo rm -f "${bin_path}"
    fi
    echo "Uninstall complete: removed ${bin_path}"
  else
    echo "Nothing to uninstall: ${bin_path} not found"
  fi
  exit 0
fi

need_cmd tar
need_cmd mktemp

os_raw="$(uname -s)"
arch_raw="$(uname -m)"

case "${os_raw}" in
  Linux) os="linux" ;;
  Darwin) os="darwin" ;;
  *)
    echo "error: unsupported OS: ${os_raw}" >&2
    exit 1
    ;;
esac

case "${arch_raw}" in
  x86_64|amd64) arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
  *)
    echo "error: unsupported architecture: ${arch_raw}" >&2
    exit 1
    ;;
esac

asset_name="${BIN_NAME}_${os}_${arch}.tar.gz"
release_json="$(fetch "${API_URL}")"
asset_url="$(
  printf '%s\n' "${release_json}" \
    | grep '"browser_download_url"' \
    | sed -E 's/.*"browser_download_url":[[:space:]]*"([^"]+)".*/\1/' \
    | grep "/${asset_name}$" \
    | head -n1
)"

if [[ -z "${asset_url}" ]]; then
  echo "error: release asset not found: ${asset_name}" >&2
  exit 1
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

archive_path="${tmpdir}/${asset_name}"
echo "Downloading ${asset_name}..."
download_file "${asset_url}" "${archive_path}"

echo "Extracting ${asset_name}..."
tar -xzf "${archive_path}" -C "${tmpdir}"

src_bin="${tmpdir}/${BIN_NAME}_${os}_${arch}/${BIN_NAME}"
if [[ ! -f "${src_bin}" ]]; then
  echo "error: extracted binary not found: ${src_bin}" >&2
  exit 1
fi

echo "Installing to ${INSTALL_DIR}/${BIN_NAME}..."
if [[ -w "${INSTALL_DIR}" ]]; then
  install -m 0755 "${src_bin}" "${INSTALL_DIR}/${BIN_NAME}"
else
  need_cmd sudo
  sudo install -m 0755 "${src_bin}" "${INSTALL_DIR}/${BIN_NAME}"
fi

echo "Applying theme: catppuccin-mocha..."
"${bin_path}" theme install catppuccin-mocha
"${bin_path}" theme apply catppuccin-mocha

echo "Install complete."
echo "Version: $("${bin_path}" version)"
echo "$("${bin_path}" theme current)"
