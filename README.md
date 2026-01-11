# Virtual Box Verifier

> US Virtual Mailbox Address Aggregator & Validator

[English](#english) | [中文](#中文)

---

## English

### Overview

A full-stack application that scrapes, validates, and manages US virtual mailbox addresses from multiple providers. The system validates addresses via the Smarty API to classify them as CMRA (Commercial Mail Receiving Agency) and RDI (Residential Delivery Indicator).

### Features

- **Multi-Source Scraping**: ATMB (~2,000 locations) and iPost1 (~4,000 locations)
- **Address Validation**: Smarty API integration with batch processing (100 addresses/request)
- **Dashboard**: Filter, search, and export mailbox data
- **Analytics**: Charts showing RDI distribution, state breakdown, and source distribution
- **Reprocessing**: Re-parse stored HTML without re-fetching (15x faster iteration)

### Tech Stack

| Layer | Technology |
|-------|------------|
| Frontend | React 19 + TypeScript + Vite + TanStack Query |
| Backend | Go 1.25 + Gin Framework |
| Database | Firebase Firestore |
| Validation | Smarty Street API |
| Automation | chromedp (Cloudflare bypass) |

### Quick Start

#### Backend

```bash
cd apps/api

# Create environment file
cat > .env.local <<'EOF'
PORT=8080
GIN_MODE=debug
ALLOWED_ORIGINS=http://localhost:5173

# Firebase
FIREBASE_PROJECT_ID=your-project-id
FIREBASE_CREDS_FILE=service-account.json

# Smarty (set SMARTY_MOCK=true to skip real API calls)
SMARTY_AUTH_ID=your-smarty-id
SMARTY_AUTH_TOKEN=your-smarty-token
SMARTY_MOCK=true

# Crawler
CRAWLER_CONCURRENCY=5
EOF

# Run server
env $(cat .env.local | xargs) go run ./cmd/server
```

#### Frontend

```bash
cd apps/web
npm install
npm run dev
```

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/healthz` | Health check |
| GET | `/api/mailboxes` | List with filters & pagination |
| GET | `/api/mailboxes/export` | CSV export |
| GET | `/api/stats` | Dashboard metrics |
| POST | `/api/crawl/run` | Start ATMB crawl |
| POST | `/api/crawl/ipost1/run` | Start iPost1 crawl |
| POST | `/api/crawl/reprocess` | Re-parse from stored HTML |
| GET | `/api/crawl/status?runId=X` | Job status |
| GET | `/api/crawl/runs` | Job history |

### Deployment

| Service | Platform | Notes |
|---------|----------|-------|
| Frontend | Vercel | Set `VITE_API_URL` |
| Backend | Render | Set env vars, build: `go build -o server cmd/server/main.go` |
| Database | Firebase | Free tier: 50K reads/day |

### Documentation

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for detailed technical documentation.

---

## 中文

### 概述

一个全栈应用，用于抓取、验证和管理美国虚拟邮箱地址。系统通过 Smarty API 验证地址，并将其分类为 CMRA（商业邮件接收机构）和 RDI（住宅配送指示器）。

### 功能特性

- **多源抓取**: ATMB (~2,000 个地点) 和 iPost1 (~4,000 个地点)
- **地址验证**: Smarty API 集成，支持批量处理 (100 个地址/请求)
- **管理面板**: 过滤、搜索和导出邮箱数据
- **数据分析**: RDI 分布、州分布和数据源分布图表
- **重处理**: 从存储的 HTML 重新解析，无需重新抓取 (迭代速度提升 15 倍)

### 技术栈

| 层级 | 技术 |
|------|------|
| 前端 | React 19 + TypeScript + Vite + TanStack Query |
| 后端 | Go 1.25 + Gin 框架 |
| 数据库 | Firebase Firestore |
| 验证 | Smarty Street API |
| 自动化 | chromedp (绕过 Cloudflare) |

### 快速开始

#### 后端

```bash
cd apps/api

# 创建环境配置文件
cat > .env.local <<'EOF'
PORT=8080
GIN_MODE=debug
ALLOWED_ORIGINS=http://localhost:5173

# Firebase 配置
FIREBASE_PROJECT_ID=your-project-id
FIREBASE_CREDS_FILE=service-account.json

# Smarty 配置 (设置 SMARTY_MOCK=true 跳过真实 API 调用)
SMARTY_AUTH_ID=your-smarty-id
SMARTY_AUTH_TOKEN=your-smarty-token
SMARTY_MOCK=true

# 爬虫配置
CRAWLER_CONCURRENCY=5
EOF

# 启动服务
env $(cat .env.local | xargs) go run ./cmd/server
```

#### 前端

```bash
cd apps/web
npm install
npm run dev
```

### API 端点

| 方法 | 端点 | 描述 |
|------|------|------|
| GET | `/healthz` | 健康检查 |
| GET | `/api/mailboxes` | 列表查询（支持过滤和分页） |
| GET | `/api/mailboxes/export` | CSV 导出 |
| GET | `/api/stats` | 仪表盘统计 |
| POST | `/api/crawl/run` | 启动 ATMB 爬虫 |
| POST | `/api/crawl/ipost1/run` | 启动 iPost1 爬虫 |
| POST | `/api/crawl/reprocess` | 从存储的 HTML 重新解析 |
| GET | `/api/crawl/status?runId=X` | 任务状态 |
| GET | `/api/crawl/runs` | 任务历史 |

### 部署

| 服务 | 平台 | 说明 |
|------|------|------|
| 前端 | Vercel | 设置 `VITE_API_URL` 环境变量 |
| 后端 | Render | 设置环境变量，构建命令: `go build -o server cmd/server/main.go` |
| 数据库 | Firebase | 免费额度: 50K 读取/天 |

### 环境变量说明

| 变量 | 说明 | 示例 |
|------|------|------|
| `PORT` | 服务端口 | `8080` |
| `GIN_MODE` | Gin 模式 (debug/release) | `debug` |
| `ALLOWED_ORIGINS` | CORS 允许的来源 | `http://localhost:5173` |
| `FIREBASE_PROJECT_ID` | Firebase 项目 ID | `your-project-id` |
| `FIREBASE_CREDS_FILE` | 本地凭证文件路径 | `service-account.json` |
| `FIREBASE_CREDS_BASE64` | 线上凭证 (Base64 编码) | - |
| `SMARTY_AUTH_ID` | Smarty 认证 ID (多个用逗号分隔) | `id1,id2` |
| `SMARTY_AUTH_TOKEN` | Smarty 认证令牌 (多个用逗号分隔) | `token1,token2` |
| `SMARTY_MOCK` | 是否使用模拟模式 | `true` |
| `CRAWLER_CONCURRENCY` | 爬虫并发数 (Render 免费版建议 5) | `5` |

### 文档

详细技术文档请参阅 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)。

---

## License

MIT
