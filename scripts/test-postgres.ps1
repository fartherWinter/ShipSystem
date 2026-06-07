param(
    [switch]$KeepDatabase
)

$ErrorActionPreference = "Stop"

$project = "shipsim-test"
$composeFile = "docker-compose.test.yml"
$databaseUrl = "postgres://shipsim_test:shipsim-test-only@127.0.0.1:15432/shipsim_test?sslmode=disable"

Write-Host "Starting isolated Postgres test database with Compose project '$project'."
Write-Host "The cleanup step removes only the '$project' test containers and volumes."

try {
    docker compose -p $project -f $composeFile --profile test up -d db-test

    $ready = $false
    for ($i = 0; $i -lt 60; $i++) {
        docker compose -p $project -f $composeFile exec -T db-test pg_isready -U shipsim_test -d shipsim_test | Out-Null
        if ($LASTEXITCODE -eq 0) {
            $ready = $true
            break
        }
        Start-Sleep -Seconds 1
    }
    if (-not $ready) {
        throw "Postgres test database did not become ready in time."
    }

    $env:TEST_DATABASE_URL = $databaseUrl
    go test -count=1 ./internal/store
    if ($LASTEXITCODE -ne 0) {
        throw "Postgres integration tests failed."
    }
}
finally {
    Remove-Item Env:\TEST_DATABASE_URL -ErrorAction SilentlyContinue
    if (-not $KeepDatabase) {
        docker compose -p $project -f $composeFile --profile test down -v
    }
}
