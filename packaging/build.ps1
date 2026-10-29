param(
    [string]$Version = "0.1.0-dev",
    [string[]]$Arch = @("amd64", "arm64"),
    [switch]$Installer
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$dist = Join-Path $root "dist"

Push-Location $root

try {
    Remove-Item $dist -Recurse -Force -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Force $dist | Out-Null

    $numeric = ($Version -split "-")[0] + ".0"

    Push-Location "cmd/boreal"
    go run github.com/tc-hib/go-winres@v0.3.3 make --arch ($Arch -join ",") --product-version $numeric --file-version $numeric
    Pop-Location

    if ($Installer) {
        $iscc = Get-Command iscc -ErrorAction SilentlyContinue

        if (-not $iscc) {
            choco install innosetup -y --no-progress | Out-Null
            $iscc = "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe"
        }
    }

    $ldflags = "-H windowsgui -s -w -X github.com/northlighthq/boreal/internal/version.Version=$Version"

    foreach ($a in $Arch) {
        $stage = Join-Path $dist "boreal-$a"
        New-Item -ItemType Directory -Force $stage | Out-Null

        $env:GOOS = "windows"
        $env:GOARCH = $a
        go build -trimpath -ldflags $ldflags -o (Join-Path $stage "boreal.exe") ./cmd/boreal
        $built = $LASTEXITCODE
        Remove-Item Env:GOOS, Env:GOARCH

        if ($built -ne 0) {
            throw "go build failed for $a"
        }

        Copy-Item LICENSE, README.md, THIRD_PARTY_NOTICES.md $stage
        Compress-Archive -Path (Join-Path $stage "*") -DestinationPath (Join-Path $dist "boreal-$Version-windows-$a.zip")

        $legacy = Join-Path $dist "legacy-$a"
        New-Item -ItemType Directory -Force $legacy | Out-Null
        Copy-Item (Join-Path $stage "*") $legacy
        Rename-Item (Join-Path $legacy "boreal.exe") "platty.exe"
        Compress-Archive -Path (Join-Path $legacy "*") -DestinationPath (Join-Path $dist "platty-$Version-windows-$a.zip")
        Remove-Item $legacy -Recurse -Force

        if ($Installer) {
            & $iscc "/DVersion=$Version" "/DArch=$a" "/DSourceDir=$stage" "/O$dist" "packaging/boreal.iss"

            if ($LASTEXITCODE -ne 0) {
                throw "Inno Setup failed for $a"
            }

            $setup = if ($a -eq "amd64") { "$Version-setup.exe" } else { "$Version-$a-setup.exe" }
            Copy-Item (Join-Path $dist "boreal-$setup") (Join-Path $dist "platty-$setup")
        }
    }

    Get-ChildItem $dist -File | Select-Object Name, Length
}
finally {
    Pop-Location
}
