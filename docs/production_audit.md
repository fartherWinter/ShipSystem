# ShipSystem 生产级项目审计报告

审计日期：2026-06-09

审计范围：`C:\Users\chenn\Documents\ShipSystem` 当前 worktree，覆盖 Go 后端、Python analytics、React 前端、Docker Compose、本地发布前脚本和 README 中的阶段路线。

## 结论

当前项目已经从演示型 MVP 推进到“可进入预生产联调”的状态：安全配置、Cookie-first 鉴权、RBAC、输入校验、迁移治理、服务间 token、WebSocket 可靠性、analytics 回调重试、前端路由拆分和静态发布前检查都已有明确实现和验证入口。

但它还不能直接判定为完整生产级终态。主要原因已经不再是“缺少基础发布证据”，而是仍缺正式 CI/CD gate、生产密钥轮换与部署参数策略、数据库备份/恢复演练、集中化日志指标告警，以及更完整的业务级并发和权限回归场景。

## 当前生产化基线

### 安全与配置

- `backend/internal/config/config.go` 会在生产环境拒绝默认 `JWT_SECRET`、默认管理员密码、通配 CORS、自动迁移、演示数据和缺失/过短服务 token。
- `backend/internal/handlers/handlers.go` 登录后设置 `shipsystem_token` HttpOnly Cookie，退出时过期同名 Cookie；生产环境 Cookie 会强制 Secure。
- `backend/internal/middleware/security.go` 与 `frontend/nginx.conf` 已提供基础安全响应头，前端 Nginx 启用 CSP。
- `scripts/smoke_check.py` 已覆盖登录 Cookie、受保护资源访问、WebSocket Cookie 鉴权链路和退出 Cookie 清理。

### 权限与输入边界

- `backend/internal/middleware/auth.go` 支持 `super_admin`、`admin`、`dispatcher`、`viewer` 和 `analytics_service` 角色。
- `backend/internal/handlers/handlers.go` 在路由层按角色约束船舶、告警、调度、对战、RBAC 和 analytics 控制接口。
- `docs/rbac_matrix.yaml` 记录 REST 路由和前端页面角色矩阵，`scripts/check_rbac_matrix.py` 校验后端 handler 实际注册权限与矩阵一致，并校验前端页面角色与 `frontend/src/App.tsx` 一致。
- URL ID、分页、坐标、航向、速度、时间、tick range、battle scenario 等关键输入已有显式校验。
- analytics 服务端通过 Pydantic validator 校验 shipId、坐标、速度、航向、sessionId 和 scenarioCode。

### 数据库与发布迁移

- `backend/internal/database/database.go` 使用 embedded SQL migrations，执行记录写入 `schema_migrations`。
- `backend/cmd/migrate` 提供 `-action=status` 和 `-action=up`，生产发布可以显式检查与执行迁移。
- README 明确采用 forward migration 策略，不提供自动 down migration。
- `backend/migrations/001_init.sql` 保留为 Compose 首次初始化 PostGIS 的职责边界。

### 服务间调用与仿真可靠性

- Go 调 analytics 使用 `ANALYTICS_ADMIN_TOKEN`，analytics 回调 Go 使用 `GO_API_TOKEN`。
- `GO_API_TOKEN` 在 Go API 侧映射为 `analytics_service`，仅允许位置上报和雷达战斗上报相关入口。
- `backend/internal/services/analytics.go` 对 analytics 响应做超时、错误摘要、requestId 透传和响应体大小限制。
- `analytics/app/main.py` 对回调 Go API 做 retry、非重试错误丢弃、delivery metrics 和 requestId 透传。

### WebSocket 与可观测性

- `backend/internal/ws/hub.go` 已具备广播队列上限、慢客户端剔除、ping/pong、deadline、heartbeat 和 dropped broadcast 计数。
- Go API 和 analytics 均透传或生成 `X-Request-ID`，错误响应中包含 requestId。
- 前端错误提示和 smoke 脚本失败输出会携带 requestId，便于定位 Go/Python 日志。

### 前端工程化

