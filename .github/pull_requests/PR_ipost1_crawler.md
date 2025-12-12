# Pull Request: iPost1 虚拟邮箱爬虫实现

## 概述

本 PR 为 virtualbox-verifier 系统添加 iPost1 虚拟邮箱数据源支持，实现了一个完整的爬虫系统，可自动发现并收集 iPost1 平台上全美 53 个州/地区的虚拟邮箱地址。

**分支**: `feature/ipost1-cralwer` → `main`

**提交历史**:
```
7c1c0b4 fix: resolve iPost1 API malformed JSON parsing and complete scraper
b7240bb feat: add features to support iPost1 mailbox
f34f6cf docs: add iPost1 scraper implementation plan
```

---

## 核心功能

### ✅ 新增功能

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

3. **迁移工具**
   - 为现有 ATMB 数据添加 `Source` 字段
   - 支持 dry-run 模式预览变更

4. **API 端点**
   - `POST /api/crawl/ipost1/run` - 启动 iPost1 爬虫

---

## 文件变更详情

### 📁 新增文件 (14 个)

#### 核心代码
| 文件 | 说明 |
|------|------|
| `internal/business/crawler/ipost1/client.go` | chromedp 客户端，处理 HTTP 请求和反 Cloudflare |
| `internal/business/crawler/ipost1/discovery.go` | 发现流程，协调州遍历、解析、验证、存储 |
| `internal/business/crawler/ipost1/parser.go` | HTML 解析器，从 API 响应提取邮箱数据 |
| `cmd/migrate-add-source/main.go` | 数据迁移工具，为旧数据添加 Source 字段 |

#### 配置文件
| 文件 | 说明 |
|------|------|
| `firestore.indexes.json` | Firestore 复合索引配置（按 Source 过滤） |
| `Makefile` | 新增 iPost1 相关命令 |

#### 文档
| 文件 | 说明 |
|------|------|
| `docs/ipost1_README.md` | iPost1 爬虫使用指南 |
| `docs/ipost1_implementation.md` | 详细实现文档 |
| `docs/ipost1_data_isolation_design.md` | 数据隔离设计文档 |
| `docs/ipost1_final_implementation_plan.md` | 实现计划文档 |
| `docs/ipost1-scraper-debugging-journey.md` | 调试历程技术文档 |

### 📝 修改文件 (6 个)

| 文件 | 变更说明 |
|------|---------|
| `internal/business/crawler/service.go` | 添加 iPost1 爬虫服务和 API handler |
| `internal/business/crawler/scraper.go` | ATMB 爬虫添加 `Source="ATMB"` 标记 |
| `internal/business/crawler/reprocess.go` | 重处理时保留 Source 字段 |
| `internal/business/crawler/orchestrator.go` | 优化协调器支持多数据源 |
| `internal/platform/http/router.go` | 注册 iPost1 API 端点 |
| `pkg/model/model.go` | Mailbox 结构新增 Source 字段 |
| `README.md` | 更新项目说明 |
| `go.mod` / `go.sum` | 新增 chromedp 依赖 |

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
displayHTML = strings.ReplaceAll(displayHTML, `\\`, `\`)
displayHTML = strings.ReplaceAll(displayHTML, `\n`, "\n")
displayHTML = strings.ReplaceAll(displayHTML, `\"`, `"`)
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

---

## 测试结果

### 爬虫运行结果

| 指标 | 数值 |
|------|------|
| 总州数 | 53 |
| 成功州数 | 51 |
| 失败州数 | 2 (CA, FL) |
| 成功率 | **96.2%** |
| 发现地点数 | **600+** (持续增加) |

### 成功样例

```
✅ Alabama: 29 locations
✅ Alaska: 4 locations
✅ Arizona: 101 locations
✅ Georgia: 164 locations
✅ Illinois: 93 locations
✅ Colorado: 63 locations
```

### 失败原因

California 和 Florida 失败原因：
```
error: html: open stack of elements exceeds 512 nodes
```
这两个州地点数量过多（可能 500+），超过 goquery 的 HTML 嵌套限制。

---

## 已知限制

1. **大州解析限制**
   - CA 和 FL 因 goquery 512 节点限制失败
   - 未来可通过分页或流式解析解决

2. **性能优化空间**
   - 当前为串行处理各州
   - 可优化为并发处理提升速度

3. **缺少完整地址字段**
   - API 返回的 HTML 结构与预期不同
   - name 字段需自动生成

---

## 依赖变更

### 新增依赖

```go
require (
    github.com/chromedp/chromedp v0.11.2  // 浏览器自动化
)
```

---

## 部署说明

### 1. 迁移现有数据

```bash
# 预览变更 (dry-run)
make migrate-source-dry

# 执行迁移
make migrate-source
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

- [x] 代码实现完成
- [x] Cloudflare 绕过验证
- [x] JSON 解析问题解决
- [x] 数据源隔离实现
- [x] API 端点添加
- [x] 迁移工具完成
- [x] 文档编写
- [ ] CA/FL 大州问题修复 (后续优化)
- [ ] 并发处理优化 (后续优化)

---

## 相关文档

- [iPost1 爬虫使用指南](./ipost1_README.md)
- [数据隔离设计](./ipost1_data_isolation_design.md)
- [调试历程文档](./ipost1-scraper-debugging-journey.md)
- [实现计划](./ipost1_final_implementation_plan.md)

---

## 统计

```
20 files changed
+3,422 insertions
-10 deletions
```
