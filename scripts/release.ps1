# Prepares a release locally: sets brand.Version, regenerates the EXE
# resources and CHANGELOG.md, commits all of it as "chore(release): vX.Y.Z"
# and tags that commit. It does not push; pushing the tag starts the
# GitHub release workflow (.github/workflows/release.yml).
# Same steps as scripts/release.sh; keep the two in sync.
#
# Usage: pwsh scripts/release.ps1 1.2.0
# Needs: git, go, npm, git-cliff (winget install git-cliff).

param(
    [Parameter(Mandatory)][string]$Version
)

$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $true

$Version = $Version -replace '^v', ''
if ($Version -notmatch '^\d+\.\d+\.\d+$') {
    throw "Version must look like 1.2.0 (got '$Version')."
}
$tag = "v$Version"

$root = git rev-parse --show-toplevel
Push-Location $root
try {
    if ((git branch --show-current) -ne 'main') { throw 'Releases are made from main.' }
    if (git status --porcelain) { throw 'Working tree is not clean.' }
    if (git tag --list $tag) { throw "Tag $tag already exists." }
    if (-not (Get-Command git-cliff -ErrorAction SilentlyContinue)) {
        throw 'git-cliff not found (winget install git-cliff).'
    }

    # Version: source, then the generated EXE resources.
    $brand = 'internal/brand/icon.go'
    $src = Get-Content $brand -Raw
    $new = $src -replace 'const Version = "[^"]*"', "const Version = `"$Version`""
    if ($new -notmatch [regex]::Escape("const Version = `"$Version`"")) {
        throw "Could not set the version in $brand."
    }
    [IO.File]::WriteAllText((Join-Path $root $brand), $new)
    go generate ./cmd/pult

    # Make sure the release builds and passes before it gets a tag.
    npm --prefix web run build
    go test ./...

    # The changelog files the unreleased commits under the new tag, which
    # does not exist yet; that is what breaks the chicken-and-egg loop.
    git cliff --tag $tag --output CHANGELOG.md

    git add $brand cmd/pult/rsrc_windows_amd64.syso CHANGELOG.md
    git commit -m "chore(release): $tag"
    git tag -a $tag -m "pult $tag"

    Write-Host ""
    Write-Host "Tagged $tag. Review with: git show $tag"
    Write-Host "Publish with:             git push origin main $tag"
    Write-Host "Undo (before pushing):    git tag -d $tag; git reset --hard HEAD~1"
}
finally {
    Pop-Location
}