- `frontend/src/App.tsx` 使用真实路由、登录守卫、角色菜单和页面懒加载。
- `frontend/scripts/check_frontend_routes.mjs` 静态校验核心业务路由、角色配置、懒加载页面和 wildcard redirect。
- `frontend/scripts/check_frontend_bundle.mjs` 作为 bundle budget gate 接入 `npm run build`。
- `frontend/package.json` 的 `npm run build` 现已先执行 `npm run test:static`，对前端路由 gate 和 bundle gate 的核心逻辑做 `node:test` 回归，再继续 `tsc -b`、路由检查、Vite 构建和 bundle budget gate。
- `frontend/vite.config.ts` 现已将 React、地图、Ant Design 主包、`@rc-component` 依赖和 `@ant-design/icons` 拆为稳定共享 chunk；当前最大 JS chunk 已从约 817.5 KiB 降到约 445.1 KiB，且无 Rollup chunk circular warning。
- 核心页面已覆盖 loading、error、empty、retry 等基础业务状态。
- `frontend/src/App.tsx` 会消费调度事件、雷达扫描、弹丸、battle event 和 battle state 的 WebSocket 更新，减少列表和地图页面对手动刷新的依赖。

### 运维交付

- Dockerfile 最终阶段使用非 root 用户。
- `docker-compose.yml` 提供服务 healthcheck 和健康依赖。
- `scripts/check_compose_config.py` 校验 Compose 渲染结果、healthcheck、健康依赖、服务 token、镜像 tag、Dockerfile 非 root、`.dockerignore`、Nginx 安全响应头和 WebSocket 代理头。
- `scripts/runtime_precheck.py` 提供运行态 smoke 前置检查，覆盖 Docker CLI、Docker Desktop Service 状态、Compose、daemon 访问权限、端口占用和 smoke 目标地址，并使用工作区 `.docker-codex` 规避用户目录 Docker 配置权限干扰；在 Windows 上会把被占用端口解析到 PID 和进程名。
- `scripts/check_event_contract.py` 校验后端 WebSocket 广播事件、前端 `WsMessage` union、前端事件消费、README 消息列表和 smoke 关键事件监听，降低接口事件契约漂移风险。
- `scripts/check_callback_contract.py` 校验 Python analytics 雷达对战回调样本与 Go `RadarReportPayload` / 子 payload JSON 字段，降低跨服务 callback payload 漂移风险。
- `docs/openapi.yaml` 提供 REST OpenAPI 操作级契约，`scripts/check_openapi_contract.py` 会校验它不仅覆盖 Go handler 注册的全部 REST 路由，还覆盖受保护接口 `x-roles`、public/protected 鉴权声明、关键 requestBody、成功状态码以及常见错误响应。
- `scripts/check_frontend_api_contract.py` 校验前端 `frontend/src/api/client.ts` 的 REST 调用都被 OpenAPI 覆盖，降低前端调用路径和后端契约漂移风险。
- `scripts/check_rbac_matrix.py` 校验 REST RBAC 权限矩阵和 Go handler middleware 绑定，并校验前端页面角色矩阵，降低权限变更漏审计风险。
- `scripts_tests/` 现已为关键发布前 gate 脚本补单元测试，当前覆盖 OpenAPI 契约脚本的 YAML 解析、runtime precheck 的 Windows 端口与 Docker 服务诊断分支、compose baseline 解析校验、smoke gate 的纯函数和 cookie/requestId 本地约束，以及 frontend API / RBAC / WebSocket event / analytics callback 等静态契约脚本的核心解析逻辑。

## 主要问题清单

