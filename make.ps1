# make.ps1 - PowerShell equivalent of the Makefile for Windows users
# who don't have GNU make installed.
#
# Usage:
#   .\make.ps1 build       compile the sidecar
#   .\make.ps1 test        run unit tests with -race
#   .\make.ps1 run         build + start on :8181 against .\policies
#   .\make.ps1 demo        sidecar + planner-executor demo end-to-end
#   .\make.ps1 fmt         gofmt the tree
#   .\make.ps1 vet         go vet the tree
#   .\make.ps1 tidy        go mod tidy
#   .\make.ps1 clean       delete build artifacts
#   .\make.ps1 help        this message
#
# If Go isn't installed, use the Docker path instead (see README.md).

param(
    [Parameter(Position = 0)]
    [string]$Target = "help"
)

$ErrorActionPreference = "Stop"

$Binary = "bin\opa-sidecar.exe"
$Cmd    = ".\cmd\opa-sidecar"
$PolicyDir = ".\policies"

function Require-Go {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Host "Go is not installed. Install with:" -ForegroundColor Yellow
        Write-Host "  winget install GoLang.Go" -ForegroundColor Cyan
        Write-Host "Or use the Docker path instead (see README.md)" -ForegroundColor Yellow
        exit 1
    }
}

function Do-Build {
    Require-Go
    if (-not (Test-Path bin)) { New-Item -ItemType Directory -Path bin | Out-Null }
    go build -o $Binary $Cmd
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    Write-Host "built $Binary" -ForegroundColor Green
}

function Do-Test {
    Require-Go
    go test -race -count=1 ./...
    exit $LASTEXITCODE
}

function Do-Fmt {
    Require-Go
    gofmt -s -w .
}

function Do-Vet {
    Require-Go
    go vet ./...
    exit $LASTEXITCODE
}

function Do-Tidy {
    Require-Go
    go mod tidy
}

function Do-Run {
    Do-Build
    & $Binary --policy-dir $PolicyDir --listen ":8181"
}

function Do-Demo {
    Do-Build
    Write-Host "starting sidecar in background..." -ForegroundColor Cyan
    $sidecar = Start-Process -FilePath $Binary -ArgumentList "--policy-dir", $PolicyDir, "--listen", ":8181" -PassThru -NoNewWindow
    Start-Sleep -Seconds 1
    try {
        Push-Location examples\planner-executor
        try {
            Write-Host "building demo binaries..." -ForegroundColor Cyan
            go build -o "$env:TEMP\planner.exe"  .\planner
            if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
            go build -o "$env:TEMP\executor.exe" .\executor
            if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

            Write-Host "starting executor on :8081..." -ForegroundColor Cyan
            $exec = Start-Process -FilePath "$env:TEMP\executor.exe" -ArgumentList "--listen", ":8081", "--sidecar-url", "http://localhost:8181/authorize" -PassThru -NoNewWindow
            Start-Sleep -Seconds 1
            try {
                foreach ($sc in @("happy", "scope_creep", "tampered")) {
                    Write-Host ""
                    Write-Host "== SCENARIO: $sc ==" -ForegroundColor Cyan
                    & "$env:TEMP\planner.exe" --executor-url "http://localhost:8081/invoke" --scenario $sc
                }
            }
            finally {
                Stop-Process -Id $exec.Id -ErrorAction SilentlyContinue
            }
        }
        finally {
            Pop-Location
        }
    }
    finally {
        Stop-Process -Id $sidecar.Id -ErrorAction SilentlyContinue
        Write-Host "sidecar stopped" -ForegroundColor Green
    }
}

function Do-Clean {
    if (Test-Path bin) { Remove-Item -Recurse -Force bin }
    if (Test-Path coverage.txt) { Remove-Item coverage.txt }
    Write-Host "cleaned" -ForegroundColor Green
}

function Do-Help {
    Write-Host "make.ps1 - PowerShell equivalent of the Makefile"
    Write-Host ""
    Write-Host "Usage:"
    Write-Host "  .\make.ps1 build       compile the sidecar"
    Write-Host "  .\make.ps1 test        run unit tests with -race"
    Write-Host "  .\make.ps1 run         build + start on :8181 against .\policies"
    Write-Host "  .\make.ps1 demo        sidecar + planner-executor demo end-to-end"
    Write-Host "  .\make.ps1 fmt         gofmt the tree"
    Write-Host "  .\make.ps1 vet         go vet the tree"
    Write-Host "  .\make.ps1 tidy        go mod tidy"
    Write-Host "  .\make.ps1 clean       delete build artifacts"
    Write-Host "  .\make.ps1 help        this message"
}

switch ($Target.ToLower()) {
    "build" { Do-Build }
    "test"  { Do-Test }
    "fmt"   { Do-Fmt }
    "vet"   { Do-Vet }
    "tidy"  { Do-Tidy }
    "run"   { Do-Run }
    "demo"  { Do-Demo }
    "clean" { Do-Clean }
    "help"  { Do-Help }
    default {
        Write-Host "unknown target: $Target" -ForegroundColor Red
        Do-Help
        exit 1
    }
}
