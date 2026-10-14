#!/usr/bin/env bash
# Single source of truth for kb release builds: the OS/arch/variant matrix,
# tags, flags, and naming one place so every release path (GitHub Actions,
# Forgejo Actions, local) produces identical binaries.
#
# Usage:
#   scripts/build.sh --os <os> --arch <arch> [--variant standard] \
#       [--version <v>] [--out <dir>]
#
# Binary naming: kb-<os>-<arch> (standard variants omit the variant suffix,
# kb-<os>-<arch>-<variant> otherwise).
#
# cgo is mandatory: mattn/go-sqlite3 (plus FTS5) live in the cgo layer, so
# CGO_ENABLED=1 is forced here. CGO_ENABLED=0 binaries compile but die at
# runtime (see AGENTS.md). Cross builds therefore need a native toolchain
# for the target arch (e.g. a linux-arm64 runner image) - CGO can never be
# crossed from a foreign host without an SDK.
set -euo pipefail

OS="" ARCH="" VARIANT="standard" VERSION="" OUT="dist"

while [[ $# -gt 0 ]]; do
	case "$1" in
	--os) OS="$2"; shift 2 ;;
	--arch) ARCH="$2"; shift 2 ;;
	--variant) VARIANT="$2"; shift 2 ;;
	--version) VERSION="$2"; shift 2 ;;
	--out) OUT="$2"; shift 2 ;;
	*) echo "build.sh: unknown flag $1" >&2; exit 2 ;;
	esac
done
[[ -n "$OS" && -n "$ARCH" ]] || { echo "usage: build.sh --os OS --arch ARCH [--variant V] [--version V] [--out DIR]" >&2; exit 2; }

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# Variant feature gates. The onnx variant needs the opt-in embedder sources;
# guarding here keeps an unimplemented variant from shipping a broken binary.
case "$VARIANT" in
	standard)
		TAGS=(sqlite_fts5)
	;;
	onnx)
		if [[ ! -d internal/embed/onnx ]]; then
			echo "onnx variant: internal/embed/onnx does not exist (opt-in embedder not implemented yet)" >&2
			exit 3
		fi
		TAGS=(sqlite_fts5 onnx)
	;;
	*)
		echo "build.sh: unknown variant '$VARIANT'" >&2; exit 2 ;;
esac

# Link flags: strip symbols; stamp the version when one is provided.
LDFLAGS=( -s -w )
[[ -n "$VERSION" ]] && LDFLAGS+=( -X "main.version=$VERSION" )

export CGO_ENABLED=1
export CGO_CFLAGS="-DSQLITE_ENABLE_FTS5"
export GOOS="$OS" GOARCH="$ARCH"

STAMP="kb-$OS-$ARCH"
[[ "$VARIANT" != standard ]] && STAMP="$STAMP-$VARIANT"
mkdir -p "$OUT"

go build -tags "$(IFS=,; echo "${TAGS[*]}")" \
	-ldflags "${LDFLAGS[*]}" \
	-o "$OUT/$STAMP" ./cmd/kb

# Future variants bundle dylibs in the same package; keep the package format
# per-OS so each asset can carry its own runtime requirements.
case "$OS" in
	windows) mv "$OUT/$STAMP" "$OUT/$STAMP.exe" 2>/dev/null || true ;;
esac

echo "built $OUT/$STAMP ($OS/$ARCH, $VARIANT)"