| 优先级 | 问题 | 影响 | 建议 |
| --- | --- | --- | --- |
| P0 | 缺少 CI/CD 配置和强制 gate | 当前验证依赖人工执行，容易漏跑或用错 Python 环境 | 引入 CI，至少强制 Go tests、analytics tests、frontend build、compose checker |
| P0 | 生产密钥、数据库密码和 token 轮换策略未落地 | 生产接入时可能继续使用弱默认值或手工散落配置 | 使用环境变量/secret manager，建立轮换和泄露响应流程 |
| P1 | 缺少数据库备份保留策略和常态化恢复演练证据 | 当前已经具备临时 PostGIS 上的 backup/restore drill 脚本与一次真实通过证据，但还没有生产节奏下的周期化演练、备份保留策略和责任归档 | 将 backup/restore drill 纳入发布证据或周期演练，并补充生产级备份保留、恢复窗口和责任人机制 |
| P1 | 缺少集中化日志、指标和告警平台 | 当前已经具备 requestId 透传、analytics delivery 状态接口和运行态 observability snapshot 脚本，但还没有集中化采集、聚合展示和主动告警 | 接入日志采集、指标暴露、错误率/延迟/WS 掉线/analytics delivery 告警，并把 snapshot 结果纳入发布或巡检证据 |
| P1 | 业务并发和幂等测试仍偏少 | 高并发位置上报、告警确认、调度状态流转、雷达回放可能出现边界问题 | 增加 service/repository 层并发、事务、幂等和状态机测试 |
| P1 | 前端浏览器级 E2E 覆盖仍不足 | 当前已补 Playwright 基座，覆盖登录、RBAC 无权限跳转、船舶表格新增/编辑/删除和退出流程，但地图、调度表格、对战回放还没有浏览器级回归 | 继续扩展 Playwright 覆盖角色菜单、更多表格交互、地图、调度和对战回放 |
| P2 | 地图与 Ant Design 共享 chunk 仍是前端首屏体积主因 | 当前最大 JS chunk 已降到约 445 KiB，但 `vendor-antd`、`vendor-antd-rc`、`vendor-map` 仍是主要传输成本 | 后续继续按路由使用面拆重组件，引入浏览器级性能基线，并结合真实访问链路决定是否继续下调 bundle budget |
| P2 | Docker Compose 仍偏本地开发 | 适合本地和预生产 smoke，不等于生产部署拓扑 | 生产环境单独维护 Helm/Kustomize/Compose prod override，并纳入审计 |
| P1 | 部分接口错误语义仍不统一 | 多数 not-found 路径已统一到 404，但仍需继续复核剩余 handler/service 分支，避免底层错误字符串或 500 语义泄漏到外部契约 | 继续逐个收敛剩余 not-found 分支，并补对应回归测试；完成后再同步收紧 OpenAPI 与 smoke 断言 |

### 2026-06-09 progress note

- Alarm ACK and dispatch status updates now have repository-level idempotency with row locks, `changed` return values, and service-layer broadcast suppression for repeated requests.
- `backend/internal/repositories/store_integration_test.go` adds PostgreSQL-backed concurrent tests for ACK idempotency and dispatch transition log de-duplication.
- Radar/battle callbacks now broadcast only newly inserted battle events and append battle snapshots idempotently by `session_id + snapshot_time` under a battle session row lock, reducing replay/report inflation when analytics retries the same payload.
- Battle session stop is now idempotent with repository row locking and a `changed` return value; repeated stop calls return the current state without creating extra stop events, snapshots, or WebSocket broadcasts.
- WebSocket event contract drift now has a static gate, and the frontend dispatch page consumes `dispatch_event_updated` instead of relying only on manual refresh.
- Analytics-to-Go radar callback payload now has a static contract gate generated from a Python battle simulation sample and Go JSON tags.
- REST contract governance is now operation-level, not just path coverage: `docs/openapi.yaml` is statically checked against Go handler registrations, RBAC role declarations, auth boundary markers, required request bodies, and common success/error responses.
- Frontend REST client calls are now checked against OpenAPI so UI code cannot introduce undocumented API paths silently.
- REST RBAC authorization now has a route matrix and static gate checked against Go handler middleware bindings and frontend page roles.
- Ship location writes and track queries now verify that the ship exists before writing or returning data; repository-level location creation also rejects soft-deleted ships to prevent orphan track/alarm data.
- The DB integration gate is opt-in because it needs a disposable PostgreSQL database: set `SHIPSYSTEM_REPOSITORY_TEST_DSN` and run `python scripts/preflight_check.py --only db-integration`.
- Frontend bundle splitting has been tightened without changing route behavior: the Vite manual chunk strategy now separates React, map, Ant Design core, `@rc-component`, and Ant Design icon dependencies. Current `npm run build` evidence shows the largest JS chunk at about 445.1 KiB, down from the earlier ~817.5 KiB single `vendor-antd` chunk, and the bundle gate is now enforced at 600 KiB per JS chunk.
- Frontend route and bundle gate scripts now export reusable check functions and have `node:test` regression coverage wired into `npm run build` via `npm run test:static`, so route/bundle guard logic is no longer validated only indirectly by a successful build.
- The gate scripts now have their own regression checks: `python -m unittest discover -s scripts_tests` currently passes with coverage over the custom OpenAPI parser, the runtime precheck Windows diagnostics, the compose baseline parser, the smoke gate helpers, and the frontend API / RBAC / WebSocket event / analytics callback static contract parsers, reducing the risk that release gates themselves drift silently.
- Remaining gap: CI/CD enforcement is still missing, centralized metrics/alerting and backup/restore drills are still missing, some handler not-found branches still need follow-up review, schema-level uniqueness for snapshot time still needs an explicit migration decision, and browser E2E coverage is still only partial beyond the current auth/RBAC/table/dispatch/replay/alarm/track/monitor baseline.

