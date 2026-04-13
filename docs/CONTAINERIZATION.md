# Containerization Plan

> 将 Go API + React 前端打包为 Docker 镜像，并通过 docker-compose 在本地一键编排运行。  
> **基于实际代码（非文档）逆向分析**：`cmd/server/main.go`、`internal/platform/config/`、`internal/platform/http/router.go`、`internal/business/crawler/ipost1/client.go`。

---

## 目录

1. [整体思路与约束](#1-整体思路与约束)
2. [必须的代码改动（chromedp 沙箱）](#2-必须的代码改动chromedp-沙箱)
3. [新增文件结构](#3-新增文件结构)
4. [后端 Dockerfile](#4-后端-dockerfile--appsapiDockerfile)
5. [前端 Dockerfile](#5-前端-dockerfile--appswebDockerfile)
6. [docker-compose.yml](#6-docker-composeyml)
7. [.dockerignore 文件](#7-dockerignore-文件)
8. [环境变量完整参考](#8-环境变量完整参考)
9. [构建与启动](#9-构建与启动)
10. [Makefile 集成](#10-makefile-集成)
11. [生产注意事项](#11-生产注意事项)
12. [故障排查](#12-故障排查)

---

## 1. 整体思路与约束

| 组件 | 策略 | Builder 镜像 | Runtime 镜像 |
|------|------|-------------|-------------|
| Go API (`apps/api`) | 多阶段静态编译 | `golang:1.25-alpine` | `alpine:3.21` |
| React 前端 (`apps/web`) | pnpm workspace 构建 + Nginx 托管 | `node:20-alpine` | `nginx:1.27-alpine` |

### 关键约束（来自代码分析）

**1. chromedp 需要 Chromium 运行时**  
`ipost1/client.go` 通过 `chromedp.NewExecAllocator()` 启动 headless Chromium 进程（见 [ipost1/client.go:39–46](../apps/api/internal/business/crawler/ipost1/client.go)）。Alpine runtime 镜像必须安装 Chromium 及其系统依赖。

**2. 容器内 Chromium 必须禁用沙箱**  
Docker 容器的 Linux 内核不支持 SUID sandbox，必须加 `--no-sandbox`（详见第 2 节）。

**3. 启动时强依赖 Firestore**  
`main.go` 在启动时调用 `firestoreclient.Ping()`（5 秒超时），Firestore 不可达则进程退出。Firebase 凭证是必需的。

**4. 5 级配置层叠**  
`crawler_config.go` 依次从代码默认值 → `config.yaml`（CWD，可选）→ 环境变量 → Firestore DB → 运行时 per-job 参数加载配置。Docker 中 `config.yaml` 不存在时静默跳过，使用代码默认值。

**5. 健康检查端点**  
路由注册为 `GET /healthz`（[router.go:55](../apps/api/internal/platform/http/router.go)），返回 `{"status":"ok"}`。

**6. pnpm workspace**  
前端使用 pnpm workspace（根目录 `pnpm-lock.yaml`），Dockerfile 需用 `corepack enable` + `pnpm --filter us-virtual-address-verifier build`。

---

## 2. 必须的代码改动（chromedp 沙箱）

**这是 iPost1 爬取在 Docker 中能否运行的前提。**

### 问题根源

`ipost1/client.go` 的 `NewClient()` 创建 chromedp allocator 时未包含 `--no-sandbox`（[client.go:39–44](../apps/api/internal/business/crawler/ipost1/client.go)）：

```go
opts := append(chromedp.DefaultExecAllocatorOptions[:],
    chromedp.Flag("headless", true),
    chromedp.Flag("disable-blink-features", "AutomationControlled"),
    chromedp.UserAgent("..."),
    chromedp.WindowSize(1920, 1080),
    // ← 没有 --no-sandbox，没有 --disable-dev-shm-usage
)
```

Docker 容器内运行 Chromium 不加 `--no-sandbox` 会立即崩溃，报错：
```
Running as root without --no-sandbox is not supported. See https://crbug.com/638180.
```

此外，Docker 默认的 `/dev/shm` 大小（64MB）远小于 Chrome 需求，需要 `--disable-dev-shm-usage`。

### 修改方案

在 `apps/api/internal/business/crawler/ipost1/client.go` 的 `NewClient()` 中，通过环境变量 `CHROME_NO_SANDBOX` 按运行环境动态注入标志：

```go
import (
    "context"
    "os"      // ← 新增
    // ... 其余 import 不变
)

func NewClient() (*Client, error) {
    opts := append(chromedp.DefaultExecAllocatorOptions[:],
        chromedp.Flag("headless", true),
        chromedp.Flag("disable-blink-features", "AutomationControlled"),
        chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
        chromedp.WindowSize(1920, 1080),
    )

    // Docker 环境下必须禁用内核沙箱，并使用内存而非 /dev/shm
    // 本地 macOS 开发不设置此变量，无需改变开发体验
    if os.Getenv("CHROME_NO_SANDBOX") == "true" {
        opts = append(opts,
            chromedp.Flag("no-sandbox", true),
            chromedp.Flag("disable-dev-shm-usage", true),
        )
    }

    allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
    // ... 其余代码不变
```

> **为什么用环境变量？**  
> 本地 macOS 开发时，Chromium 沙箱是安全特性，不应禁用。`CHROME_NO_SANDBOX=true` 只在 Docker 环境注入，不影响现有开发流程。

---

## 3. 新增文件结构

容器化完成后，仓库新增以下文件：

```
virtualbox-non-cmra-verifier/
├── apps/
│   ├── api/
│   │   ├── Dockerfile          ← 新增
│   │   └── .dockerignore       ← 新增
│   └── web/
│       ├── Dockerfile          ← 新增
│       └── .dockerignore       ← 新增
├── docker-compose.yml          ← 新增
└── .env.docker.example         ← 新增（提交到 git，作为模板）
    .env.docker                 ← 本地填写真实值，已在 .gitignore 排除
```

---

## 4. 后端 Dockerfile — `apps/api/Dockerfile`

```dockerfile
# ============================================================
# Stage 1: Build — 编译 Go 静态二进制
# ============================================================
FROM golang:1.25-alpine AS builder

WORKDIR /app

# 先复制 go.mod/go.sum 利用层缓存，避免每次重新下载依赖
# 注意：go.mod 声明 go 1.25.3，必须用 golang:1.25，不能用 1.24
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download

# 复制全部源码（build context 为仓库根目录）
COPY apps/api/ .

# CGO_ENABLED=0：静态编译，不依赖 libc（Alpine 用 musl，glibc 二进制无法运行）
# -ldflags="-w -s"：去除调试符号，缩小二进制体积约 30%
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o server \
    ./cmd/server/main.go

# ============================================================
# Stage 2: Runtime — 内置 Chromium 的最小化运行时
# ============================================================
FROM alpine:3.21

WORKDIR /app

# chromedp 运行时依赖说明：
#   chromium          — Headless Chrome，iPost1 爬虫的核心依赖
#   nss               — NSS 密码库，Chromium 的 TLS 依赖
#   freetype          — 字体渲染库（Chromium 页面渲染）
#   harfbuzz          — Unicode 文本整形（Chromium 必需）
#   ca-certificates   — 系统根证书，保证 Smarty API / Firestore HTTPS 正常
#   tzdata            — 时区数据（revalidation checker 按 UTC 2AM 调度）
RUN apk update && apk add --no-cache \
    chromium \
    nss \
    freetype \
    harfbuzz \
    ca-certificates \
    tzdata && \
    rm -rf /var/cache/apk/*

# Alpine 的 Chromium 安装路径与 Debian/Ubuntu 不同
# 显式设置，确保 chromedp 的 ExecAllocator 找到正确路径
ENV CHROME_PATH=/usr/bin/chromium-browser

# 从构建阶段复制静态二进制
COPY --from=builder /app/server .

# 非 root 用户运行（最小权限原则）
# Chromium 加了 --no-sandbox 后可以非 root 运行
RUN addgroup -S appgroup && adduser -S appuser -G appgroup && \
    chown appuser:appgroup /app/server
USER appuser

EXPOSE 8080

CMD ["./server"]
```

---

## 5. 前端 Dockerfile — `apps/web/Dockerfile`

```dockerfile
# ============================================================
# Stage 1: Build — pnpm workspace 构建 React + Vite SPA
# ============================================================
FROM node:20-alpine AS builder

WORKDIR /app

# 启用 corepack，使用项目配套的 pnpm 版本（项目使用 pnpm workspace）
RUN corepack enable

# 复制 workspace 配置和锁文件（build context 为仓库根目录）
# pnpm-lock.yaml 在仓库根目录，不在 apps/web/
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web/package.json ./apps/web/

# 仅安装 web app 依赖（--filter 按 package.json 中的 name 字段过滤）
# apps/web/package.json: "name": "us-virtual-address-verifier"
RUN pnpm install --frozen-lockfile --filter=us-virtual-address-verifier

# 复制 web app 全部源码
COPY apps/web/ ./apps/web/

# VITE_API_URL 是构建时变量，Vite 会将其打包进 JS bundle
# 值必须是浏览器可访问的地址（见第 11 节说明）
ARG VITE_API_URL
ENV VITE_API_URL=$VITE_API_URL

RUN pnpm --filter us-virtual-address-verifier build

# ============================================================
# Stage 2: Runtime — Nginx 托管静态文件
# ============================================================
FROM nginx:1.27-alpine

RUN rm -rf /usr/share/nginx/html/*

# 复制 Vite 构建产物（输出目录为 apps/web/dist）
COPY --from=builder /app/apps/web/dist /usr/share/nginx/html

# SPA 路由配置：任何路径未匹配时回落到 index.html（防刷新 404）
RUN printf 'server {\n\
    listen 80;\n\
    server_name localhost;\n\
    root /usr/share/nginx/html;\n\
    index index.html;\n\
    location / {\n\
        try_files $uri $uri/ /index.html;\n\
    }\n\
    location ~* \\.(js|css|png|ico|svg|woff2?)$ {\n\
        expires 1y;\n\
        add_header Cache-Control "public, immutable";\n\
    }\n\
}\n' > /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
```

---

## 6. docker-compose.yml

```yaml
# docker-compose.yml（仓库根目录）
# Docker Compose v2，不需要 version 字段

services:

  # ─────────────────────────────────────────────────────────
  # Go API 后端
  # ─────────────────────────────────────────────────────────
  api:
    build:
      context: .                        # 构建上下文为仓库根目录
      dockerfile: apps/api/Dockerfile
    ports:
      - "8080:8080"
    environment:
      # ── 服务器基础 ─────────────────────────────────────────
      PORT: "8080"
      GIN_MODE: release
      # 浏览器请求 API 时携带的 Origin 是 http://localhost（前端 Nginx 80→80）
      ALLOWED_ORIGINS: "http://localhost"

      # ── Chromium（Docker 环境必须，本地开发不设置）─────────
      CHROME_NO_SANDBOX: "true"

      # ── Firebase ──────────────────────────────────────────
      # 启动时强依赖：main.go 会 Ping Firestore，失败则退出
      FIREBASE_PROJECT_ID: ${FIREBASE_PROJECT_ID}
      FIREBASE_CREDS_BASE64: ${FIREBASE_CREDS_BASE64}

      # ── Smarty API ────────────────────────────────────────
      SMARTY_AUTH_ID: ${SMARTY_AUTH_ID}
      SMARTY_AUTH_TOKEN: ${SMARTY_AUTH_TOKEN}
      SMARTY_MOCK: ${SMARTY_MOCK:-false}

      # ── 爬虫基础配置 ───────────────────────────────────────
      CRAWLER_CONCURRENCY: "5"
      USE_AGGRESSIVE_SCRAPING: "true"
      # CRAWL_LINK_SEEDS 可选；不设置则通过 POST /api/crawl/run body 传入 links

      # ── 后台 Worker（默认关闭）────────────────────────────
      ENABLE_VALIDATION_WORKERS: ${ENABLE_VALIDATION_WORKERS:-false}
      ENABLE_REVALIDATION_CHECKER: ${ENABLE_REVALIDATION_CHECKER:-false}

    restart: unless-stopped
    healthcheck:
      # 端点来自 router.go:55: router.GET("/healthz", ...)
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/healthz"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 20s   # Firestore Ping 最多 5s，留足启动余量

  # ─────────────────────────────────────────────────────────
  # React 前端
  # ─────────────────────────────────────────────────────────
  web:
    build:
      context: .
      dockerfile: apps/web/Dockerfile
      args:
        # 浏览器通过宿主机端口映射访问 API（localhost:8080）
        # 不是容器间的 http://api:8080（浏览器无法解析）
        VITE_API_URL: ${VITE_API_URL:-http://localhost:8080}
    ports:
      - "80:80"
    depends_on:
      - api
    restart: unless-stopped
```

---

## 7. `.dockerignore` 文件

### `apps/api/.dockerignore`

```
# 本地凭证和环境文件
.env
.env.local
*.env.*
service-account.json

# 测试数据（testdata/ 含 rawHTML 样本，可能很大）
testdata/
**/*_test.go

# 编译产物
server

# 不需要进入构建上下文的文件
.git
*.md
scripts/
```

### `apps/web/.dockerignore`

```
node_modules/
dist/
.env
.env.local
*.env.*
.vite/
```

---

## 8. 环境变量完整参考

基于对 `config/config.go` 和 `config/crawler_config.go` 的实际代码分析，共两组配置。

### 8a. 服务器主配置（`config.go`）

| 变量 | 是否必需 | 默认值 | 说明 |
|------|---------|--------|------|
| `PORT` | 否 | `8080` | HTTP 监听端口 |
| `GIN_MODE` | 否 | `release` | `debug` 或 `release` |
| `FIREBASE_PROJECT_ID` | **必需** | — | Firebase 项目 ID |
| `FIREBASE_CREDS_BASE64` | **必需**¹ | — | service-account.json 的 Base64 编码 |
| `FIREBASE_CREDS_FILE` | **必需**¹ | — | service-account.json 文件路径（本地用） |
| `SMARTY_AUTH_ID` | **必需**² | — | Smarty API auth ID，多账号逗号分隔 |
| `SMARTY_AUTH_TOKEN` | **必需**² | — | Smarty API token，数量必须与 ID 一致 |
| `SMARTY_MOCK` | 否 | `false` | `true` 跳过真实 API，返回固定 CMRA=Y |
| `ALLOWED_ORIGINS` | 否 | — | CORS 允许的 Origin，逗号分隔 |
| `CRAWL_LINK_SEEDS` | 否 | — | ATMB 爬虫种子 URL，逗号分隔 |
| `USE_AGGRESSIVE_SCRAPING` | 否 | `true` | 启用激进爬取策略 |
| `ENABLE_VALIDATION_WORKERS` | 否 | `false` | 每 5 分钟自动验证 pending 地址 |
| `ENABLE_REVALIDATION_CHECKER` | 否 | `false` | 每天 02:00 UTC 标记需重验地址 |
| `CRAWLER_CONCURRENCY` | 否 | `5` | ATMB 并发 worker 数量 |

¹ `FIREBASE_CREDS_BASE64` 或 `FIREBASE_CREDS_FILE` 二选一  
² `SMARTY_MOCK=true` 时 Smarty 凭证非必需

### 8b. 爬虫精细配置（`crawler_config.go`，均可选）

这些变量是 5 级配置层叠中的第 3 级（优先级高于代码默认值，低于 Firestore DB 配置）。

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `CRAWLER_WORKER_COUNT` | `5` | 爬虫并发 worker 数 |
| `CRAWLER_SCRAPE_BATCH_SIZE` | `20` | 批量写入 Firestore 的记录数 |
| `CRAWLER_SCRAPE_TIMEOUT` | `30m` | 单次爬取超时，Go duration 格式 |
| `CRAWLER_VALIDATION_BATCH_SIZE` | `100` | Smarty 单批验证地址数（上限 100）|
| `CRAWLER_VALIDATION_WORKER_COUNT` | `3` | 验证并发 worker 数 |
| `CRAWLER_MAX_RETRY_ATTEMPTS` | `5` | 验证失败最大重试次数 |
| `CRAWLER_RETRY_BACKOFF_BASE` | `1s` | 重试退避基础时间 |
| `CRAWLER_RETRY_BACKOFF_MULTIPLIER` | `2.0` | 退避时间乘数 |
| `CRAWLER_RETRY_BACKOFF_JITTER` | `0.1` | 退避抖动系数（0–1）|
| `CRAWLER_REVALIDATION_INTERVAL` | `2160h`（90天）| 地址强制重验周期 |
| `CRAWLER_REVALIDATION_THRESHOLD` | `1680h`（70天）| 开始提升重验优先级的阈值 |
| `CRAWLER_DAILY_VALIDATION_BUDGET` | `10000` | 每日验证配额上限 |
| `CRAWLER_HIGH_PRIORITY_QUOTA` | `5000` | 高优先级配额（新地址）|
| `CRAWLER_MEDIUM_PRIORITY_QUOTA` | `3000` | 中优先级配额（临近重验）|

### 8c. 本地 `.env.docker` 模板

```bash
# .env.docker.example（提交到 git）
# 使用前复制为 .env.docker 并填写真实值

# ── Firebase ──────────────────────────────────────────────
FIREBASE_PROJECT_ID=your-project-id

# 生成方式（macOS/Linux）：
# base64 -i apps/api/service-account.json | tr -d '\n'
FIREBASE_CREDS_BASE64=

# ── Smarty API ────────────────────────────────────────────
# 多账号负载均衡：SMARTY_AUTH_ID=id1,id2  SMARTY_AUTH_TOKEN=tok1,tok2
SMARTY_AUTH_ID=
SMARTY_AUTH_TOKEN=
# 本地测试可开启 mock 模式，跳过真实 API
SMARTY_MOCK=false

# ── 前端 API 地址 ─────────────────────────────────────────
# 浏览器访问地址（通过宿主机端口映射）
VITE_API_URL=http://localhost:8080

# ── 可选功能 ──────────────────────────────────────────────
ENABLE_VALIDATION_WORKERS=false
ENABLE_REVALIDATION_CHECKER=false
```

---

## 9. 构建与启动

```bash
# 1. 创建本地环境变量文件
cp .env.docker.example .env.docker
# 编辑 .env.docker，填入 FIREBASE_PROJECT_ID、FIREBASE_CREDS_BASE64、SMARTY_* 等

# 2. 构建并启动（前台，方便查看日志）
docker compose --env-file .env.docker up --build

# 3. 访问
# 前端：http://localhost
# API：http://localhost:8080/healthz
```

### 常用命令

```bash
# 后台运行
docker compose --env-file .env.docker up -d --build

# 只重建 API，不动前端（代码改动后）
docker compose --env-file .env.docker up -d --build api

# 实时查看 API 日志
docker compose logs -f api

# 停止（保留镜像）
docker compose down

# 完全清理（删除镜像和缓存）
docker compose down --rmi local --volumes
```

### 快速验证

```bash
# API 健康检查
curl http://localhost:8080/healthz
# 期望输出：{"status":"ok"}

# 前端
open http://localhost
```

---

## 10. Makefile 集成

在现有 `Makefile` 末尾追加：

```makefile
# ── Docker 相关命令 ──────────────────────────────────────────

docker-build: ## 构建所有 Docker 镜像
	@echo "🐳 构建 Docker 镜像..."
	docker compose --env-file .env.docker build

docker-up: ## 启动所有容器（后台运行）
	@echo "🚀 启动容器..."
	docker compose --env-file .env.docker up -d

docker-down: ## 停止并删除容器
	@echo "🛑 停止容器..."
	docker compose down

docker-logs: ## 查看 API 日志（实时）
	docker compose logs -f api

docker-rebuild-api: ## 仅重新构建并重启 API 容器
	@echo "🔄 重新构建 API..."
	docker compose --env-file .env.docker up -d --build api

docker-clean: ## 清理所有容器、镜像和构建缓存
	@echo "🧹 清理 Docker 资源..."
	docker compose down --rmi local --volumes
	docker builder prune -f
```

---

## 11. 生产注意事项

### VITE_API_URL 的正确设置

`VITE_API_URL` 是**构建时**变量，Vite 在编译时将其替换进 JS bundle。它的值必须是**浏览器**能访问的地址，而非容器间通信地址：

| 场景 | 正确值 | 错误值 |
|------|--------|--------|
| 本地 docker-compose | `http://localhost:8080` | `http://api:8080`（容器内网，浏览器无法访问）|
| Render + Vercel 生产 | `https://your-api.onrender.com` | `http://api:8080` |
| 自托管同域反向代理 | `/api`（Nginx 代理到后端）| — |

### Chromium 内存需求

iPost1 爬取期间内存消耗估算：

| 组件 | 内存占用 |
|------|---------|
| Go API 进程 | ~50MB |
| Chromium headless（单实例，30min 会话）| ~200–400MB |
| 峰值总计 | **~450MB** |

**建议**：容器至少配置 1GB 内存。如需在 docker-compose 中限制：

```yaml
services:
  api:
    mem_limit: 1g
```

### Firebase 凭证安全

- 使用 `FIREBASE_CREDS_BASE64` 而非文件挂载（无需维护 `.secrets/` 目录）
- `.env.docker` 已在 `.gitignore` 中排除，不要提交
- CI/CD 环境通过 Secrets / Environment Variables 注入，不写入任何配置文件

---

## 12. 故障排查

### iPost1 爬取后 API 容器崩溃

**症状**：`POST /api/crawl/ipost1/run` 成功，但任务状态随即变为 `failed`，日志出现 Chromium 相关报错。

**原因**：[第 2 节](#2-必须的代码改动chromedp-沙箱)代码改动未完成，或 `CHROME_NO_SANDBOX` 未注入。

```bash
# 验证环境变量是否正确传入
docker compose exec api env | grep CHROME
# 期望：CHROME_NO_SANDBOX=true
#       CHROME_PATH=/usr/bin/chromium-browser
```

---

### API 容器启动后立即退出（exit code 1）

**检查日志**：
```bash
docker compose logs api
```

**常见原因**：

| 日志关键词 | 原因 | 解决 |
|-----------|------|------|
| `config load: FIREBASE_PROJECT_ID is required` | 环境变量未传入 | 检查 `.env.docker` 是否有该变量 |
| `config load: provide FIREBASE_CREDS_BASE64 or FIREBASE_CREDS_FILE` | Firebase 凭证缺失 | 生成 Base64 凭证并填入 |
| `config load: SMARTY_AUTH_ID count ... must match SMARTY_AUTH_TOKEN count` | 凭证数量不匹配 | 检查逗号分隔的数量是否一致 |
| `firestore ping:` | Firestore 网络不通 | 检查 Firebase project ID 和凭证 JSON 是否正确 |

---

### 前端构建失败

**症状**：`web` 服务 build 阶段报 `Cannot find module` 或 `Workspace not found`。

**检查**：Dockerfile 中的复制顺序必须如下（缺一不可）：

```dockerfile
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./  # 根目录 workspace 文件
COPY apps/web/package.json ./apps/web/                   # web app 的 package.json
RUN pnpm install --frozen-lockfile --filter=us-virtual-address-verifier
COPY apps/web/ ./apps/web/                               # 源码最后复制（利用缓存层）
```

---

### 浏览器 API 请求 CORS 错误

**原因**：前端 Nginx 默认在 80 端口，浏览器发出请求时 `Origin` 是 `http://localhost`（无端口）。

**检查 docker-compose.yml 中**：
```yaml
ALLOWED_ORIGINS: "http://localhost"
```

如果前端映射到非 80 端口（如 `3000:80`），需改为 `http://localhost:3000`。

---

### `docker compose` 找不到 `.env.docker`

**原因**：直接运行 `docker compose up` 未指定 `--env-file`。

**解决**：始终带上参数：
```bash
docker compose --env-file .env.docker up
```

或将变量写入 `.env`（Docker Compose 默认加载），但 `.env` 文件已在 `.gitignore` 中，需要自行创建。

---

*文档版本：2026-04-12 | 基于代码分析：`main.go`、`config.go`、`crawler_config.go`、`router.go`、`ipost1/client.go`*
