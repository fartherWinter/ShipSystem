# Release and CI/CD

ShipSystem release artifacts are for training, demonstration, replay, and audit deployments only. Do not publish or document any release as a real weapon-control, fire-control, electronic-warfare, radar control, or tactical recommendation system.

## CI Gate

The GitHub Actions workflow in `.github/workflows/ci.yml` runs on pull requests, pushes to `main`, pushes to `codex/**`, and `v*` tags.

The gate runs:

- `go test ./...`
- `go vet ./...`
- `go build ./cmd/sim-server`
- `govulncheck` through `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`
- `npm ci`
- `npm run generate:types`
- `npm test`
- `npm run build`
- `npm audit --audit-level=high --registry=https://registry.npmjs.org`
- `docker build`
- SBOM generation as an uploaded SPDX JSON artifact
- Trivy image scan for high and critical vulnerabilities

Use the public npm registry or an internal registry that supports the npm security audit endpoint. If an enterprise registry proxies npm, verify that `npm audit` still receives advisory data; otherwise document the compensating dependency scanning control before relying on the mirror.

## Local Release Checks

Before tagging a release:

```powershell
go test ./...
go vet ./...
go build ./cmd/sim-server
cd web
npm ci
npm run generate:types
npm test
npm run build
cd ..
make security-audit
docker build -t shipsim:local .
docker compose config
docker compose --env-file .env.production.example config
.\scripts\test-postgres.ps1
```

Do not run release checks with production secrets in the environment. Use example files or secret-backed CI variables only.

## Image Tags

Use traceable image tags:

- Release tags: `shipsim:vX.Y.Z`
- Commit tags: `shipsim:git-<short-sha>`
- CI branch tags: `shipsim:ci-<short-sha>`

The CI workflow builds `shipsim:<tag>` for `v*` tags and `shipsim:ci-<short-sha>` for branch and pull request builds. If publishing to a registry, also record the immutable image digest in the release notes.

## Release Notes

Release notes must include:

- Git commit SHA and image digest.
- Go toolchain version and Node/npm version used by CI.
- OpenAPI contract version.
- `RunReport.version`.
- Required database migration version and migration files.
- Frontend build settings that affect deployment behavior, including API base, auth mode, and map tile URL.
- Authentication mode and proxy header sanitization requirements.
- Data retention defaults and any capacity limits.
- Known production risks or follow-up items.

For releases that include database migrations, state whether the migration is additive or destructive. Destructive migrations require a preview plan and backup location before execution. `migrations/003_training_product.sql` is additive.

## SBOM and Image Scanning

CI uploads an SPDX JSON SBOM artifact for the built image. Keep the SBOM with release artifacts so dependency and base-image provenance can be audited later.

CI uses Trivy for high and critical image vulnerabilities. Grype is an acceptable alternative if a deployment environment already standardizes on it. Treat scanner failures as release blockers unless a documented exception includes the affected package, advisory id, reason for acceptance, expiration date, and owner.

## Production Boundary

Every release must preserve the simulator boundary. Reports, audit logs, scenario records, and assessments are for training review only. Do not add or release integrations that command real sensors, radar control chains, fire-control systems, weapon systems, electronic-warfare devices, or tactical engagement workflows.