### 2026-06-26 progress note

- Full local runtime smoke has been executed successfully: `python scripts/smoke_check.py` reported `18/18 smoke checks passed`.
- The PostgreSQL-backed repository integration gate has been executed successfully with a real disposable DSN: `python scripts/run_repository_db_integration.py` passed.
- A database backup/restore drill is now executable and has passed on a real temporary PostGIS stack: `python scripts/run_backup_restore_drill.py` created a custom-format `pg_dump`, restored it into a fresh database, re-checked migrations, and verified restored sample rows plus table counts.
- A read-only runtime observability snapshot is now executable and has passed on the current local stack: `python scripts/run_runtime_observability_snapshot.py` captured backend `/health`, backend `/ready`, analytics `/health`, frontend `/`, and backend-proxied analytics status with requestId echo checks for API endpoints plus delivery metrics and latency evidence.
- Release evidence collection is now real, not just designed: `python scripts/collect_release_evidence.py --output-dir .release-evidence/latest` and `python scripts/collect_release_evidence.py --include-runtime --include-db-integration --output-dir .release-evidence/latest-full` both completed and wrote manifest-backed artifacts.
- Backup/restore drill evidence can now be archived explicitly: `python scripts/collect_release_evidence.py --include-backup-restore-drill --output-dir .release-evidence/latest-backup-drill` produced a manifest-backed artifact set for this workspace state.
- Runtime observability evidence can now be archived explicitly: `python scripts/collect_release_evidence.py --include-runtime-observability-snapshot --output-dir .release-evidence/latest-observability` produced a manifest-backed artifact set for this workspace state.
- The full evidence bundle in `.release-evidence/latest-full` now contains migrate status, preflight output, smoke output, and DB integration output, which closes the previous “no runtime evidence / no DB integration evidence” audit gap for this workspace state.
- Handler error semantics have been tightened further: track queries, alarm ACK, dispatch status updates, battle timeline/snapshot/report/stop endpoints, radar report callbacks, and analytics proxy handlers now avoid leaking raw storage/internal errors while keeping stable 404-facing or upstream-facing messages.
- Browser E2E now has a real Playwright base: `frontend/playwright.config.ts` plus mocked browser flows currently cover successful login-to-dashboard rendering, viewer-role redirect away from `/rbac`, ship table create/edit/delete, dispatch event create/status progression, battle replay session loading/step-forward behavior, alarm ACK, track query rendering, monitor page connectivity/simulator start, and logout, giving the repo executable browser-level auth/RBAC/table/dispatch/replay/alarm/track/monitor/logout regression checks via `cd frontend`, then `npm run test:e2e`.
- Browser E2E can now be pulled into release scripts explicitly: `python scripts/preflight_check.py --only frontend-e2e` passes on this workspace, and `python scripts/collect_release_evidence.py --continue-on-failure --include-frontend-e2e --output-dir .release-evidence/latest-frontend-e2e` archives a manifest-backed `frontend-e2e` step even when local `migration-status` still fails without a reachable PostgreSQL instance.

