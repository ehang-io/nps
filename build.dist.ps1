# build.dist.ps1 - full cross-build + repo-style packaging
#
# Output layout (same naming scheme as build.sh in this repo):
#   dist/<os>_<arch>[_vN]_server.tar.gz
#   dist/<os>_<arch>[_vN]_client.tar.gz
#   dist/BUILD-INFO.txt, dist/SHA256SUMS.txt
#
# Server archive : nps[.exe] conf/{nps.conf,tasks.json,clients.json,hosts.json,server.key,server.pem} web/views web/static
# Client archive : npc[.exe] conf/npc.conf conf/multi_account.conf
#
# Usage:
#   pwsh -File build.dist.ps1
#   pwsh -File build.dist.ps1 -Version 0.26.12
#   pwsh -File build.dist.ps1 -Only linux/amd64,windows/amd64 -ServerOnly
#   pwsh -File build.dist.ps1 -NoArchive        # binaries + staged trees only

[CmdletBinding()]
param(
    [string]$OutDir = 'dist',
    [string]$Version = '0.26.12',
    [string[]]$Only = @(),
    [switch]$ServerOnly,
    [switch]$ClientOnly,
    [switch]$NoArchive,
    [switch]$KeepStage
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $root

$goExe = (Get-Command go -ErrorAction SilentlyContinue)
if (-not $goExe) { throw 'go toolchain not found in PATH' }

$ldflags = '-s -w -extldflags -static'

# ---------------------------------------------------------------- target table
$allTargets = @(
    @{ GoOs = 'linux';   GoArch = 'amd64' },
    @{ GoOs = 'linux';   GoArch = '386' },
    @{ GoOs = 'linux';   GoArch = 'arm'; GoArm = '7' },
    @{ GoOs = 'linux';   GoArch = 'arm'; GoArm = '6' },
    @{ GoOs = 'linux';   GoArch = 'arm'; GoArm = '5' },
    @{ GoOs = 'linux';   GoArch = 'arm64' },
    @{ GoOs = 'linux';   GoArch = 'mips' },
    @{ GoOs = 'linux';   GoArch = 'mipsle' },
    @{ GoOs = 'linux';   GoArch = 'mips64' },
    @{ GoOs = 'linux';   GoArch = 'mips64le' },
    @{ GoOs = 'freebsd'; GoArch = '386' },
    @{ GoOs = 'freebsd'; GoArch = 'amd64' },
    @{ GoOs = 'freebsd'; GoArch = 'arm' },
    @{ GoOs = 'darwin';  GoArch = 'amd64' },
    @{ GoOs = 'darwin';  GoArch = 'arm64' },
    @{ GoOs = 'windows'; GoArch = '386' },
    @{ GoOs = 'windows'; GoArch = 'amd64' }
)

function Get-TargetName([hashtable]$t) {
    $n = "$($t.GoOs)_$($t.GoArch)"
    if ($t.GoArm) { $n = "${n}_v$($t.GoArm)" }
    return $n
}

function Get-TargetKey([hashtable]$t) {
    return "$($t.GoOs)/$($t.GoArch)"
}

# ------------------------------------------------------------------ workspaces
$outFull = Join-Path $root $OutDir
$stageRoot = Join-Path $outFull '.stage'
if (Test-Path $stageRoot) { Remove-Item $stageRoot -Recurse -Force }
New-Item -ItemType Directory -Path $outFull -Force | Out-Null
New-Item -ItemType Directory -Path $stageRoot -Force | Out-Null

# accept "-Only linux/amd64 -Only windows/amd64" or "-Only linux/amd64,windows/amd64"
$Only = @($Only | ForEach-Object { $_ -split '[,;\s]+' } | Where-Object { $_ })

$selected = $allTargets
if ($Only.Count -gt 0) {
    $sel = @()
    foreach ($o in $Only) {
        $hit = $allTargets | Where-Object { (Get-TargetKey $_) -eq $o }
        if (-not $hit) { throw "unknown target '$o' (expect os/arch, e.g. linux/amd64)" }
        $sel += $hit
    }
    $selected = $sel
}

$roles = @()
if (-not $ClientOnly) { $roles += 'server' }
if (-not $ServerOnly) { $roles += 'client' }

$tarExe = (Get-Command tar -ErrorAction SilentlyContinue)
if (-not $NoArchive -and -not $tarExe) { throw 'tar not found; use -NoArchive to skip packaging' }

# strip stale binaries left in the repo root so packaging never picks them up
foreach ($stale in 'nps', 'nps.exe', 'npc', 'npc.exe') {
    if (Test-Path $stale) { Remove-Item $stale -Force }
}

$results = @()
$failures = @()
$started = Get-Date

foreach ($t in $selected) {
    foreach ($role in $roles) {
        $tname = Get-TargetName $t
        $pkgName = "${tname}_$role"
        $stage = Join-Path $stageRoot $pkgName
        $archive = Join-Path $outFull "$pkgName.tar.gz"
        if (Test-Path $archive) { Remove-Item $archive -Force }

        $isServer = $role -eq 'server'
        $exeSuffix = ''
        if ($t.GoOs -eq 'windows') { $exeSuffix = '.exe' }
        $binName = if ($isServer) { "nps$exeSuffix" } else { "npc$exeSuffix" }
        $pkgPath = if ($isServer) { './cmd/nps' } else { './cmd/npc' }

        Write-Host ("[{0}] building {1}" -f (Get-Date -Format 'HH:mm:ss'), $pkgName) -ForegroundColor Cyan

        $env:GOOS = $t.GoOs
        $env:GOARCH = $t.GoArch
        $env:CGO_ENABLED = '0'
        if ($t.GoArm) { $env:GOARM = $t.GoArm } else { Remove-Item Env:GOARM -ErrorAction SilentlyContinue }

        New-Item -ItemType Directory -Path $stage -Force | Out-Null

        try {
            $buildOut = & go build -trimpath -ldflags $ldflags -o (Join-Path $stage $binName) $pkgPath 2>&1
            if ($LASTEXITCODE -ne 0) { throw ($buildOut -join "`n") }

            if ($isServer) {
                $confDst = Join-Path $stage 'conf'
                New-Item -ItemType Directory -Path $confDst -Force | Out-Null
                foreach ($f in 'nps.conf', 'tasks.json', 'clients.json', 'hosts.json', 'server.key', 'server.pem') {
                    $src = Join-Path (Join-Path $root 'conf') $f
                    if (Test-Path $src) { Copy-Item $src (Join-Path $confDst $f) -Force }
                }
                foreach ($d in 'views', 'static') {
                    $src = Join-Path (Join-Path $root 'web') $d
                    if (Test-Path $src) { Copy-Item $src (Join-Path $stage "web/$d") -Recurse -Force }
                }
                # ship the server management scripts (cert / tls switch / start-stop) with the server package
                if (Test-Path -LiteralPath (Join-Path $root 'scripts')) {
                    Copy-Item -LiteralPath (Join-Path $root 'scripts') -Destination (Join-Path $stage 'scripts') -Recurse -Force
                }
            }
            else {
                $confDst = Join-Path $stage 'conf'
                New-Item -ItemType Directory -Path $confDst -Force | Out-Null
                foreach ($f in 'npc.conf', 'multi_account.conf') {
                    $src = Join-Path (Join-Path $root 'conf') $f
                    if (Test-Path $src) { Copy-Item $src (Join-Path $confDst $f) -Force }
                }
            }

            if (-not $NoArchive) {
                $members = @($binName)
                if ($isServer) { $members += 'conf', 'web', 'scripts' } else { $members += 'conf' }
                & tar -czf $archive -C $stage @members
                if ($LASTEXITCODE -ne 0) { throw "tar failed for $pkgName" }
                $size = (Get-Item $archive).Length
                $results += [pscustomobject]@{ Package = $pkgName; Archive = "$pkgName.tar.gz"; Bytes = $size; Status = 'ok' }
                Write-Host ("       -> {0} ({1:N0} KB)" -f "$pkgName.tar.gz", ($size / 1KB)) -ForegroundColor DarkGray
            }
            else {
                $results += [pscustomobject]@{ Package = $pkgName; Archive = '(staged)'; Bytes = 0; Status = 'ok' }
            }
        }
        catch {
            # keep every line of the compiler output: multi-line errors got truncated in logs
            $msg = (@($_.Exception.Message -split "`r?`n" | Where-Object { $_ -ne '' }) -join ' | ')
            # position helps when the same message can come from several Copy-Item calls
            $pos = $_.InvocationInfo
            if ($pos -and $pos.ScriptLineNumber) {
                $msg = "$msg [line $($pos.ScriptLineNumber): $($pos.Line.Trim())]"
            }
            $failures += [pscustomobject]@{ Package = $pkgName; Error = $msg }
            $results += [pscustomobject]@{ Package = $pkgName; Archive = '-'; Bytes = 0; Status = 'failed' }
            Write-Host ("       !! {0}: {1}" -f $pkgName, $msg) -ForegroundColor Red
        }
    }
}

# ------------------------------------------------------------------ checksums
$env:CGO_ENABLED = ''
Remove-Item Env:GOOS, Env:GOARCH, Env:GOARM -ErrorAction SilentlyContinue

if (-not $NoArchive) {
    $archives = Get-ChildItem $outFull -Filter '*.tar.gz' | Sort-Object Name
    $lines = foreach ($a in $archives) {
        $h = (Get-FileHash $a.FullName -Algorithm SHA256).Hash.ToLower()
        "$h  $($a.Name)"
    }
    Set-Content -Path (Join-Path $outFull 'SHA256SUMS.txt') -Value $lines -Encoding ASCII
}

# ship the usage doc next to the archives
$docSrc = Join-Path $root 'docs/usage-zh.md'
if (Test-Path $docSrc) { Copy-Item $docSrc (Join-Path $outFull 'USAGE-zh.md') -Force }

# staged trees are intermediate output: drop them once everything is packaged
if (-not $NoArchive -and -not $KeepStage) {
    Remove-Item $stageRoot -Recurse -Force -ErrorAction SilentlyContinue
}

$goVer = (& go version) -join ' '
$elapsed = (Get-Date) - $started
$info = @()
$info += "nps/npc build bundle"
$info += "version    : $Version"
$info += "built at   : $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss zzz')"
$info += "toolchain  : $goVer"
$info += "ldflags    : $ldflags"
$info += "targets    : $($selected.Count) x $($roles.Count) role(s) = $($results.Count) package(s)"
$info += "duplicates : nps/npc built with the same source tree, compiled per platform"
$info += "names      : <os>_<arch>[_vN]_<server|client>.tar.gz  (same scheme as build.sh)"
$info += "elapsed    : $([int]$elapsed.TotalSeconds)s"
$info += ""
$info += "PACKAGES"
foreach ($r in ($results | Sort-Object Package)) {
    $info += ("  {0,-30} {1,-34} {2,10} KB  {3}" -f $r.Package, $r.Archive, [int]($r.Bytes / 1KB), $r.Status)
}
if ($failures.Count -gt 0) {
    $info += ""
    $info += "FAILURES"
    foreach ($f in $failures) { $info += ("  {0}: {1}" -f $f.Package, $f.Error) }
}
$info += ""
$info += "NOTES"
$info += "  * npc_sdk (cgo shared library) is not built here: needs gcc/mingw + 'go build -tags sdk -buildmode=c-shared'."
$info += "  * server archives are drop-in deployable: conf + web + nps, unzip and run ./nps install"
$info += "  * USAGE-zh.md sits next to the archives: deployment, multi-server tunneling, tunnel IP allowlist."
$info += "  * .stage/ (unpacked per-platform trees) is removed after packaging; pass -KeepStage to keep it."
$info += "  * server archives also carry scripts/: nps-server.sh (cert, tls switch, start-stop) and npc-client.sh (-server, tls, basic)."
Set-Content -Path (Join-Path $outFull 'BUILD-INFO.txt') -Value $info -Encoding UTF8

Write-Host ''
Write-Host ("done: {0} ok, {1} failed, {2}s -> {3}" -f @($results | Where-Object Status -eq 'ok').Count, $failures.Count, [int]$elapsed.TotalSeconds, $outFull) -ForegroundColor Green
if ($failures.Count -gt 0) { exit 1 }
