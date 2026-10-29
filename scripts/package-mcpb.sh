#!/usr/bin/env bash
# Packs the Claude Desktop extension (dist/mcp-tiime.mcpb) from the binaries
# GoReleaser left in dist/: the macOS universal binary and the Windows amd64
# one, which Windows on ARM runs through emulation.
#
# Usage: scripts/package-mcpb.sh <version>
set -euo pipefail

version="${1:?usage: $0 <version>}"
dist="dist"
stage="$dist/mcpb"
mcpb_cli="@anthropic-ai/mcpb@2.1.2"

artifact_path() {
	local goos="$1" goarch="$2"
	jq -r --arg goos "$goos" --arg goarch "$goarch" \
		'[.[] | select(.type == "Binary" and .goos == $goos and .goarch == $goarch)][0].path // empty' \
		"$dist/artifacts.json"
}

# goarch "all" is the universal binary.
darwin_binary="$(artifact_path darwin all)"
windows_binary="$(artifact_path windows amd64)"
if [[ -z "$darwin_binary" || -z "$windows_binary" ]]; then
	echo "package-mcpb: binaries not found in $dist/artifacts.json, run goreleaser first" >&2
	exit 1
fi

rm -rf "$stage"
mkdir -p "$stage/server"
cp "$darwin_binary" "$stage/server/mcp-tiime"
cp "$windows_binary" "$stage/server/mcp-tiime.exe"
chmod +x "$stage/server/mcp-tiime"
cp README.md LICENSE "$stage/"
jq --arg version "$version" '.version = $version' mcpb/manifest.json > "$stage/manifest.json"

npx --yes "$mcpb_cli" validate "$stage/manifest.json"
npx --yes "$mcpb_cli" pack "$stage" "$dist/mcp-tiime.mcpb"

(cd "$dist" && sha256sum mcp-tiime.mcpb > mcp-tiime.mcpb.sha256)
