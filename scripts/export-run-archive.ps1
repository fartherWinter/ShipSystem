param(
    [Parameter(Mandatory = $true)]
    [string]$RunID,
    [string]$ApiBase = "http://localhost:8080",
    [string]$OutputRoot = "outputs",
    [string]$Token = "",
    [switch]$SkipSnapshots,
    [switch]$SkipTrackPoints,
    [switch]$Compress,
    [switch]$RemoveUncompressed,
    [string]$UploadUrl = "",
    [string[]]$UploadHeader = @()
)

$ErrorActionPreference = "Stop"

$headers = @{}
if ($Token) {
    $headers.Authorization = "Bearer $Token"
}

function Join-ApiPath {
    param([string]$Path)
    return "$($ApiBase.TrimEnd('/'))$Path"
}

function Invoke-ShipSimJson {
    param(
        [string]$Path,
        [string]$Method = "Get"
    )
    return Invoke-RestMethod -Method $Method -Uri (Join-ApiPath -Path $Path) -Headers $headers
}

function Save-JsonFile {
    param(
        [string]$Path,
        [object]$Value
    )
    $Value | ConvertTo-Json -Depth 80 | Set-Content -Path $Path -Encoding UTF8
}

function Save-Download {
    param(
        [string]$Path,
        [string]$OutFile
    )
    Invoke-WebRequest -Method Get -Uri (Join-ApiPath -Path $Path) -Headers $headers -OutFile $OutFile | Out-Null
}

function Escape-QueryValue {
    param([string]$Value)
    return [System.Uri]::EscapeDataString($Value)
}

function Format-ApiTime {
    param([object]$Value)
    $time = [DateTimeOffset]::Parse([string]$Value)
    return $time.UtcDateTime.ToString("yyyy-MM-ddTHH:mm:ss.fffZ")
}

function Add-Milliseconds {
    param(
        [object]$Value,
        [int]$Milliseconds
    )
    $time = [DateTimeOffset]::Parse([string]$Value)
    return $time.AddMilliseconds($Milliseconds).UtcDateTime.ToString("yyyy-MM-ddTHH:mm:ss.fffZ")
}

function Safe-Name {
    param([string]$Value)
    return ($Value -replace '[^A-Za-z0-9._-]', '_')
}

function Count-Items {
    param([object]$Value)
    if ($null -eq $Value) {
        return 0
    }
    return @($Value).Count
}

function Convert-UploadHeaders {
    param([string[]]$Header)
    $out = @{}
    foreach ($item in $Header) {
        if (-not $item) {
            continue
        }
        $parts = $item -split ":", 2
        if ($parts.Count -ne 2 -or -not $parts[0].Trim()) {
            throw "UploadHeader entries must use 'Name: Value' format."
        }
        $out[$parts[0].Trim()] = $parts[1].Trim()
    }
    return $out
}

function Export-Events {
    param([string]$DataDir)
    $events = @()
    $cursor = ""
    do {
        $path = "/api/runs/$RunID/events?limit=200"
        if ($cursor) {
            $path = "$path&cursor=$(Escape-QueryValue -Value $cursor)"
        }
        $page = Invoke-ShipSimJson -Path $path
        if ($page.items) {
            $events += @($page.items)
        }
        $cursor = [string]$page.next_cursor
    } while ($cursor)
    Save-JsonFile -Path (Join-Path $DataDir "events.json") -Value $events
    return $events.Count
}

function Export-Snapshots {
    param(
        [string]$DataDir,
        [object]$Report
    )
    if (-not $Report.snapshot_range) {
        return 0
    }
    $snapshotDir = Join-Path $DataDir "snapshots"
    New-Item -ItemType Directory -Force -Path $snapshotDir | Out-Null
    $from = Format-ApiTime -Value $Report.snapshot_range.from
    $to = Format-ApiTime -Value $Report.snapshot_range.to
    $seen = @{}
    $pageNumber = 1
    $total = 0
    while ($true) {
        $path = "/api/runs/$RunID/snapshots?limit=1000&from=$(Escape-QueryValue -Value $from)&to=$(Escape-QueryValue -Value $to)"
        $frames = @(Invoke-ShipSimJson -Path $path)
        if ($frames.Count -eq 0) {
            break
        }
        $unique = @()
        foreach ($frame in $frames) {
            $key = "$($frame.sampled_at)|$($frame.tick)"
            if (-not $seen.ContainsKey($key)) {
                $seen[$key] = $true
                $unique += $frame
            }
        }
        if ($unique.Count -gt 0) {
            $fileName = "snapshots-page-{0:D4}.json" -f $pageNumber
            Save-JsonFile -Path (Join-Path $snapshotDir $fileName) -Value $unique
            $pageNumber += 1
            $total += $unique.Count
        }
        if ($frames.Count -lt 1000) {
            break
        }
        $nextFrom = Add-Milliseconds -Value $frames[$frames.Count - 1].sampled_at -Milliseconds 1
        if ($nextFrom -eq $from) {
            break
        }
        $from = $nextFrom
    }
    return $total
}

