# ShipSystem 生产级项目审计报告

审计日期：2026-07-08

审计范围：当前 worktree 中的 Go API、Python analytics、React 前端、Docker Compose、GitHub Actions、发布脚本、`README.md` 和 `docs/`。

## 结论

ShipSystem 已经具备预生产联调基础：核心业务链路完整，REST/OpenAPI、RBAC、WebSocket、analytics callback、前端 API 和 Compose 都有静态 gate；运行时 smoke、DB integration、备份恢复演练、runtime observability snapshot、retention preview、capacity estimate 也已经脚本化。

当前距离稳定生产仍有差距，主要集中在生产部署参数、真实环境观测平台、周期性恢复演练、数据保留策略、业务并发回归和部分中文文案编码质量。建议按“发布 gate 固化 -> 运维证据闭环 -> 业务正确性加固 -> 前端体验与性能治理”的顺序推进。

## 已具备的生产化能力

### 安全与配置

- 后端在生产环境拒绝弱 `JWT_SECRET`、弱管理员密码、弱服务 token、通配 CORS、自动迁移和演示数据种子。
- 登录后写入 `shipsystem_token` HttpOnly Cookie；生产环境 Cookie 启用 `Secure`。
- REST 和 WebSocket 都接入鉴权；analytics 回调通过服务 token 映射为 `analytics_service`。
- Go API 和 analytics 之间区分管理 token 与回调 token，避免同一个 token 同时承担双向权限。
- 请求体大小、HTTP read/write/header/idle timeout、graceful shutdown timeout 都已配置化。

### 权限与输入边界

- 后端角色包括 `super_admin`、`admin`、`dispatcher`、`viewer`、`analytics_service`。
- `docs/rbac_matrix.yaml` 记录 REST 路由和前端页面角色矩阵，`scripts/check_rbac_matrix.py` 校验矩阵与 Go handler、前端页面角色保持一致。
- URL ID、分页、坐标、航向、速度、时间范围、battle scenario、sessionId 等关键输入有显式校验。
- analytics 侧使用 Pydantic validator 校验 shipId、坐标、速度、航向、sessionId 和 scenarioCode。

### 数据库与迁移

- 应用内迁移来自 `backend/internal/database/migrations/*.sql`，迁移状态写入 `schema_migrations`。
- `backend/cmd/migrate` 支持 `status`、`check`、`up`，适合发布前显式检查。
- Docker Compose 首次初始化保留 `backend/migrations/001_init.sql`，职责边界清晰。
- 已有 repository DB integration gate，可以在一次性 PostgreSQL 数据库上验证关键写路径。

### 服务间调用

- Go API 调 analytics 带 `ANALYTICS_ADMIN_TOKEN`，analytics 回调 Go 带 `GO_API_TOKEN`。
- `backend/internal/services/analytics.go` 处理超时、错误摘要、requestId 透传和响应体大小限制。
- analytics 回调有 retry、非重试错误丢弃、delivery metrics 和 requestId 透传。

### WebSocket 与实时能力

- WebSocket hub 有广播队列上限、慢客户端剔除、ping/pong、deadline、heartbeat 和 dropped broadcast 计数。
- 当前事件契约包括 `ship_location_updated`、`alarm_created`、`dispatch_event_updated`、`radar_scan_updated`、`projectile_updated`、`battle_event_created`、`battle_state_updated`、`heartbeat`。
- `scripts/check_event_contract.py` 校验后端事件、前端 union、前端消费逻辑、README 文档和 smoke 监听点的一致性。

### 前端工程化

- 前端使用真实路由、登录守卫、角色菜单和页面懒加载。
- `npm run build` 会先执行前端静态测试、OpenAPI 类型生成、TypeScript 构建、路由 gate、Vite 构建和 bundle gate。
- Vite 已按 React、OpenLayers、Ant Design、图标等依赖拆分 vendor chunk。
- Playwright E2E 覆盖登录、RBAC、船舶、告警、调度、对战、轨迹、监控、WebSocket 消费和错误重试等核心浏览器流程。

### 发布与运维证据

- `python scripts/preflight_check.py` 统一串联 Go、analytics、脚本、前端、Compose 和契约 gate。
- `scripts/collect_release_evidence.py` 能生成 manifest-backed 发布证据目录。
- GitHub Actions 已包含静态 CI、前端 E2E 和运维 gate 工作流。
- 运维 gate 覆盖 runtime precheck、Compose 启动、smoke、DB integration、备份恢复演练、runtime observability snapshot、retention preview 和 capacity estimate。

