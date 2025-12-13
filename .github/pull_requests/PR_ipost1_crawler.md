# Pull Request: iPost1 虚拟邮箱爬虫 & 前端优化

## 概述

本 PR 为 virtualbox-verifier 系统添加 iPost1 虚拟邮箱数据源支持，并对前端进行了全面优化，包括 React Query 迁移、Analytics 页面增强、Crawler Status 页面改进等。

**分支**: `feature/ipost1-cralwer` → `main`

**提交历史** (10 commits):
```
4b0271f feat: improve crawler status page with React Query and stale job handling
9b8f5a5 feat: update the analysts page to add new data and filter by source
3c5efdf feat: clean up iPost1 address
19435c9 feat: add utility to cleanup tags in iPost1 addresses
62ff3c3 docs: document mailbox loading enhancements
928c5f9 feat: improve API fetching logic and adding frontend cache
d4dafb6 feat: add filter to group locations by source and remove html tags from iPost1 locations
81cdcaa docs: add PR summary document for iPost1 crawler feature
7c1c0b4 fix: resolve iPost1 API malformed JSON parsing and complete scraper
b7240bb feat: add features to support iPost1 mailbox
```

---

## 核心功能

### ✅ Part 1: iPost1 爬虫系统

1. **iPost1 爬虫系统**
   - 自动遍历 53 个美国州/地区
   - 从 iPost1 AJAX API 提取邮箱位置数据
   - 绕过 Cloudflare 反爬虫保护
   - 处理畸形 JSON 响应格式

2. **数据源隔离**
   - 新增 `Source` 字段区分数据来源
   - ATMB 数据标记为 `Source="ATMB"`
   - iPost1 数据标记为 `Source="iPost1"`
   - 两个爬虫可独立运行，数据互不干扰

3. **地址清理工具**
   - 移除 iPost1 地址中的 HTML 标签 (`<wbr>`, `<span>` 等)
   - 清理乱码和特殊字符
   - 支持迁移脚本批量处理

4. **API 端点**
   - `POST /api/crawl/ipost1/run` - 启动 iPost1 爬虫

### ✅ Part 2: 前端 React Query 优化

1. **Mailboxes 页面优化**
   - 迁移到 React Query，支持 30 分钟缓存
   - 硬编码州/Source/RDI 选项，减少 API 请求
   - 后端使用 Firestore Count API，O(n) → O(1)

2. **Analytics 页面增强**
   - 迁移到 React Query，修复 StrictMode 双重调用
   - 新增 **Source 分布统计** (ATMB vs iPost1)
   - 新增 **Refresh Stats** 按钮手动刷新
   - 显示 **Last Updated** 时间戳

3. **Crawler Status 页面改进**
   - 迁移到 React Query，智能轮询（仅 running 时轮询）
   - 新增 **自动超时检测**（>45分钟自动标记为 timeout）
   - 新增 **Cancel 按钮** 手动取消运行中的任务
   - 支持 `timeout` 和 `cancelled` 状态显示

---

## 文件变更详情

### 📁 新增文件 (16 个)

#### 核心代码
| 文件 | 说明 |
|------|------|
| `internal/business/crawler/ipost1/client.go` | chromedp 客户端，处理 HTTP 请求和反 Cloudflare |
| `internal/business/crawler/ipost1/discovery.go` | 发现流程，协调州遍历、解析、验证、存储 |
| `internal/business/crawler/ipost1/parser.go` | HTML 解析器，从 API 响应提取邮箱数据 |
| `cmd/migrate-add-source/main.go` | 数据迁移工具，为旧数据添加 Source 字段 |
| `cmd/migrate-clean-addresses/main.go` | 地址清理迁移工具 |
| `pkg/util/address_cleaner.go` | 地址清理工具函数 |
| `apps/web/constants.ts` | 前端硬编码常量（州、Source、RDI） |

