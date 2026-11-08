$ErrorActionPreference = "Stop"
$sdk = if ($env:ANDROID_HOME) { $env:ANDROID_HOME } else { Join-Path $env:LOCALAPPDATA "Android\Sdk" }
$platform = Get-ChildItem (Join-Path $sdk "platforms") -Directory | Sort-Object Name | Select-Object -Last 1
$out = Join-Path ([IO.Path]::GetTempPath()) "boreal-android-classes"
Remove-Item $out -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $out | Out-Null
$sources = Get-ChildItem $PSScriptRoot -Filter *.java | ForEach-Object { $_.FullName }
javac --release 8 -cp (Join-Path $platform.FullName "android.jar") -d $out @sources
jar cf (Join-Path $PSScriptRoot "boreal_android.jar") -C $out .
