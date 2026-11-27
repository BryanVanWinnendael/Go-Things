# Builds every Go Things app. The Windows setup stays in <app>/build alongside
# the executable; attach it to a GitHub release from there. Build directories
# are ignored by Git, but are left in place so the installers are easy to find.

param(
  [string]$Platform = "windows/amd64"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$apps = @("go-hub", "go-post", "go-mail")

foreach ($app in $apps) {
  $dir = Join-Path $root $app
  $build = Join-Path $dir "build"

  # Never let a previous setup become a resource of the next setup.
  Remove-Item $build -Recurse -Force -ErrorAction SilentlyContinue

  Push-Location $dir
  try {
    go run github.com/egoist/mygo/cmd/mygo build -platform $Platform
  }
  finally {
    Pop-Location
  }

  $target = if ($Platform -eq "windows/amd64") { "windows-amd64" } else { $Platform.Replace("/", "-") }
  $installer = Get-ChildItem (Join-Path $build $target) -Filter "*-setup-*.exe" | Select-Object -First 1
  if (-not $installer) {
    throw "No installer was produced for $app"
  }

  "Installer: $($installer.FullName)"
}
