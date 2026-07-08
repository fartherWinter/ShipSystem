# ShipSystem 运维可观测性方案

## 目标

当前阶段先复用已有健康检查、requestId、运行时 smoke、runtime observability snapshot 和发布证据目录，形成发布前和巡检证据。正式生产环境应在此基础上接入集中日志、指标、dashboard 和主动告警。

## 已有观测入口

| 入口 | 用途 |
| --- | --- |
| `GET /health` | Go API 进程存活 |
| `GET /ready` | Go API 与数据库连通性 |
| analytics `GET /health` | Python 服务存活 |
| frontend `/` | 前端静态资源入口 |
| `/api/v1/analytics/simulate/status` | analytics 状态和 delivery metrics |
| `X-Request-ID` | 串联前端、Go API、analytics 和错误响应 |
| `scripts/smoke_check.py` | 端到端运行时冒烟 |
| `scripts/run_runtime_observability_snapshot.py` | 只读运行时快照 |

## 发布与巡检证据

- CI 静态证据：`.release-evidence/ci-static`
- CI 前端 E2E 证据：`.release-evidence/ci-frontend-e2e`
- 运维巡检证据：`.release-evidence/operations`
- 手动发布证据：`.release-evidence/<release-name>`

manifest 要求：

- 实际执行的步骤写入 `steps`。
- 跳过的步骤写入 `skippedSteps`。
- 每个步骤有命令、工作目录、输出文件、退出码和状态。

## 日志字段

生产日志至少保留：

- `timestamp`
- `service`：`backend`、`analytics`、`frontend-nginx`
- `level`
- `requestId`
- `method`
- `path`
- `status`
- `latencyMs`
- `userId` 或角色摘要
- `error`

禁止记录：

- token
- 密码
- 完整 Cookie
- 私钥
- 真实 secret 值
- 包含敏感字段的完整请求体

排障时以 `requestId` 为主线：前端错误提示、Go API 访问日志、Go 调 analytics、analytics 回调 Go API 都应该能通过同一个 requestId 关联。

## 指标建议

第一阶段优先落地：

- backend `/ready` 成功率和延迟。
- analytics `/health` 成功率和延迟。
- REST 请求量、4xx、5xx、P50、P95、P99。
- 登录、船舶列表、告警列表、调度列表、battle 状态等核心接口 P95。
- WebSocket active clients、broadcast queue depth、dropped broadcasts。
- analytics delivery success、failure、retry、dropped、last error。
- 数据增长：`ship_locations`、`battle_snapshots`、`battle_events`、`radar_targets` 行数。
- retention preview 匹配行数和清理耗时。
- backup/restore drill 耗时、备份大小和恢复校验结果。

## 告警规则

建议初始阈值：

| 告警 | 初始阈值 |
| --- | --- |
| backend ready 失败 | 连续 3 次失败 |
| analytics health 失败 | 连续 3 次失败 |
| API 5xx | 5 分钟窗口超过 2% |
| 核心接口 P95 | 5 分钟窗口超过 1s |
| analytics delivery failure | 持续增长，或最近一次失败超过 5 分钟未恢复 |
| WebSocket dropped broadcasts | 持续增长 |
| smoke 或 runtime snapshot | 任一步骤失败 |
| retention preview | 待清理行数超过容量预算 |
| capacity estimate | radar/battle 日增长超过保留窗口可承受范围 |

阈值必须在预生产和生产采样后调整，不能长期使用默认值。

## Dashboard 结构

第一版 dashboard 保持运维视角，不做业务大屏：

- 服务健康：backend health、backend ready、analytics health、frontend index。
- API 质量：请求量、错误率、P50/P95/P99、Top 5 错误接口。
- 实时链路：WebSocket active clients、dropped broadcasts、慢客户端剔除。
- analytics 链路：simulate status、delivery success/failure/retry、最近失败 requestId。
- 数据容量：高增长表行数趋势、retention preview、capacity estimate。
- 发布证据：最近一次 CI、E2E、operations gate 的 manifest 状态。

## 运行时快照

手动执行：

```bash
python scripts/run_runtime_observability_snapshot.py
```

纳入证据采集：

```bash
python scripts/collect_release_evidence.py --include-runtime-observability-snapshot --output-dir .release-evidence/latest-observability
```

快照失败时优先检查：

- 服务是否启动。
- backend、analytics、frontend URL 环境变量是否指向正确地址。
- 登录账号是否来自当前初始化数据。
- requestId 是否被代理层或服务层丢弃。
- analytics status 是否包含 delivery metrics。

## 容量基线

`scripts/run_capacity_smoke.py --estimate-only` 使用 track 数、tick 数和运行时长估算 battle/radar 数据增长。

当前默认估算关注：

- 5 条轨迹
- 20 条轨迹
- 100 条轨迹

容量决策要求：

- 用真实业务频率校准 `track-counts`、`ticks` 和 `duration-seconds`。
- 用真实查询路径验证索引和分页。
- retention 窗口必须同时考虑恢复需要、审计需要和存储成本。
- 清理任务先执行 preview，再执行真实清理。

## 巡检节奏

建议：

- 每个 PR：静态 gate 和前端 E2E。
- 每次预生产发布：runtime precheck、smoke、runtime observability snapshot。
- 每周：operations gate，包含备份恢复演练、DB integration、retention preview 和 capacity estimate。
- 每次生产事故后：补充可复现 smoke 或契约 gate，避免同类问题再次逃逸。

## 后续落地项

- 接入生产日志采集和查询平台。
- 暴露或采集标准 metrics。
- 建立 dashboard 和告警通知。
- 将备份恢复、retention、capacity 证据纳入周期性运维审计。
- 修复源码中剩余中文乱码文案，并增加 UTF-8 防回归检查。
