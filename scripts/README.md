# Research scripts

The `.mjs` research scripts are UTF-8 encoded and contain Chinese workbook labels.
On Windows PowerShell 5, `Get-Content` may display UTF-8 text as mojibake when the
console code page is not UTF-8. Use Node or an editor that honors `.editorconfig`
when inspecting or editing these files.

Rebuild the GitHub statistics workbook:

```powershell
node scripts/build_ship_system_github_stats.mjs
node scripts/add_function_analysis.mjs
```

Set `REFRESH_GITHUB=1` before the first command to refresh the GitHub API cache.

## Operations scripts

Preview or apply retention policies:

```powershell
.\scripts\retention.ps1 -Days 30
.\scripts\retention.ps1 -Days 30 -Apply
```

Estimate or smoke-test storage growth:

```powershell
.\scripts\run-capacity-smoke.ps1 -EstimateOnly
.\scripts\run-capacity-smoke.ps1 -DurationSeconds 600
```

Export a completed training run archive before pruning or cold-storage handoff:

```powershell
.\scripts\export-run-archive.ps1 -RunID <run-id>
.\scripts\export-run-archive.ps1 -RunID <run-id> -Compress
.\scripts\export-run-archive.ps1 -RunID <run-id> -Compress -UploadUrl $env:SHIP_SIM_ARCHIVE_UPLOAD_URL
```

The archive script is read-only. It writes report exports, events, annotations,
audit logs, tracks, zones, snapshot pages, track-point pages, and a manifest to
`outputs/run-<run-id>-archive/`. Use `-Compress` to create
`outputs/run-<run-id>-archive.zip`; add `-RemoveUncompressed` only when the zip
is the desired cold-storage handoff artifact. `-UploadUrl` uploads the zip with
a pre-signed object-store `PUT`; pass repeated `-UploadHeader "Name: Value"`
entries when the storage provider requires additional headers.

Restore replay evidence from a compressed archive into PostgreSQL/PostGIS:

```powershell
go run ./cmd/archive-restore -archive outputs/run-<run-id>-archive.zip -database-url $env:DATABASE_URL
```

The restore command recreates the training run, events, snapshot replay frames,
derived track points, annotations, and audit entries. It refuses to append into
an existing run with replay data unless `-allow-append` is set.
