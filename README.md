# Virtual Box Verifier

> US Virtual Mailbox Address Aggregator & Validator

[English](#english) | [中文](#中文)

---

## English

### Overview

A full-stack application that scrapes, validates, and manages US virtual mailbox addresses from multiple providers. The system validates addresses via the Smarty API to classify them as CMRA (Commercial Mail Receiving Agency) and RDI (Residential Delivery Indicator).

### Features

- **Multi-Source Scraping**: ATMB (~2,000 locations via goquery) and iPost1 (~4,000 locations via headless Chrome / Cloudflare bypass)
- **Batch Address Validation**: Smarty API with 100 addresses/request, multi-credential load balancing and circuit breaker
- **Async Validation Workers**: Background worker processes pending validations every 5 minutes; daily re-validation checker marks stale addresses (configurable via feature flags)
- **Validation Lifecycle**: Per-address state machine — `pending → validated / failed / retry_scheduled / needs_revalidation / manual_review`
- **Dashboard**: Filter by state, CMRA, RDI, source; CSV export with dynamic filenames
- **Analytics**: Charts for RDI distribution, state breakdown, source distribution
- **Reprocessing**: Re-parse stored HTML without re-fetching (~2 min vs ~30 min re-crawl)
- **5-Level Config Cascade**: Code defaults → `config.yaml` → env vars → Firestore DB → per-job overrides

### Tech Stack

| Layer | Technology |
|-------|------------|
| Frontend | React 19 + TypeScript + Vite + TanStack Query |
| Backend | Go 1.25 + Gin Framework |
| Database | Firebase Firestore |
| Validation | Smarty Street API |
| Browser Automation | chromedp (Cloudflare bypass for iPost1) |

### Quick Start

#### Option A — Docker (recommended)

```bash
# 1. Copy and fill in credentials
cp .env.docker.example .env

# 2. Build and start
docker compose up --build

# Frontend: http://localhost
# API:      http://localhost:8080/healthz
```

See [docs/CONTAINERIZATION.md](docs/CONTAINERIZATION.md) for the full Docker guide.

#### Option B — Local Development

The quickest way is the included `start.sh` (or `make start`), which auto-creates `apps/api/.env.local` if missing, installs dependencies, and starts both services concurrently:

```bash
./start.sh
# or
make start

# API:      http://localhost:8080
# Frontend: http://localhost:5173
# Press Ctrl+C to stop both
```

To start services individually:

**Backend** (from `apps/api/`):

```bash
cd apps/api
# Edit .env.local first (see apps/api/.env.local template created by start.sh)
go run ./cmd/server
```

**Frontend** (from repo root):

```bash
pnpm install
pnpm dev:web    # Vite dev server on :5173
```

### API Endpoints

**Health**

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/healthz` | Health check → `{"status":"ok"}` |

**Mailboxes**

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/mailboxes` | List with filters (`state`, `cmra`, `rdi`, `source`, `active`) and pagination |
| GET | `/api/mailboxes/export` | Streaming CSV export with dynamic filename |
| GET | `/api/stats` | Dashboard metrics (reads 1 singleton document) |
| POST | `/api/stats/refresh` | Recompute and save aggregate stats |

**Crawl Jobs**

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/crawl/run` | Start ATMB crawl (body: `{"links":[...]}`) |
| POST | `/api/crawl/ipost1/run` | Start iPost1 crawl (headless Chrome) |
| POST | `/api/crawl/reprocess` | Re-parse from stored HTML (body: `{"onlyOutdated":true}`) |
| GET | `/api/crawl/status?runId=X` | Poll job status |
| GET | `/api/crawl/runs` | Job history (last 20) |
| POST | `/api/crawl/runs/:runId/cancel` | Cancel a running job |

**Validation**

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/validation/run` | Trigger manual validation of pending addresses |
| GET | `/api/validation/stats` | Validation status breakdown by state |
| GET | `/api/validation/runs` | Validation run history (last 20) |
| GET | `/api/validation/runs/:runId` | Single validation run detail |
| POST | `/api/validation/revalidation/check` | Mark addresses needing re-validation |

### Environment Variables

**Required**

| Variable | Description |
|----------|-------------|
| `FIREBASE_PROJECT_ID` | Firebase project ID |
| `FIREBASE_CREDS_BASE64` | service-account.json as Base64 (production) |
| `FIREBASE_CREDS_FILE` | Path to service-account.json (local dev) |
| `SMARTY_AUTH_ID` | Smarty auth ID(s), comma-separated for multiple accounts |
| `SMARTY_AUTH_TOKEN` | Smarty auth token(s), must match ID count |

**Optional — Server**

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP listen port |
| `GIN_MODE` | `release` | `debug` or `release` |
| `ALLOWED_ORIGINS` | — | CORS allowed origins, comma-separated |
| `SMARTY_MOCK` | `false` | Skip real API calls; returns CMRA=Y, RDI=Commercial |
| `CRAWL_LINK_SEEDS` | — | ATMB seed URLs, comma-separated |
| `CRAWLER_CONCURRENCY` | `5` | Concurrent ATMB worker count (Render free tier limit: 5) |
| `USE_AGGRESSIVE_SCRAPING` | `true` | Enable mark-and-sweep deactivation |
| `ENABLE_VALIDATION_WORKERS` | `false` | Run background validation every 5 minutes |
| `ENABLE_REVALIDATION_CHECKER` | `false` | Daily check at 02:00 UTC for stale addresses |

For the full crawler tuning variables (`CRAWLER_DAILY_VALIDATION_BUDGET`, `CRAWLER_RETRY_*`, etc.), see [docs/CONTAINERIZATION.md §8](docs/CONTAINERIZATION.md#8-环境变量完整参考).

### Pre-Commit Checks

```bash
cd apps/api
go build ./...   # verify all packages compile
go vet ./...     # static analysis
go test ./...    # run tests
```

### Deployment

| Service | Platform | Notes |
|---------|----------|-------|
| Frontend | Vercel | Set `VITE_API_BASE_URL` to the backend public URL |
| Backend | Render | Build: `cd apps/api && go build -o server ./cmd/server/main.go` |
| Database | Firebase Firestore | Free tier: 50K reads / 20K writes per day |

### Documentation

| Document | Description |
|----------|-------------|
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | System architecture, data models, crawl workflows |
| [docs/CONTAINERIZATION.md](docs/CONTAINERIZATION.md) | Docker setup, env var reference, troubleshooting |

---

## 中文

### 概述

一个全栈应用，用于抓取、验证和管理美国虚拟邮箱地址。系统通过 Smarty API 验证地址，将其分类为 CMRA（商业邮件接收机构）和 RDI（住宅配送指示器）。

### 功能特性

- **多源抓取**：ATMB（~2,000 个地点，goquery 静态解析）和 iPost1（~4,000 个地点，headless Chrome 绕过 Cloudflare）
- **批量地址验证**：Smarty API，每次请求 100 个地址；支持多账号负载均衡和熔断器
- **异步验证 Worker**：后台 Worker 每 5 分钟处理 pending 地址；每日重验检查器标记过期地址（通过 feature flag 控制）
- **验证生命周期**：每个地址有独立状态机 —— `pending → validated / failed / retry_scheduled / needs_revalidation / manual_review`
- **管理面板**：按州、CMRA、RDI、数据源过滤；CSV 动态文件名导出
- **数据分析**：RDI 分布、州分布、数据源分布图表
- **重处理**：从存储的 HTML 重新解析，无需重新抓取（约 2 分钟 vs 重爬 30 分钟）
- **5 级配置层叠**：代码默认值 → `config.yaml` → 环境变量 → Firestore DB → 运行时 per-job 参数

### 技术栈

| 层级 | 技术 |
|------|------|
| 前端 | React 19 + TypeScript + Vite + TanStack Query |
| 后端 | Go 1.25 + Gin 框架 |
| 数据库 | Firebase Firestore |
| 验证 | Smarty Street API |
| 浏览器自动化 | chromedp（绕过 Cloudflare，用于 iPost1）|

### 快速开始

#### 方式 A — Docker（推荐）

```bash
# 1. 复制并填写凭证
cp .env.docker.example .env

# 2. 构建并启动
docker compose up --build

# 前端：http://localhost
# API： http://localhost:8080/healthz
```

完整 Docker 说明见 [docs/CONTAINERIZATION.md](docs/CONTAINERIZATION.md)。

#### 方式 B — 本地开发

最简单的方式是使用根目录的 `start.sh`（或 `make start`），会自动创建 `apps/api/.env.local`、安装依赖并并行启动两个服务：

```bash
./start.sh
# 或
make start

# API：      http://localhost:8080
# 前端：     http://localhost:5173
# Ctrl+C 同时停止两个服务
```

单独启动：

**后端**（在 `apps/api/` 目录下）：

```bash
cd apps/api
# 先编辑 .env.local（start.sh 会自动生成模板）
go run ./cmd/server
```

**前端**（从仓库根目录）：

```bash
pnpm install
pnpm dev:web    # Vite 开发服务器，端口 5173
```

### API 端点

**健康检查**

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/healthz` | 健康检查 → `{"status":"ok"}` |

**邮箱数据**

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/api/mailboxes` | 列表查询（支持 `state`、`cmra`、`rdi`、`source`、`active` 过滤和分页）|
| GET | `/api/mailboxes/export` | 流式 CSV 导出，动态文件名 |
| GET | `/api/stats` | 仪表盘统计（读取 1 个单例文档）|
| POST | `/api/stats/refresh` | 重新计算并保存统计数据 |

**爬取任务**

| 方法 | 端点 | 说明 |
|------|------|------|
| POST | `/api/crawl/run` | 启动 ATMB 爬虫（body: `{"links":[...]}`）|
| POST | `/api/crawl/ipost1/run` | 启动 iPost1 爬虫（使用 headless Chrome）|
| POST | `/api/crawl/reprocess` | 从存储的 HTML 重新解析（body: `{"onlyOutdated":true}`）|
| GET | `/api/crawl/status?runId=X` | 轮询任务状态 |
| GET | `/api/crawl/runs` | 任务历史（最近 20 条）|
| POST | `/api/crawl/runs/:runId/cancel` | 取消正在运行的任务 |

**验证**

| 方法 | 端点 | 说明 |
|------|------|------|
| POST | `/api/validation/run` | 手动触发 pending 地址验证 |
| GET | `/api/validation/stats` | 验证状态分布统计 |
| GET | `/api/validation/runs` | 验证任务历史（最近 20 条）|
| GET | `/api/validation/runs/:runId` | 单条验证任务详情 |
| POST | `/api/validation/revalidation/check` | 标记需要重验的地址 |

### 环境变量

**必需**

| 变量 | 说明 |
|------|------|
| `FIREBASE_PROJECT_ID` | Firebase 项目 ID |
| `FIREBASE_CREDS_BASE64` | service-account.json 的 Base64 编码（生产环境）|
| `FIREBASE_CREDS_FILE` | service-account.json 文件路径（本地开发）|
| `SMARTY_AUTH_ID` | Smarty 认证 ID，多账号用逗号分隔 |
| `SMARTY_AUTH_TOKEN` | Smarty 认证令牌，数量必须与 ID 一致 |

**可选 — 服务器**

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PORT` | `8080` | HTTP 监听端口 |
| `GIN_MODE` | `release` | `debug` 或 `release` |
| `ALLOWED_ORIGINS` | — | CORS 允许来源，逗号分隔 |
| `SMARTY_MOCK` | `false` | 跳过真实 API，返回 CMRA=Y、RDI=Commercial |
| `CRAWL_LINK_SEEDS` | — | ATMB 种子 URL，逗号分隔 |
| `CRAWLER_CONCURRENCY` | `5` | ATMB 并发 worker 数（Render 免费版限制为 5）|
| `USE_AGGRESSIVE_SCRAPING` | `true` | 启用标记清除（mark-and-sweep）下架策略 |
| `ENABLE_VALIDATION_WORKERS` | `false` | 每 5 分钟自动验证 pending 地址 |
| `ENABLE_REVALIDATION_CHECKER` | `false` | 每天 02:00 UTC 检查并标记过期地址 |

爬虫精细调参变量（`CRAWLER_DAILY_VALIDATION_BUDGET`、`CRAWLER_RETRY_*` 等）请参阅 [docs/CONTAINERIZATION.md §8](docs/CONTAINERIZATION.md#8-环境变量完整参考)。

### 提交前检查

```bash
cd apps/api
go build ./...   # 验证所有包可以编译
go vet ./...     # 静态分析
go test ./...    # 运行测试
```

### 部署

| 服务 | 平台 | 说明 |
|------|------|------|
| 前端 | Vercel | 设置 `VITE_API_BASE_URL` 为后端公网地址 |
| 后端 | Render | 构建命令：`cd apps/api && go build -o server ./cmd/server/main.go` |
| 数据库 | Firebase Firestore | 免费额度：50K 读取 / 20K 写入每天 |

### 文档

| 文档 | 内容 |
|------|------|
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | 系统架构、数据模型、爬取流程 |
| [docs/CONTAINERIZATION.md](docs/CONTAINERIZATION.md) | Docker 配置、完整环境变量参考、故障排查 |

---

## License

MIT
