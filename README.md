# ShipSystem 船舶管理与监控调度系统

ShipSystem 是一个面向船舶基础资料、实时位置、告警、调度事件、轨迹回放和雷达对战模拟的管理系统。当前仓库包含 Go 后端、Python analytics 服务、React 前端、PostgreSQL/PostGIS 数据库和本地发布检查脚本。

## 技术栈

| 模块 | 技术 |
| --- | --- |
| 后端 API | Go、Gin、GORM、JWT、WebSocket |
| 数据库 | PostgreSQL、PostGIS、SQL migrations |
| 分析服务 | Python、FastAPI、httpx、Pydantic |
| 前端 | React、Vite、TypeScript、Ant Design、OpenLayers、lucide-react |
| 交付 | Docker Compose、GitHub Actions、发布证据脚本 |

## 功能范围

- 船舶管理：船舶列表、详情、新增、编辑、软删除。
- 位置与轨迹：位置上报、最近位置展示、历史轨迹查询。
- 告警中心：告警列表、告警确认和 WebSocket 实时告警。
- 调度事件：调度创建、状态流转、调度事件实时更新。
- 雷达对战：场景列表、会话创建、实时状态、时间线、快照和报告导出。
- RBAC：`super_admin`、`admin`、`dispatcher`、`viewer`、`analytics_service` 角色。
- 发布治理：OpenAPI、RBAC、WebSocket、analytics callback、前端 API、Compose 等静态契约检查。

## 目录结构

```text
backend/        Go API、数据库迁移、领域服务、仓储和 WebSocket hub
analytics/      Python FastAPI 模拟与回调服务
frontend/       React 管理端、Playwright E2E、前端静态 gate
docs/           OpenAPI、RBAC 矩阵、发布和运维文档
scripts/        发布前检查、证据采集、运行时 smoke、备份恢复演练
scripts_tests/  发布脚本的单元测试
.github/        CI 和运维 gate 工作流
```

## 快速启动

本地联调优先使用 Docker Compose。复制环境变量模板后，按环境补齐必要变量；不要把真实密钥、令牌、密码提交到仓库。

```bash
cp .env.example .env
python scripts/runtime_precheck.py
docker compose up --build
```

默认访问地址：

- 前端：`http://localhost:3000`
- Go API：`http://localhost:8080/api/v1`
- analytics：`http://localhost:8090`

登录账号和密码来自初始化数据与环境变量配置。生产环境必须通过环境变量或 secret manager 注入强值。

## 本地开发

后端：

```bash
cd backend
go mod tidy
go run ./cmd/api
```

analytics：

```bash
cd analytics
python -m venv .venv
.\.venv\Scripts\Activate.ps1
pip install -r requirements.txt
uvicorn app.main:app --reload --port 8090
```

前端：

```bash
cd frontend
npm install
npm run dev
```

前端开发服务器默认监听 `5173`，并通过 Vite proxy 转发 `/api` 和 `/ws` 到 Go API。

## 配置原则

- `APP_ENV=production` 时必须关闭自动迁移和演示数据种子。
- 生产环境必须配置强 `JWT_SECRET`、`ADMIN_PASSWORD`、`ANALYTICS_ADMIN_TOKEN`、`GO_API_TOKEN`。
- `GO_API_TOKEN` 只用于 analytics 回调 Go API，映射为 `analytics_service` 角色。
- `ANALYTICS_ADMIN_TOKEN` 只用于 Go API 调用 analytics 管理接口。
- `CORS_ORIGINS` 在生产环境不能使用通配符。
- Go API 登录后设置 `shipsystem_token` HttpOnly Cookie；也支持 `Authorization: Bearer <JWT>`。
- 生产环境 Cookie 会启用 `Secure`，部署层必须正确传递 HTTPS 代理头。

## 数据库迁移

Go API 使用 `backend/internal/database/migrations/*.sql` 作为应用内迁移来源，并写入 `schema_migrations` 表。生产发布建议显式执行迁移命令，而不是依赖启动时自动迁移。