## 主要风险清单

| 优先级 | 风险 | 影响 | 建议 |
| --- | --- | --- | --- |
| P0 | 生产 secret 注入、轮换和泄露响应流程未在仓库内落成 Runbook | 上线时容易沿用临时配置或缺少轮换纪律 | 在部署平台中接入 secret manager，定义 owner、轮换周期、泄露响应和审计记录 |
| P0 | 当前 Compose 更偏本地和预生产验证，不等于生产部署拓扑 | 真实生产网络、TLS、域名、资源限制和扩缩容策略缺失 | 为生产单独维护 Helm/Kustomize/Compose prod override，并纳入发布审计 |
| P1 | 集中化日志、指标、告警平台未落地 | 线上故障仍依赖脚本快照和人工排查 | 接入日志采集、指标暴露、dashboard 和主动告警 |
| P1 | 备份恢复演练已有脚本，但生产节奏、保留策略和责任人还需制度化 | 恢复能力无法持续证明 | 将备份恢复演练纳入运维 gate 或定期巡检，并保存证据 |
| P1 | 业务并发和幂等测试仍应继续扩展 | 高并发位置上报、告警确认、调度流转、雷达回放可能出现边界问题 | 增加 service/repository 层并发、事务、唯一约束和状态机测试 |
| P1 | 数据保留策略仍需结合真实容量压测定稿 | battle/radar 数据长期增长会推高存储和查询成本 | 以 capacity estimate、retention preview 和真实查询路径决定保留窗口与索引 |
| P2 | 前端部分中文显示文案和少数后端错误消息仍存在编码质量风险 | 用户界面可能继续出现乱码 | 单独修复源码中的中文字符串，并用 UTF-8 检查脚本防回归 |
| P2 | OpenAPI schema 仍可继续细化 | 外部集成方对错误码、分页和事件结构理解成本高 | 补充 response schema、示例、错误码和 WebSocket payload 契约 |

## 推荐路线

### 阶段 A：发布 gate 固化

- 保持 PR/push 必跑 CI：静态发布 gate 和前端 E2E。
- 每次进入预生产前执行 `python scripts/preflight_check.py`。
- 发布证据统一归档到 `.release-evidence/<name>`，manifest 中必须区分 passed、failed 和 skipped。
- 让 OpenAPI、RBAC、WebSocket、callback、frontend API gate 成为变更评审的必要证据。

### 阶段 B：生产部署与 secret 治理

- 明确生产部署拓扑、TLS 终止、反向代理头、CORS、镜像 tag、资源限制和健康检查策略。
- 通过部署平台注入 secret，不在仓库、日志或文档中记录真实值。
- 明确数据库迁移策略：生产使用显式 `cmd/migrate`，禁止启动时自动迁移。

### 阶段 C：运维观测闭环

- 接入集中日志，至少保留 timestamp、service、level、requestId、method、path、status、latencyMs、user/role 和 error 摘要。
- 接入指标和告警：ready、health、5xx、P95、WebSocket dropped broadcast、analytics delivery、DB 容量增长。
- 将 runtime observability snapshot 作为发布后验证和巡检证据。

### 阶段 D：业务正确性加固

- 补充告警确认、调度状态流转、船舶删除、轨迹查询、battle session 生命周期的并发与幂等测试。
- 针对 analytics retry、重复雷达报告、WebSocket 重连、慢客户端剔除继续扩展回归。
- 对高增长表建立保留窗口、索引策略和清理演练。

### 阶段 E：前端体验与性能治理

- 修复剩余中文乱码文案。
- 继续完善 loading、empty、error、retry 和 requestId 展示规范。
- 基于真实访问路径监控 bundle、首屏耗时和地图页面性能。

## 最小发布 Gate

```bash
python scripts/preflight_check.py
```

需要浏览器回归：

```bash
python scripts/preflight_check.py --only frontend-e2e
```

需要运行时证据：

```bash
python scripts/runtime_precheck.py
docker compose up --build -d
python scripts/smoke_check.py
python scripts/run_runtime_observability_snapshot.py
```

需要数据恢复证据：

```bash
python scripts/run_backup_restore_drill.py
```