function Export-TrackPoints {
    param(
        [string]$DataDir,
        [object[]]$Tracks,
        [object]$Report
    )
    if (-not $Report.snapshot_range) {
        return 0
    }
    $pointDir = Join-Path $DataDir "track-points"
    New-Item -ItemType Directory -Force -Path $pointDir | Out-Null
    $total = 0
    foreach ($track in $Tracks) {
        $trackID = [string]$track.id
        if (-not $trackID) {
            continue
        }
        $from = Format-ApiTime -Value $Report.snapshot_range.from
        $to = Format-ApiTime -Value $Report.snapshot_range.to
        $seen = @{}
        $points = @()
        while ($true) {
            $path = "/api/runs/$RunID/track-points?limit=1000&track_id=$(Escape-QueryValue -Value $trackID)&from=$(Escape-QueryValue -Value $from)&to=$(Escape-QueryValue -Value $to)"
            $page = @(Invoke-ShipSimJson -Path $path)
            if ($page.Count -eq 0) {
                break
            }
            foreach ($point in $page) {
                $key = "$($point.sampled_at)|$($point.track_id)"
                if (-not $seen.ContainsKey($key)) {
                    $seen[$key] = $true
                    $points += $point
                }
            }
            if ($page.Count -lt 1000) {
                break
            }
            $from = Add-Milliseconds -Value $page[$page.Count - 1].sampled_at -Milliseconds 1
        }
        Save-JsonFile -Path (Join-Path $pointDir "$(Safe-Name -Value $trackID).json") -Value $points
        $total += $points.Count
    }
    return $total
}

$safeRunID = Safe-Name -Value $RunID
$archiveDir = Join-Path $OutputRoot "run-$safeRunID-archive"
$archiveZip = Join-Path $OutputRoot "run-$safeRunID-archive.zip"
$reportDir = Join-Path $archiveDir "reports"
$dataDir = Join-Path $archiveDir "data"
New-Item -ItemType Directory -Force -Path $reportDir, $dataDir | Out-Null

if ($UploadUrl -and -not $Compress) {
    throw "-UploadUrl requires -Compress so the object-store handoff is a single zip bundle."
}

Write-Host "Exporting training archive for run $RunID to $archiveDir"

$run = Invoke-ShipSimJson -Path "/api/runs/$RunID"
$report = Invoke-ShipSimJson -Path "/api/runs/$RunID/report"
Save-JsonFile -Path (Join-Path $reportDir "report.json") -Value $report
Save-Download -Path "/api/runs/$RunID/report?format=csv" -OutFile (Join-Path $reportDir "report.csv")
Save-Download -Path "/api/runs/$RunID/report?format=html" -OutFile (Join-Path $reportDir "report.html")
Save-Download -Path "/api/runs/$RunID/report?format=pdf" -OutFile (Join-Path $reportDir "report.pdf")

$eventsCount = Export-Events -DataDir $dataDir
$annotations = @(Invoke-ShipSimJson -Path "/api/runs/$RunID/annotations")
$auditLogs = @(Invoke-ShipSimJson -Path "/api/runs/$RunID/audit?limit=200")
$tracks = @(Invoke-ShipSimJson -Path "/api/runs/$RunID/tracks")
$zones = @(Invoke-ShipSimJson -Path "/api/runs/$RunID/zones")

Save-JsonFile -Path (Join-Path $dataDir "run.json") -Value $run
Save-JsonFile -Path (Join-Path $dataDir "annotations.json") -Value $annotations
Save-JsonFile -Path (Join-Path $dataDir "audit.json") -Value $auditLogs
Save-JsonFile -Path (Join-Path $dataDir "tracks.json") -Value $tracks
Save-JsonFile -Path (Join-Path $dataDir "zones.json") -Value $zones

$snapshotCount = 0
if (-not $SkipSnapshots) {
    $snapshotCount = Export-Snapshots -DataDir $dataDir -Report $report
}

$trackPointCount = 0
if (-not $SkipTrackPoints) {
    $trackPointCount = Export-TrackPoints -DataDir $dataDir -Tracks $tracks -Report $report
}

$compressionMode = "none"
$archiveFile = $null
if ($Compress) {
    $compressionMode = "zip"
    $archiveFile = Split-Path -Leaf $archiveZip
}

$manifest = [ordered]@{
    generated_at = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ss.fffZ")
    api_base = $ApiBase
    run_id = $RunID
    training_only = $true
    safety_notice = $report.safety_notice
    report_version = $report.version
    replay_mode = $report.replay_mode
    snapshot_range = $report.snapshot_range
    compression = $compressionMode
    archive_file = $archiveFile
    contents = [ordered]@{
        reports = @("report.json", "report.csv", "report.html", "report.pdf")
        events = $eventsCount
        annotations = Count-Items -Value $annotations
        audit_logs = Count-Items -Value $auditLogs
        tracks = Count-Items -Value $tracks
        zones = Count-Items -Value $zones
        snapshots = $snapshotCount
        track_points = $trackPointCount
    }
}
Save-JsonFile -Path (Join-Path $archiveDir "manifest.json") -Value $manifest

if ($Compress) {
    if (Test-Path $archiveZip) {
        Remove-Item -LiteralPath $archiveZip -Force
    }
    Compress-Archive -Path (Join-Path $archiveDir "*") -DestinationPath $archiveZip -CompressionLevel Optimal
    if ($RemoveUncompressed) {
        Remove-Item -LiteralPath $archiveDir -Recurse -Force
    }
}

if ($UploadUrl) {
    $uploadHeaders = Convert-UploadHeaders -Header $UploadHeader
    Write-Host "Uploading compressed training archive to object storage"
    Invoke-WebRequest -Method Put -Uri $UploadUrl -Headers $uploadHeaders -InFile $archiveZip -ContentType "application/zip" | Out-Null
    $manifest["upload"] = [ordered]@{
        method = "presigned_put"
        uploaded_at = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ss.fffZ")
    }
}

$manifest | ConvertTo-Json -Depth 20
