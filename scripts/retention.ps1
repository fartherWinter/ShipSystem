param(
    [string]$ApiBase = "http://localhost:8080",
    [int]$Days = 0,
    [string]$Cutoff = "",
    [string]$EndedBefore = "",
    [int]$MaxTrackPointsPerRun = 0,
    [int]$MaxEventsPerRun = 0,
    [int]$MaxSnapshotsPerRun = 0,
    [string]$Token = "",
    [switch]$Apply
)

$ErrorActionPreference = "Stop"

$policy = [ordered]@{}
if ($Days -gt 0) { $policy.days = $Days }
if ($Cutoff) { $policy.cutoff = $Cutoff }
if ($EndedBefore) { $policy.ended_before = $EndedBefore }
if ($MaxTrackPointsPerRun -gt 0) { $policy.max_track_points_per_run = $MaxTrackPointsPerRun }
if ($MaxEventsPerRun -gt 0) { $policy.max_events_per_run = $MaxEventsPerRun }
if ($MaxSnapshotsPerRun -gt 0) { $policy.max_snapshots_per_run = $MaxSnapshotsPerRun }

if ($policy.Count -eq 0) {
    throw "Provide at least one retention selector: -Days, -Cutoff, -EndedBefore, -MaxTrackPointsPerRun, -MaxEventsPerRun, or -MaxSnapshotsPerRun."
}

$headers = @{}
if ($Token) {
    $headers.Authorization = "Bearer $Token"
}

$parts = @()
foreach ($key in $policy.Keys) {
    $escapedKey = [System.Uri]::EscapeDataString($key)
    $escapedValue = [System.Uri]::EscapeDataString([string]$policy[$key])
    $parts += "$escapedKey=$escapedValue"
}

$previewUrl = "$ApiBase/api/retention/preview?$($parts -join "&")"
Write-Host "Retention preview:"
$preview = Invoke-RestMethod -Method Get -Uri $previewUrl -Headers $headers
$preview | ConvertTo-Json -Depth 5

if (-not $Apply) {
    Write-Host "Dry run only. Re-run with -Apply to prune the previewed policy."
    return
}

Write-Host "Applying retention prune after preview."
$body = $policy | ConvertTo-Json -Depth 5
$result = Invoke-RestMethod -Method Post -Uri "$ApiBase/api/retention/prune" -Headers $headers -ContentType "application/json" -Body $body
$result | ConvertTo-Json -Depth 5
