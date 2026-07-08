# ShipSystem 发布 Runbook

适用范围：ShipSystem 本地、预生产和生产发布前后的人工流程与 CI/运维 gate。本文只记录步骤和证据，不记录真实密钥、token、密码或 `.env` 具体值。

## 1. 发布前准备

- 确认发布代码版本、镜像 tag、迁移文件、配置变更和回滚方案。
- 确认生产环境使用 secret manager 或环境变量注入敏感配置。
- 确认生产环境关闭 `DATABASE_AUTO_MIGRATE` 和 `SEED_DEMO_DATA`。
- 确认 CORS、TLS、反向代理头、Cookie Secure、WebSocket 代理和健康检查配置。
- 确认 `docs/openapi.yaml`、`docs/rbac_matrix.yaml` 与代码变更同步。

## 2. 静态发布 Gate

默认执行：

```bash
python scripts/preflight_check.py
```

该命令覆盖：

- Go 单元测试：`go test ./...`
- analytics 测试：`uv run --with-requirements requirements.txt python -m unittest discover -s tests`
- 发布脚本单元测试：`python -m unittest discover -s scripts_tests`
- 前端构建：`npm run build`
- Compose 基线检查：`python scripts/check_compose_config.py`
- WebSocket 事件契约：`python scripts/check_event_contract.py`
- analytics callback 契约：`python scripts/check_callback_contract.py`
- OpenAPI 契约：`python scripts/check_openapi_contract.py`
- 前端 API 覆盖：`python scripts/check_frontend_api_contract.py`
- RBAC 矩阵：`python scripts/check_rbac_matrix.py`

单独排查前端：

```bash
cd frontend
npm run build
npm run test:e2e
```

单独排查脚本 gate：

```bash
python -m unittest discover -s scripts_tests
```

## 3. 发布证据采集

默认证据：

```bash
python scripts/collect_release_evidence.py --output-dir .release-evidence/latest
```

只采集前端 E2E：

```bash
python scripts/collect_release_evidence.py --skip-migration-status --skip-preflight --include-frontend-e2e --output-dir .release-evidence/latest-frontend-e2e
```

采集运行时证据：

```bash
python scripts/collect_release_evidence.py --include-runtime --include-runtime-observability-snapshot --output-dir .release-evidence/latest-runtime
```

采集数据恢复证据：

```bash
python scripts/collect_release_evidence.py --include-backup-restore-drill --output-dir .release-evidence/latest-backup-drill
```

manifest 要求：

- 每个实际执行步骤必须记录 `status=passed|failed`。
- 显式跳过的步骤必须出现在 `skippedSteps`。
- 使用 `--continue-on-failure` 时，最终命令仍应以非零退出码反映失败。

## 4. 数据库迁移

发布前检查：

```bash
cd backend
go run ./cmd/migrate -action=status
go run ./cmd/migrate -action=check
```

执行迁移：

```bash
cd backend
go run ./cmd/migrate -action=up
```

约束：

- 生产环境不使用自动迁移。
- 迁移采用 forward-only 策略；回滚优先通过发布回滚、数据恢复或补偿迁移处理。
- 每个迁移 PR 必须说明数据影响、锁表风险、回滚策略和验证方式。

## 5. 启动与健康检查

启动前检查本机或目标机器运行环境：

```bash
python scripts/runtime_precheck.py
```

本地或预生产 Compose 启动：

```bash
docker compose up --build -d
docker compose ps
```

