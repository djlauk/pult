#!/usr/bin/env bash
# Prepares a release locally: sets brand.Version, regenerates the EXE
# resources and CHANGELOG.md, commits all of it as "chore(release): vX.Y.Z"
# and tags that commit. It does not push; pushing the tag starts the
# GitHub release workflow (.github/workflows/release.yml).
# Same steps as scripts/release.ps1; keep the two in sync.
#
# Usage: scripts/release.sh 1.2.0
# Needs: git, go, npm, git-cliff.

set -euo pipefail

die() { echo "release: $*" >&2; exit 1; }

[ $# -eq 1 ] || die "usage: $0 X.Y.Z"
version=${1#v}
[[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "version must look like 1.2.0 (got '$version')"
tag="v$version"

cd "$(git rev-parse --show-toplevel)"

[ "$(git branch --show-current)" = main ] || die "releases are made from main"
[ -z "$(git status --porcelain)" ] || die "working tree is not clean"
[ -z "$(git tag --list "$tag")" ] || die "tag $tag already exists"
command -v git-cliff >/dev/null || die "git-cliff not found (https://git-cliff.org/docs/installation)"

# Version: source, then the generated EXE resources.
brand=internal/brand/icon.go
sed -i.bak "s/^const Version = \".*\"\$/const Version = \"$version\"/" "$brand"
rm -f "$brand.bak"
grep -q "^const Version = \"$version\"\$" "$brand" || die "could not set the version in $brand"
go generate ./cmd/pult

# Make sure the release builds and passes before it gets a tag.
npm --prefix web run build
go test ./...

# The changelog files the unreleased commits under the new tag, which
# does not exist yet; that is what breaks the chicken-and-egg loop.
git cliff --tag "$tag" --output CHANGELOG.md

git add "$brand" cmd/pult/rsrc_windows_amd64.syso CHANGELOG.md
git commit -m "chore(release): $tag"
git tag -a "$tag" -m "pult $tag"

cat <<EOF

Tagged $tag. Review with: git show $tag
Publish with:             git push origin main $tag
Undo (before pushing):    git tag -d $tag && git reset --hard HEAD~1
EOF