#### 配置文件
| 文件 | 说明 |
|------|------|
| `firestore.indexes.json` | Firestore 复合索引配置（按 Source 过滤） |
| `Makefile` | 新增 iPost1 相关命令 |

#### 文档
| 文件 | 说明 |
|------|------|
| `docs/ipost1_implementation.md` | 详细实现文档 |
| `docs/ipost1_data_isolation_design.md` | 数据隔离设计文档 |
| `docs/ipost1-scraper-debugging-journey.md` | 调试历程技术文档 |
| `docs/ipost1_address_cleanup_design.md` | 地址清理设计文档 |
| `docs/mailboxes-page-optimization.md` | Mailboxes 页面优化方案 |

### 📝 修改文件 (15 个)

#### 后端
| 文件 | 变更说明 |
|------|---------|
| `internal/business/crawler/service.go` | 添加 iPost1 爬虫服务和 API handler |
| `internal/business/crawler/scraper.go` | ATMB 爬虫添加 `Source="ATMB"` 标记 |
| `internal/business/crawler/reprocess.go` | 重处理时保留 Source 字段 |
| `internal/business/crawler/orchestrator.go` | 优化协调器支持多数据源 |
| `internal/business/crawler/stats.go` | 添加 `BySource` 统计 |
| `internal/platform/http/router.go` | 注册 iPost1 API、stats refresh、cancel run 端点 |
| `internal/repository/mailbox_repo.go` | 使用 Firestore Count API 优化计数 |
| `internal/repository/run_repo.go` | 添加超时检测和 CancelRun 方法 |
| `pkg/model/model.go` | Mailbox 新增 Source，SystemStats 新增 BySource |

#### 前端
| 文件 | 变更说明 |
|------|---------|
| `apps/web/index.tsx` | 配置 QueryClientProvider |
| `apps/web/pages/Mailboxes.tsx` | 迁移到 React Query |
| `apps/web/pages/Analytics.tsx` | 迁移到 React Query，添加 Source 统计和刷新按钮 |
| `apps/web/pages/Crawler.tsx` | 迁移到 React Query，添加超时和取消功能 |
| `apps/web/services/api.ts` | 添加 refreshStats、cancelCrawlRun API |
| `apps/web/types.ts` | 添加新状态和字段类型 |

---

## 技术实现亮点

### 1. Cloudflare 绕过

```go
opts := append(chromedp.DefaultExecAllocatorOptions[:],
    chromedp.Flag("headless", true),
    chromedp.Flag("disable-blink-features", "AutomationControlled"),
    chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)..."),
    chromedp.WindowSize(1920, 1080),
)
```

### 2. 畸形 JSON 处理

iPost1 API 返回的 JSON 包含 `\&quot;` 混合编码，无法被任何标准 JSON 解析器处理。

**解决方案**: 完全绕过 JSON 解析，使用字符串操作提取数据

```go
// 定位 display 字段
displayStart := strings.Index(rawHTML, `"display":"`)
displayEnd := strings.Index(rawHTML[displayStart:], `","searched"`)

// 正确的转义处理顺序
displayHTML = strings.ReplaceAll(displayHTML, `\&quot;`, ``)
displayHTML = strings.ReplaceAll(displayHTML, `&quot;`, ``)
```

### 3. 数据隔离架构

```
Firestore
└── mailboxes/
    ├── {id} { Source: "ATMB", ... }    ← ATMB 数据
    └── {id} { Source: "iPost1", ... }  ← iPost1 数据