健康检查：

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
curl http://localhost:8090/health
curl http://localhost:3000/
```

Windows 环境没有 `curl` 时，可用浏览器或 PowerShell `Invoke-WebRequest` 替代。

## 6. 运行时 Smoke

需要栈已启动：

```bash
python scripts/smoke_check.py
```

smoke 关注点：

- 登录和 Cookie 鉴权。
- 受保护 REST 资源访问。
- WebSocket Cookie 鉴权链路。
- 关键业务路径：船舶、位置、告警、调度、对战和 analytics 代理。
- requestId 是否能帮助定位失败。

## 7. 运行时可观测性快照

需要栈已启动：

```bash
python scripts/run_runtime_observability_snapshot.py
```

输出应包含：

- backend `/health`
- backend `/ready`
- analytics `/health`
- frontend `/`
- backend 代理的 analytics status
- API 端 requestId 回显和延迟
- analytics delivery metrics

## 8. DB Integration

推荐使用一键脚本。该脚本会自动启动临时 PostGIS 容器、生成一次性 DSN、执行仓储层集成测试，并在结束后清理容器和卷：

```bash
python scripts/run_repository_db_integration.py
```

如果要复用 `preflight_check.py` 的 `db-integration` gate，则需要先提供一个可丢弃 PostgreSQL/PostGIS DSN。

PowerShell：

```powershell
$env:SHIPSYSTEM_REPOSITORY_TEST_DSN = "host=localhost user=shipsystem password=shipsystem dbname=shipsystem_test port=5432 sslmode=disable TimeZone=Asia/Shanghai"
python scripts/preflight_check.py --only db-integration
```

Bash：

```bash
export SHIPSYSTEM_REPOSITORY_TEST_DSN="host=localhost user=shipsystem password=shipsystem dbname=shipsystem_test port=5432 sslmode=disable TimeZone=Asia/Shanghai"
python scripts/preflight_check.py --only db-integration
```

要求：

- 优先使用 `scripts/run_repository_db_integration.py`，避免手工 DSN 指错环境。
- 手工 DSN 方式必须使用可丢弃数据库。
- 不连接生产库。
- 执行前确认 DSN 不会误指向真实生产数据。

## 9. 备份恢复演练

脚本会启动临时 PostGIS 容器，执行迁移、插入样本、`pg_dump`、恢复到新库并校验行数和样本数据。

```bash
python scripts/run_backup_restore_drill.py
```

采集证据：

```bash
python scripts/collect_release_evidence.py --include-backup-restore-drill --output-dir .release-evidence/latest-backup-drill
```

生产要求：

- 定期演练，而不是只在上线前演练一次。
- 保存演练时间、数据库版本、备份大小、恢复耗时、校验结果和负责人。
- 恢复演练不得覆盖生产库。

## 10. 保留策略和容量估算

预览清理：

```bash
python scripts/retention_maintenance.py
```

容量估算：

```bash
python scripts/run_capacity_smoke.py --estimate-only
```

采集证据：

```bash
python scripts/collect_release_evidence.py --include-retention-preview --output-dir .release-evidence/latest-retention-preview
python scripts/collect_release_evidence.py --include-capacity-estimate --output-dir .release-evidence/latest-capacity-estimate
```

原则：

- 先用真实查询路径和增长估算确定 retention 窗口。
- 不为了清理数据盲目变更 schema。
- 清理脚本默认先跑 preview，确认后再执行破坏性清理。

## 11. 前端 E2E

```bash
cd frontend
npm run test:e2e
```

CI 中会安装 Chromium 并归档 E2E 证据。失败时优先查看 Playwright trace、失败截图、mock 数据和 route guard。

## 12. 回滚流程

推荐顺序：

1. 停止新流量或切回上一版前端/后端镜像。
2. 保留当前故障证据：compose ps、服务日志、requestId、健康检查、发布证据目录。
3. 判断是否涉及数据库迁移。
4. 如果只涉及应用代码，回滚镜像并重新执行健康检查和 smoke。
5. 如果涉及数据，先评估补偿迁移或恢复窗口，禁止直接覆盖生产数据。
6. 记录故障时间线、影响范围、根因、修复版本和后续防回归 gate。

## 13. 发布后验证

```bash
python scripts/smoke_check.py
python scripts/run_runtime_observability_snapshot.py
```

人工确认：

- 前端能登录并进入首页。
- 主要页面无明显白屏、路由错误、权限误跳转。
- WebSocket 状态可连接，实时数据能更新。
- 后端 `/ready` 正常，analytics `/health` 正常。
- 关键错误响应包含 requestId。

## 14. CI 与运维工作流

当前仓库包含：

- `.github/workflows/ci.yml`：PR/push 静态发布 gate 和前端 E2E 证据。
- `.github/workflows/operations-gates.yml`：手动或定时执行运行时和数据 gate。

运维 gate 会启动本地 Compose 栈，并采集 runtime smoke、DB integration、备份恢复演练、runtime observability snapshot、retention preview、capacity estimate 和 Compose 诊断。

## 15. 常见失败处理

- OpenAPI gate 失败：同步 `docs/openapi.yaml` 的路径、method、`x-roles`、requestBody、成功响应和常见错误响应。
- RBAC gate 失败：同步 `docs/rbac_matrix.yaml`、Go route middleware 和前端页面角色。
- WebSocket gate 失败：同步后端事件、`frontend/src/types.ts`、前端消费逻辑、README 事件列表和 smoke 监听点。
- frontend API gate 失败：新增或变更 REST 调用后，先更新 OpenAPI，再重新生成前端类型。
- Compose gate 失败：检查 healthcheck、depends_on、镜像 tag、非 root 用户、Nginx 安全响应头和 WebSocket 代理。
- 运行时 smoke 失败：先用 requestId 关联前端错误、Go 日志、analytics 日志，再判断是配置、鉴权、数据库还是服务间调用问题。
