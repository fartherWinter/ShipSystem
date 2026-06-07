# Retention and Capacity

ShipSystem stores training, demonstration, replay, and audit data only. Retention controls must not be used to manage real weapon-control, fire-control, electronic-warfare, radar-control, or device-command records.

## Runtime Retention

Retention still runs once at startup when a policy is configured. It can also run as a background task by setting:

```text
SHIP_SIM_RETENTION_INTERVAL=6h
```

Set `SHIP_SIM_RETENTION_INTERVAL=0s` to disable the background worker. The worker recalculates `SHIP_SIM_RETENTION_DAYS` on every pass, so `30` always means "older than 30 days from this retention run."

Retention policy inputs:

- `SHIP_SIM_RETENTION_DAYS`: delete events, contacts, track points, and snapshots older than this many days.
- `SHIP_SIM_MAX_TRACK_POINTS_PER_RUN`: keep only the newest track points per run.
- `SHIP_SIM_MAX_EVENTS_PER_RUN`: keep only the newest events per run.
- `SHIP_SIM_MAX_SNAPSHOTS_PER_RUN`: keep only the newest replay snapshots per run.

Local demo templates leave all retention values at `0`, which disables pruning. The production example uses:

```text
SHIP_SIM_RETENTION_DAYS=30
SHIP_SIM_RETENTION_INTERVAL=6h
SHIP_SIM_MAX_TRACK_POINTS_PER_RUN=250000
SHIP_SIM_MAX_EVENTS_PER_RUN=50000
SHIP_SIM_MAX_SNAPSHOTS_PER_RUN=250000
```

Tune these values for the number of tracks, snapshot rate, replay fidelity, and database budget of the deployment.

## Preview Before Delete

Retention pruning is destructive. Preview first:

```powershell
.\scripts\retention.ps1 -Days 30
.\scripts\retention.ps1 -MaxTrackPointsPerRun 250000 -MaxEventsPerRun 50000 -MaxSnapshotsPerRun 250000
```

Apply only after reviewing the preview:

```powershell
.\scripts\retention.ps1 -Days 30 -MaxTrackPointsPerRun 250000 -MaxEventsPerRun 50000 -MaxSnapshotsPerRun 250000 -Apply
```

For authenticated deployments, pass `-Token` for simple token mode or run the command through a trusted authenticated proxy.

## Capacity Smoke Script

Run a live storage growth smoke test against a local or test deployment:

```powershell
.\scripts\run-capacity-smoke.ps1 -DurationSeconds 600
```

Estimate only, without creating runs:

```powershell
.\scripts\run-capacity-smoke.ps1 -EstimateOnly
```

The script exercises 5, 20, and 100 simulated tracks at 10 Hz snapshot rate by default. It creates training-only runs, starts them, optionally submits abstract training actions, stops them, and reads each report's actual snapshot count.

## Growth Estimates

At `snapshot_hz=10`, one continuously running training run produces these row counts per day:

| Tracks | Snapshots/day | Track points/day | Raw contacts/day | Events/day with 30s actions |
| ---: | ---: | ---: | ---: | ---: |
| 5 | 864,000 | 4,320,000 | 4,320,000 | 2,882 |
| 20 | 864,000 | 17,280,000 | 17,280,000 | 2,882 |
| 100 | 864,000 | 86,400,000 | 86,400,000 | 2,882 |

Rough storage varies heavily with JSONB, geometry, index fill factor, and PostgreSQL page overhead. As a planning range before compression or partitioning:

- `track_points` and `contacts_raw`: often hundreds of bytes per row plus btree/GiST indexes.
- `sim_snapshots`: grows with track count because each frame stores track/contact JSON for replay.
- GiST geometry indexes can materially increase write amplification and disk use; monitor table and index size separately.

For high-rate 100-track exercises, full-fidelity 10 Hz replay can grow by tens of GB per day. Prefer short retention windows, lower `snapshot_hz`, or dedicated archival/export workflows before using long retention on shared infrastructure.

Recommended starting points:

- Demo/local: retention disabled; prune manually after experiments.
- Small team training, 5-20 tracks: 7-30 days plus per-run caps.
- High-density 100-track tests: 1-7 days, shorter per-run caps, and explicit database capacity monitoring.

Future hardening can add partitioning by time/run and offline archive exports. Until then, use preview plus backups before destructive maintenance.