查询示例:
- 所有数据: collection("mailboxes")
- 仅 ATMB: collection("mailboxes").where("Source", "==", "ATMB")
- 仅 iPost1: collection("mailboxes").where("Source", "==", "iPost1")
```

### 4. React Query 缓存策略

```typescript
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30 * 60 * 1000, // 30 minutes
      gcTime: 60 * 60 * 1000,    // 1 hour
      refetchOnWindowFocus: false,
      refetchOnReconnect: false,
    },
  },
});
```

### 5. 智能轮询（仅 running 时轮询）

```typescript
const { data: runs } = useQuery({
  queryKey: ['crawlRuns'],
  queryFn: api.getCrawlRuns,
  refetchInterval: (query) => {
    const hasRunning = query.state.data?.some((r) => r.status === 'running');
    return hasRunning ? 5000 : false;
  },
});
```

### 6. 自动超时检测

```go
const StaleRunTimeout = 45 * time.Minute

// 在 ListRuns 中自动检测
if run.Status == "running" && now.Sub(run.StartedAt) > StaleRunTimeout {
    run.Status = "timeout"
    run.FinishedAt = now
    go func() { _ = r.UpdateRun(context.Background(), run) }()
}
```

---

## 新增 API 端点

| 方法 | 端点 | 说明 |
|------|------|------|
| POST | `/api/crawl/ipost1/run` | 启动 iPost1 爬虫 |
| POST | `/api/stats/refresh` | 手动刷新统计数据 |
| POST | `/api/crawl/runs/:runId/cancel` | 取消运行中的任务 |

---

## 前端优化效果

| 指标 | 优化前 | 优化后 | 改善 |
|------|--------|--------|------|
| Mailboxes 页面请求数 | 2 次 | 1 次 | -50% |
| 后端计数复杂度 | O(n) | O(1) | 显著 |
| Analytics StrictMode 双调用 | 2 次 | 1 次 | 修复 |
| Crawler 无 job 时轮询 | 持续轮询 | 不轮询 | 修复 |
| 页面返回时请求 | 每次都请求 | 30分钟内复用 | -100% |

---

## 测试结果

### 爬虫运行结果

| 指标 | 数值 |
|------|------|
| 总州数 | 53 |
| 成功州数 | 51 |
| 失败州数 | 2 (CA, FL) |
| 成功率 | **96.2%** |
| 发现地点数 | **600+** |

---

## 依赖变更

### 新增依赖

**后端 (Go)**:
```go
require (
    github.com/chromedp/chromedp v0.11.2  // 浏览器自动化
)
```

**前端 (npm)**:
```json
{
  "@tanstack/react-query": "^5.x"
}
```

---

## 部署说明

### 1. 迁移现有数据

```bash
# 预览变更 (dry-run)
make migrate-source-dry

# 执行迁移
make migrate-source

# 清理 iPost1 地址
make migrate-clean-addresses
```

### 2. 部署 Firestore 索引

```bash
firebase deploy --only firestore:indexes
```

### 3. 运行 iPost1 爬虫

```bash
# 启动服务器
make run

# 触发爬虫
curl -X POST http://localhost:8080/api/crawl/ipost1/run
```

---

## Checklist

- [x] iPost1 爬虫实现完成
- [x] Cloudflare 绕过验证
- [x] JSON 解析问题解决
- [x] 数据源隔离实现
- [x] iPost1 地址清理工具
- [x] 前端迁移到 React Query
- [x] Analytics 页面 Source 分布统计
- [x] Analytics 页面刷新按钮
- [x] Crawler 页面智能轮询
- [x] Crawler 页面超时检测
- [x] Crawler 页面取消功能
- [x] API 端点添加
- [x] 迁移工具完成
- [x] 文档编写
- [ ] CA/FL 大州问题修复 (后续优化)

---

## 相关文档

- [数据隔离设计](../../docs/ipost1_data_isolation_design.md)
- [调试历程文档](../../docs/ipost1-scraper-debugging-journey.md)
- [地址清理设计](../../docs/ipost1_address_cleanup_design.md)
- [Mailboxes 页面优化方案](../../docs/mailboxes-page-optimization.md)
- [iPost1 实现文档](../../docs/ipost1_implementation.md)

---

## 统计

```
35 files changed
+4,268 insertions
-155 deletions
```
