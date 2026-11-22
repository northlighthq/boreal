param(
    [string]$Version = "0.1.0-dev",
    [int]$VersionCode = 1,
    [string[]]$Arch = @("arm64", "amd64"),
    [int]$MinSdk = 28,
    [int]$TargetSdk = 36,
    [string]$AppId = "com.northlighthq.boreal",
    [string]$SignKey = "",
    [string]$SignPass = ""
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$dist = Join-Path $root "dist"

if (-not $env:ANDROID_HOME) {
    $env:ANDROID_HOME = Join-Path (Join-Path $env:LOCALAPPDATA "Android") "Sdk"
}

if (-not (Test-Path $env:ANDROID_HOME)) {
    throw "Android SDK not found at $env:ANDROID_HOME. Set ANDROID_HOME."
}

if (-not $env:ANDROID_NDK_ROOT) {
    $ndk = Get-ChildItem (Join-Path $env:ANDROID_HOME "ndk") -Directory -ErrorAction SilentlyContinue | Sort-Object Name | Select-Object -Last 1

    if (-not $ndk) {
        throw "Android NDK not found. Install it with sdkmanager or set ANDROID_NDK_ROOT."
    }

    $env:ANDROID_NDK_ROOT = $ndk.FullName
}

if (-not (Get-Command javac -ErrorAction SilentlyContinue)) {
    $userHome = [Environment]::GetFolderPath("UserProfile")
    $candidates = @()

    if ($env:JAVA_HOME) {
        $candidates += $env:JAVA_HOME
    }

    $candidates += Get-ChildItem (Join-Path $userHome ".jdks") -Directory -ErrorAction SilentlyContinue | Sort-Object Name -Descending | ForEach-Object { $_.FullName }
    $candidates += @("C:\Program Files\Android\Android Studio\jbr", "/Applications/Android Studio.app/Contents/jbr/Contents/Home")
    $jdk = $candidates | Where-Object { Test-Path (Join-Path (Join-Path $_ "bin") "javac*") } | Select-Object -First 1

    if (-not $jdk) {
        throw "No JDK found. Install JDK 17 or newer and set JAVA_HOME."
    }

    $env:JAVA_HOME = $jdk
    $env:PATH = (Join-Path $jdk "bin") + [IO.Path]::PathSeparator + $env:PATH
}

if (-not (Get-Command gogio -ErrorAction SilentlyContinue)) {
    go install gioui.org/cmd/gogio@v0.10.3
}

$numeric = ($Version -split "-")[0]
$out = Join-Path $dist "boreal-$Version-android.apk"
New-Item -ItemType Directory -Force $dist | Out-Null

$gogioArgs = @(
    "-target", "android",
    "-arch", ($Arch -join ","),
    "-appid", $AppId,
    "-name", "Boreal",
    "-version", "$numeric.$VersionCode",
    "-minsdk", $MinSdk,
    "-targetsdk", $TargetSdk,
    "-icon", (Join-Path (Join-Path $root "packaging") "boreal.png"),
    "-ldflags", "-s -w -X github.com/northlighthq/boreal/internal/core/version.Version=$Version",
    "-o", $out
)

if (-not $SignKey) {
    $SignKey = Join-Path (Join-Path ([Environment]::GetFolderPath("UserProfile")) ".android") "debug.keystore"
    $SignPass = "android"

    if (-not (Test-Path $SignKey)) {
        New-Item -ItemType Directory -Force (Split-Path $SignKey) | Out-Null
        $ErrorActionPreference = "Continue"
        keytool -genkeypair -keystore $SignKey -storepass $SignPass -keypass $SignPass -alias androiddebugkey -keyalg RSA -keysize 2048 -validity 10000 -dname "CN=Android Debug,O=Android,C=US" 2>&1 | Out-Null
        $ErrorActionPreference = "Stop"

        if ($LASTEXITCODE -ne 0) {
            throw "keytool failed"
        }
    }
}

$gogioArgs += @("-signkey", $SignKey, "-signpass", $SignPass)

Push-Location $root

try {
    gogio @gogioArgs ./cmd/boreal

    if ($LASTEXITCODE -ne 0) {
        throw "gogio failed"
    }
}
finally {
    Pop-Location
}

Write-Host $out