```bash
cd backend
go run ./cmd/migrate -action=status
go run ./cmd/migrate -action=check
go run ./cmd/migrate -action=up
```

`backend/migrations/001_init.sql` 保留给 Docker Compose 首次初始化 PostgreSQL/PostGIS 使用。

## API 与契约

- REST 契约：`docs/openapi.yaml`
- RBAC 矩阵：`docs/rbac_matrix.yaml`
- OpenAPI 静态检查：`python scripts/check_openapi_contract.py`
- 前端 API 覆盖检查：`python scripts/check_frontend_api_contract.py`
- RBAC 矩阵检查：`python scripts/check_rbac_matrix.py`
- WebSocket 事件契约检查：`python scripts/check_event_contract.py`
- analytics callback 契约检查：`python scripts/check_callback_contract.py`

主要 REST 入口：

- `/api/v1/auth/login`
- `/api/v1/auth/logout`
- `/api/v1/ships`
- `/api/v1/ships/{id}/locations`
- `/api/v1/ships/{id}/tracks`
- `/api/v1/alarms`
- `/api/v1/dispatch-events`
- `/api/v1/battle/scenarios`
- `/api/v1/battle/sessions`
- `/api/v1/radar/reports`
- `/api/v1/analytics/simulate/*`
- `/api/v1/rbac/*`

## WebSocket

监控 WebSocket 地址为 `/ws/monitor`。浏览器优先通过 Cookie 鉴权，也支持 Bearer token。当前事件类型必须同时在后端、前端类型和 README 中维护：

- `ship_location_updated`
- `alarm_created`
- `dispatch_event_updated`
- `radar_scan_updated`
- `projectile_updated`
- `battle_event_created`
- `battle_state_updated`
- `heartbeat`

## 发布前检查

最快的静态发布 gate：

```bash
python scripts/preflight_check.py
```

需要分步排查时：

```bash
cd backend
go test ./...
```

```bash
cd analytics
uv run --with-requirements requirements.txt python -m unittest discover -s tests
```

```bash
python -m unittest discover -s scripts_tests
python scripts/check_compose_config.py
python scripts/check_event_contract.py
python scripts/check_callback_contract.py
python scripts/check_openapi_contract.py
python scripts/check_frontend_api_contract.py
python scripts/check_rbac_matrix.py
```

```bash
cd frontend
npm run build
npm run test:e2e
```

运行时 gate 需要本地栈或临时数据库环境，详见 `docs/release_runbook.md`：

```bash
python scripts/runtime_precheck.py
python scripts/smoke_check.py
python scripts/run_runtime_observability_snapshot.py
python scripts/run_repository_db_integration.py
python scripts/run_backup_restore_drill.py
python scripts/run_capacity_smoke.py --estimate-only
```

## 发布证据

发布证据统一写入 `.release-evidence/`：

```bash
python scripts/collect_release_evidence.py --output-dir .release-evidence/latest
```

常用扩展项：

```bash
python scripts/collect_release_evidence.py --include-runtime --output-dir .release-evidence/latest-runtime
python scripts/collect_release_evidence.py --include-frontend-e2e --skip-migration-status --skip-preflight --output-dir .release-evidence/latest-frontend-e2e
python scripts/collect_release_evidence.py --include-backup-restore-drill --output-dir .release-evidence/latest-backup-drill
```

CI 会采集静态 gate 和前端 E2E 证据；运维工作流会定期采集 runtime smoke、DB integration、备份恢复演练、runtime observability snapshot、retention preview 和 capacity estimate。

## 文档索引

- `docs/production_audit.md`：生产级差距、风险和后续路线。
- `docs/release_runbook.md`：发布、回滚、证据采集和排障流程。
- `docs/operations_observability.md`：日志、告警、仪表盘和容量基线。
- `docs/openapi.yaml`：REST API 契约。
- `docs/rbac_matrix.yaml`：后端路由和前端页面角色矩阵。

## 编码约定

所有文档使用 UTF-8 保存，中文直接写入源码，不使用 Unicode escape。Windows 终端查看中文时如出现乱码，优先确认终端输出编码，而不是把文件转成 GBK。