## 后续开发路线

### 阶段 A：发布证据闭环

目标：每次变更都能证明三段链路可工作。

- 建立 CI：优先复用 `python scripts/preflight_check.py`，或等价执行 `go test ./...`、`uv run --with-requirements requirements.txt python -m unittest discover -s tests`、`npm run build`、`python scripts/check_compose_config.py`、`python scripts/check_event_contract.py`、`python scripts/check_callback_contract.py`、`python scripts/check_openapi_contract.py`、`python scripts/check_frontend_api_contract.py`、`python scripts/check_rbac_matrix.py`。
- 启动完整栈前执行 `python scripts/runtime_precheck.py`，先确认 Docker daemon 权限、Compose 配置、端口和 smoke 目标地址。
- 在完整本地栈和预生产栈执行 `scripts/smoke_check.py`。
- 在已启动栈上补一份只读运行态观测证据：`python scripts/preflight_check.py --only runtime-observability-snapshot` 或 `python scripts/run_runtime_observability_snapshot.py`。
- 按 `docs/release_runbook.md` 归档 smoke 输出、镜像版本、迁移状态和关键环境变量清单。

### 阶段 B：生产运维能力

目标：上线后能恢复、能定位、能告警。

- 编写部署 Runbook：迁移前检查、迁移执行、健康检查、回滚发布、日志定位。
- 建立数据库备份和恢复演练。
- 接入日志聚合、metrics、dashboard 和告警规则。
- 将 secret 注入、轮换、最小权限账号纳入发布流程。

### 阶段 C：业务正确性加固

目标：核心业务在并发、异常和边界输入下保持一致。

- 补充告警确认幂等、调度状态流转、船舶删除/轨迹查询、battle session 生命周期测试。
- 对 analytics 回调失败、重复雷达上报、WebSocket 慢客户端和重连做更完整回归。
- 为服务层关键写路径明确事务边界和唯一约束策略。

### 阶段 D：前端体验和性能

目标：控制台可长期维护，且弱网可接受。

- 继续扩展 Playwright E2E，补齐角色菜单、地图实时刷新、调度流转和对战回放。
- 优化 Ant Design 和地图相关 chunk，控制首屏 JS 和样式体积。
- 统一页面错误组件、空状态、刷新按钮和 requestId 展示规范。

### 阶段 E：接口与集成治理

目标：让系统从内部项目变成可集成平台。

- 继续细化 OpenAPI 契约中的请求体、响应 schema 和示例。
- 明确错误码、分页、排序、权限矩阵、幂等键和事件类型。
- 对 WebSocket 消息结构、battle snapshot JSONB、analytics callback payload 建立 schema 或契约测试。

## 发布前最小 Gate

完整发布流程见 `docs/release_runbook.md`。

每次进入预生产或生产发布前，至少执行：

```bash
python scripts/preflight_check.py
```

需要单独排错时，可分步执行：

```bash
cd backend
go test ./...
go run ./cmd/migrate -action=status
```

```bash
cd analytics
uv run --with-requirements requirements.txt python -m unittest discover -s tests
```

```bash
cd frontend
npm run build
```

```bash
C:\Users\chenn\.cache\codex-runtimes\codex-primary-runtime\dependencies\python\python.exe scripts\check_compose_config.py
```

完整栈启动后执行：

```bash
python scripts/runtime_precheck.py
python scripts/smoke_check.py
```

如果本机 `python` 不在 PATH，使用明确 Python 路径或 `uv run`，避免验证命令跑在缺依赖环境里。

## 当前不能声明完成的事项

- 没有 CI/CD 对阶段 gate 的强制执行证据。
- 没有生产环境配置、secret、备份恢复、日志指标告警的落地证据。
- 浏览器级 E2E 已有首批结果，但覆盖面还不够完整。

因此当前 Goal 仍应保持 active，后续继续按“发布证据闭环 -> 运维能力 -> 业务正确性 -> 前端 E2E/性能 -> 接口契约”推进。
