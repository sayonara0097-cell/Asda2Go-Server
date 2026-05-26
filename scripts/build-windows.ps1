[CmdletBinding()]
param(
    [string]$OutputDir = "bin"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$bin = Join-Path $root $OutputDir

New-Item -ItemType Directory -Force -Path $bin | Out-Null

$targets = @(
    @{ Name = "loginserver.exe"; Package = "./cmd/loginserver" },
    @{ Name = "gameserver.exe"; Package = "./cmd/gameserver" },
    @{ Name = "gameserver-ch0.exe"; Package = "./cmd/gameserver" },
    @{ Name = "gameserver-ch1.exe"; Package = "./cmd/gameserver" },
    @{ Name = "gameserver-ch2.exe"; Package = "./cmd/gameserver" },
    @{ Name = "npcserver.exe"; Package = "./cmd/npcserver" },
    @{ Name = "npcserver-ch0.exe"; Package = "./cmd/npcserver" },
    @{ Name = "npcserver-ch1.exe"; Package = "./cmd/npcserver" },
    @{ Name = "npcserver-ch2.exe"; Package = "./cmd/npcserver" },
    @{ Name = "relayserver.exe"; Package = "./cmd/relayserver" },
    @{ Name = "gmtool.exe"; Package = "./cmd/gmtool" }
)

Push-Location $root
try {
    foreach ($target in $targets) {
        $out = Join-Path $bin $target.Name
        Write-Host "Building $($target.Name) from $($target.Package)"
        go build -trimpath -o $out $target.Package
    }
}
finally {
    Pop-Location
}
