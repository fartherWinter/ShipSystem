param(
    [string]$ApiBase = "http://localhost:8080",
    [int[]]$TrackCounts = @(5, 20, 100),
    [int]$DurationSeconds = 60,
    [int]$TickHz = 10,
    [int]$SnapshotHz = 10,
    [int]$ActionEverySeconds = 30,
    [string]$Token = "",
    [switch]$EstimateOnly
)

$ErrorActionPreference = "Stop"

$headers = @{}
if ($Token) {
    $headers.Authorization = "Bearer $Token"
}

function Invoke-ShipSimJson {
    param(
        [string]$Method,
        [string]$Path,
        [object]$Body = $null
    )
    $uri = "$ApiBase$Path"
    if ($null -eq $Body) {
        return Invoke-RestMethod -Method $Method -Uri $uri -Headers $headers
    }
    return Invoke-RestMethod -Method $Method -Uri $uri -Headers $headers -ContentType "application/json" -Body ($Body | ConvertTo-Json -Depth 20)
}

function New-CapacityScenario {
    param([int]$Tracks)
    return [ordered]@{
        name = "capacity-smoke-$Tracks-tracks"
        description = "Capacity smoke scenario for training simulation storage only."
        seed = 20260608 + $Tracks
        tick_hz = $TickHz
        snapshot_hz = $SnapshotHz
        initial_contacts = $Tracks
        ownship = @{ lon = 121.5; lat = 31.2; alt_m = 0 }
        allowed_actions = @("maneuver", "decoy", "training_response")
        sensors = @(
            @{ id = "sim-sensor-1"; name = "Simulated Search Sensor"; kind = "simulated_sensor"; position = @{ lon = 121.5; lat = 31.2; alt_m = 0 } }
        )
        zones = @(
            @{
                id = "training-area"
                name = "Training Area"
                kind = "exercise_boundary"
                polygon = @(
                    @{ lon = 121.1; lat = 30.9; alt_m = 0 },
                    @{ lon = 121.9; lat = 30.9; alt_m = 0 },
                    @{ lon = 121.9; lat = 31.6; alt_m = 0 },
                    @{ lon = 121.1; lat = 31.6; alt_m = 0 }
                )
            }
        )
    }
}

function Estimate-Growth {
    param([int]$Tracks)
    $secondsPerDay = 86400
    $snapshotRowsPerDay = $SnapshotHz * $secondsPerDay
    $trackPointRowsPerDay = $Tracks * $SnapshotHz * $secondsPerDay
    $contactRowsPerDay = $Tracks * $SnapshotHz * $secondsPerDay
    $eventRowsPerDay = 2
    if ($ActionEverySeconds -gt 0) {
        $eventRowsPerDay += [math]::Floor($secondsPerDay / $ActionEverySeconds)
    }
    return [ordered]@{
        tracks = $Tracks
        snapshot_hz = $SnapshotHz
        snapshots_per_day = $snapshotRowsPerDay
        track_points_per_day = $trackPointRowsPerDay
        contacts_per_day = $contactRowsPerDay
        events_per_day = $eventRowsPerDay
    }
}

$results = @()
foreach ($tracks in $TrackCounts) {
	$estimate = Estimate-Growth -Tracks $tracks
	if ($EstimateOnly) {
		$results += [pscustomobject]$estimate
		continue
	}

	$run = $null
	$stopped = $false
	try {
        $scenario = New-CapacityScenario -Tracks $tracks
        $run = Invoke-ShipSimJson -Method Post -Path "/api/runs" -Body @{ name = "capacity-smoke-$tracks"; scenario = $scenario }
        Invoke-ShipSimJson -Method Post -Path "/api/runs/$($run.id)/start" | Out-Null

        $deadline = (Get-Date).AddSeconds($DurationSeconds)
        while ((Get-Date) -lt $deadline) {
            $remaining = [math]::Max(0, [int]($deadline - (Get-Date)).TotalSeconds)
            if ($ActionEverySeconds -gt 0 -and $remaining -gt 0) {
                Start-Sleep -Seconds ([math]::Min($ActionEverySeconds, $remaining))
                if ((Get-Date) -lt $deadline) {
                    Invoke-ShipSimJson -Method Post -Path "/api/runs/$($run.id)/actions" -Body @{ type = "training_response" } | Out-Null
                }
            } else {
                Start-Sleep -Seconds $remaining
            }
        }

		Invoke-ShipSimJson -Method Post -Path "/api/runs/$($run.id)/stop" | Out-Null
		$stopped = $true
		$report = Invoke-ShipSimJson -Method Get -Path "/api/runs/$($run.id)/report"
        $row = [ordered]@{
            tracks = $tracks
            run_id = $run.id
            duration_seconds = $DurationSeconds
            observed_snapshots = $report.snapshot_range.count
            observed_events = $report.event_audit.event_count
            estimated_snapshots_per_day = $estimate.snapshots_per_day
            estimated_track_points_per_day = $estimate.track_points_per_day
            estimated_contacts_per_day = $estimate.contacts_per_day
            estimated_events_per_day = $estimate.events_per_day
        }
        $results += [pscustomobject]$row
	}
	finally {
		if ($run -and -not $stopped) {
			try {
				Invoke-ShipSimJson -Method Post -Path "/api/runs/$($run.id)/stop" | Out-Null
            } catch {
                Write-Warning "Failed to stop run $($run.id): $($_.Exception.Message)"
            }
        }
    }
}

$results | Format-Table -AutoSize
