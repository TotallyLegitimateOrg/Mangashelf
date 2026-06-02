#!/bin/sh
set -eu

# <asset_name>_<version>_<os>_<arch>.
github_owner="TotallyLegitimateOrg"
github_repo="Mangashelf"
asset_name="mangashelf"
binary_name="mangashelf"

move_to_path="${MOVE:-1}"
if [ "$move_to_path" = "1" ] || [ "$move_to_path" = "true" ]; then
	install_dir="${INSTALL_DIR:-/usr/local/bin}"
else
	install_dir="${INSTALL_DIR:-$(pwd)}"
fi
version="${VERSION:-latest}"

need() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "install.sh: missing required command: $1" >&2
		exit 1
	fi
}

need tar
need uname
need mktemp

http_get() {
	url="$1"
	if command -v curl >/dev/null 2>&1; then
		curl -fL "$url"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO- "$url"
	else
		echo "install.sh: missing required command: curl or wget" >&2
		exit 1
	fi
}

os="${OS:-}"
if [ -z "$os" ]; then
	os="$(uname -s | tr '[:upper:]' '[:lower:]')"
fi
case "$os" in
	darwin|linux) ;;
	mingw*|msys*|cygwin*|windows*) os="windows" ;;
	*) echo "install.sh: unsupported OS: $os" >&2; exit 1 ;;
esac

arch="${ARCH:-}"
if [ -z "$arch" ]; then
	arch="$(uname -m)"
fi
case "$arch" in
	aarch64|arm64) arch="arm64" ;;
	x86_64|amd64) arch="amd64" ;;
	*) echo "install.sh: unsupported architecture: $arch" >&2; exit 1 ;;
esac

case "${os}_${arch}" in
	darwin_arm64|linux_amd64|linux_arm64|windows_amd64) ;;
	*) echo "install.sh: unsupported ${github_repo} release target: ${os}/${arch}" >&2; exit 1 ;;
esac

if [ "$os" = "windows" ]; then
	archive_ext="zip"
	archive_binary="${binary_name}.exe"
	need unzip
else
	archive_ext="tar.gz"
	archive_binary="$binary_name"
fi
bin_name="${BIN_NAME:-$archive_binary}"

if [ "$version" = "latest" ]; then
	latest_url="https://api.github.com/repos/${github_owner}/${github_repo}/releases/latest"
	version="$(http_get "$latest_url" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
	if [ -z "$version" ]; then
		echo "install.sh: could not resolve latest release" >&2
		exit 1
	fi
fi

case "$version" in
	v*) tag="$version"; asset_version="${version#v}" ;;
	*) tag="v$version"; asset_version="$version" ;;
esac

asset="${asset_name}_${asset_version}_${os}_${arch}.${archive_ext}"
url="https://github.com/${github_owner}/${github_repo}/releases/download/${tag}/${asset}"
tmp_dir="$(mktemp -d)"

cleanup() {
	rm -rf "$tmp_dir"
}
trap cleanup EXIT INT TERM

if [ "$move_to_path" = "1" ] || [ "$move_to_path" = "true" ]; then
	echo "Installing ${github_owner}/${github_repo} ${tag} (${os}/${arch})"
else
	echo "Downloading ${github_owner}/${github_repo} ${tag} (${os}/${arch})"
fi
if [ "$archive_ext" = "zip" ]; then
	http_get "$url" > "$tmp_dir/archive.zip"
	unzip -q "$tmp_dir/archive.zip" -d "$tmp_dir"
else
	http_get "$url" | tar -xz -C "$tmp_dir"
fi

if [ ! -f "$tmp_dir/$archive_binary" ]; then
	echo "install.sh: release archive did not contain $archive_binary" >&2
	exit 1
fi

mkdir -p "$install_dir"
chmod 0755 "$tmp_dir/$archive_binary"

if mv "$tmp_dir/$archive_binary" "$install_dir/$bin_name" 2>/dev/null; then
	:
elif command -v sudo >/dev/null 2>&1; then
	sudo mv "$tmp_dir/$archive_binary" "$install_dir/$bin_name"
else
	echo "install.sh: could not write to $install_dir and sudo is unavailable" >&2
	exit 1
fi

if [ "$move_to_path" = "1" ] || [ "$move_to_path" = "true" ]; then
	echo "Installed at $install_dir/$bin_name"
else
	echo "Downloaded to $install_dir/$bin_name"
fi
