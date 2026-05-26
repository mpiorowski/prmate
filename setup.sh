#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_DIR="${INSTALL_DIR:-${HOME}/.local/bin}"
GO_VERSION="${GO_VERSION:-1.25.4}"
GH_VERSION="${GH_VERSION:-2.88.1}"

detect_os() {
    case "$(uname -s)" in
        Linux) echo "linux" ;;
        Darwin) echo "darwin" ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        *)
            echo "Unsupported operating system: $(uname -s)" >&2
            exit 1
            ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64) echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *)
            echo "Unsupported architecture: $(uname -m)" >&2
            exit 1
            ;;
    esac
}

download_file() {
    local url="$1"
    local output="$2"

    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" -o "$output"
    elif command -v wget >/dev/null 2>&1; then
        wget -qO "$output" "$url"
    else
        echo "Error: curl or wget is required to download files." >&2
        exit 1
    fi
}

install_go() {
    local os arch go_root
    os="$(detect_os)"
    arch="$(detect_arch)"
    go_root="${HOME}/.local/go"

    echo "Installing Go ${GO_VERSION} (${os}/${arch})..."
    rm -rf "$go_root"
    mkdir -p "$go_root"

    if [[ "$os" == "windows" ]]; then
        local archive archive_path
        archive="go${GO_VERSION}.${os}-${arch}.zip"
        archive_path="$(mktemp)"
        download_file "https://go.dev/dl/${archive}" "$archive_path"
        unzip -q "$archive_path" -d "$go_root"
        mv "$go_root/go/"* "$go_root/"
        rmdir "$go_root/go"
        rm -f "$archive_path"
    else
        local tarball archive_path
        tarball="go${GO_VERSION}.${os}-${arch}.tar.gz"
        archive_path="$(mktemp)"
        download_file "https://go.dev/dl/${tarball}" "$archive_path"
        tar -xzf "$archive_path" -C "$go_root" --strip-components=1
        rm -f "$archive_path"
    fi

    export PATH="${go_root}/bin:${PATH}"
    echo "Go installed to ${go_root}"
}

install_gh() {
    local os arch tmpdir
    os="$(detect_os)"
    arch="$(detect_arch)"
    tmpdir="$(mktemp -d)"

    echo "Installing GitHub CLI ${GH_VERSION} (${os}/${arch})..."

    if [[ "$os" == "windows" ]]; then
        local archive archive_path
        archive="gh_${GH_VERSION}_windows_${arch}.zip"
        archive_path="${tmpdir}/${archive}"
        download_file "https://github.com/cli/cli/releases/download/v${GH_VERSION}/${archive}" "$archive_path"
        unzip -q "$archive_path" -d "$tmpdir"
        mkdir -p "$INSTALL_DIR"
        mv "$tmpdir"/gh_"${GH_VERSION}"_windows_"${arch}"/bin/gh.exe "$INSTALL_DIR/gh.exe"
    else
        local tarball archive_path
        tarball="gh_${GH_VERSION}_${os}_${arch}.tar.gz"
        archive_path="${tmpdir}/${tarball}"
        download_file "https://github.com/cli/cli/releases/download/v${GH_VERSION}/${tarball}" "$archive_path"
        tar -xzf "$archive_path" -C "$tmpdir"
        mkdir -p "$INSTALL_DIR"
        mv "$tmpdir"/gh_*/bin/gh "$INSTALL_DIR/gh"
    fi

    rm -rf "$tmpdir"
    echo "GitHub CLI installed to ${INSTALL_DIR}"
}

binary_name() {
    if [[ "$(detect_os)" == "windows" ]]; then
        echo "$1.exe"
    else
        echo "$1"
    fi
}

if ! command -v git >/dev/null 2>&1; then
    echo "Error: git is required and was not found in PATH." >&2
    exit 1
fi

if ! command -v go >/dev/null 2>&1; then
    install_go
fi

if ! command -v gh >/dev/null 2>&1; then
    install_gh
fi

echo "Building pr CLI..."
cd "$SCRIPT_DIR"
PR_BINARY="$(binary_name pr)"
go build -o "$PR_BINARY" .

echo "Installing to ${INSTALL_DIR}/${PR_BINARY}..."
mkdir -p "$INSTALL_DIR"
mv "$PR_BINARY" "$INSTALL_DIR/$PR_BINARY"

if ! gh auth status >/dev/null 2>&1; then
    echo ""
    echo "GitHub CLI is installed but not authenticated."
    echo "Run: gh auth login"
fi

if [[ ":$PATH:" != *":${INSTALL_DIR}:"* ]]; then
    echo ""
    echo "WARNING: ${INSTALL_DIR} is not in your PATH."
    echo "Add it with:"
    echo "  fish:     fish_add_path ${INSTALL_DIR}"
    echo "  bash/zsh: export PATH=\"${INSTALL_DIR}:\$PATH\""
fi

echo ""
echo "Done. Run '${PR_BINARY} --help' to get started."
echo "Review defaults to two LLMs: '${PR_BINARY} review --pr <number>'"
echo "Single reviewer mode: '${PR_BINARY} review --pr <number> --llm claude'"
