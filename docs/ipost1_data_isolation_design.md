# iPost1 数据隔离设计方案

## 问题分析

### 问题 1: 数据来源区分

**现状**：
- `Mailbox` 模型没有 `Source` 字段
- 无法区分数据来自 ATMB 还是 iPost1
- 前端无法按来源筛选

**影响**：
- 用户无法知道地址来自哪个服务商
- 无法分别统计各服务商的数据
- 调试困难（无法定位问题来源）

### 问题 2: 流程干扰

**现状**：
```go
// orchestrator.go:84-101
func MarkAndSweep(ctx context.Context, repo MailboxStore, currentRunID string) error {
    all, err := repo.FetchAllMap(ctx)  // 获取所有 Mailbox
    for _, m := range all {
        if m.CrawlRunID != currentRunID && m.Active {
            m.Active = false  // ⚠️ 会影响其他来源的数据！
        }
    }
}
```

**问题**：
- MarkAndSweep 会将**所有**非当前 runID 的 Mailbox 设为 `Active=false`
- 如果 ATMB 和 iPost1 同时运行，会互相覆盖对方的 Active 状态

**场景示例**：
```
时间 0:00 - ATMB 爬取开始 (runID=RUN_001)
时间 0:10 - ATMB 完成，MarkAndSweep(RUN_001)
            → 所有 ATMB 数据 Active=true

时间 0:15 - iPost1 爬取开始 (runID=RUN_002)
时间 0:30 - iPost1 完成，MarkAndSweep(RUN_002)
            → ❌ ATMB 数据全部变为 Active=false！
            → ✅ 只有 iPost1 数据 Active=true
```

## 解决方案

### 方案 1: 添加 Source 字段（推荐）

#### 1.1 修改数据模型

```go
// apps/api/pkg/model/model.go

type Mailbox struct {
    ID                  string
    Source              string  // 新增：数据来源 "ATMB" | "iPost1"
    Name                string
    AddressRaw          AddressRaw
    Price               float64
    Link                string
    CMRA                string
    RDI                 string
    StandardizedAddress StandardizedAddress
    DataHash            string
    LastValidatedAt     time.Time
    CrawlRunID          string
    Active              bool
    RawHTML             string
    ParserVersion       string
    LastParsedAt        time.Time
}

// CrawlRun 也需要记录来源
type CrawlRun struct {
    RunID       string
    Source      string  // 新增："ATMB" | "iPost1"
    Status      string
    Stats       CrawlRunStats
    StartedAt   time.Time
    FinishedAt  time.Time
    ErrorSample []ErrorSample
}
```

#### 1.2 修改 MarkAndSweep（关键！）

```go
// apps/api/internal/business/crawler/orchestrator.go

// MarkAndSweep 只影响同源数据
func MarkAndSweep(ctx context.Context, repo MailboxStore, currentRunID string, source string) error {
    all, err := repo.FetchAllMap(ctx)
    if err != nil {
        return err
    }

    var toUpdate []model.Mailbox
    for _, m := range all {
        // ✅ 只处理同源且非当前 runID 的数据
        if m.Source == source && m.CrawlRunID != currentRunID && m.Active {
            m.Active = false
            toUpdate = append(toUpdate, m)
        }
    }

    if len(toUpdate) == 0 {
        return nil
    }
    return repo.BatchUpsert(ctx, toUpdate)
}
```

#### 1.3 ATMB 爬虫修改

```go
// apps/api/internal/business/crawler/service.go

func (s *Service) execute(ctx context.Context, runID string, links []string, startedAt time.Time) {
    // ... 现有代码 ...

    for _, link := range links {
        // 设置来源
        parsed.Source = "ATMB"  // ✅ 标记来源
        parsed.CrawlRunID = runID
        // ...
    }

    // MarkAndSweep 只影响 ATMB 数据
    if err := MarkAndSweep(ctx, s.mailboxes, runID, "ATMB"); err != nil {
        log.Printf("mark and sweep error: %v", err)
    }
}
```

#### 1.4 iPost1 爬虫实现

```go
// apps/api/internal/business/crawler/service.go

func (s *Service) executeIPost1(ctx context.Context, runID string, startedAt time.Time) {
    // ... 发现逻辑 ...

    for _, mb := range mailboxes {
        mb.Source = "iPost1"  // ✅ 标记来源
        mb.CrawlRunID = runID
        mb.DataHash = util.HashMailboxKey(mb.Name, mb.AddressRaw)
        // ...
    }

    // MarkAndSweep 只影响 iPost1 数据
    if err := MarkAndSweep(ctx, s.mailboxes, runID, "iPost1"); err != nil {
        log.Printf("mark and sweep error: %v", err)
    }
}
```

#### 1.5 API 查询支持

```go
// apps/api/internal/platform/http/handlers.go

// GET /api/mailboxes?source=ATMB
// GET /api/mailboxes?source=iPost1
// GET /api/mailboxes (返回所有)

func handleListMailboxes(repo *repository.MailboxRepository) gin.HandlerFunc {
    return func(c *gin.Context) {
        source := c.Query("source")  // 新增：按来源筛选
        state := c.Query("state")
        // ...

        // Repository 需要支持按 source 查询
        mailboxes, total, err := repo.List(ctx, ListOptions{
            Source:   source,  // ✅ 新增参数
            State:    state,
            CMRA:     cmra,
            Page:     page,
            PageSize: pageSize,
        })
    }
}
```

#### 1.6 Firestore 索引

```json
// apps/api/firestore.indexes.json

{
  "indexes": [
    {
      "collectionGroup": "mailboxes",
      "queryScope": "COLLECTION",
      "fields": [
        { "fieldPath": "source", "order": "ASCENDING" },
        { "fieldPath": "active", "order": "ASCENDING" },
        { "fieldPath": "state", "order": "ASCENDING" }
      ]
    },
    {
      "collectionGroup": "mailboxes",
      "queryScope": "COLLECTION",
      "fields": [
        { "fieldPath": "source", "order": "ASCENDING" },
        { "fieldPath": "cmra", "order": "ASCENDING" }
      ]
    }
  ]
}
```

### 方案 2: 分离 Collection（备选）

**不推荐**，因为：
- 增加复杂度（两个 Collection）
- 前端需要查询两次
- 统计困难
- 违反 DRY 原则

但如果未来有更多服务商，可以考虑：

```
mailboxes_atmb/     → ATMB 数据
mailboxes_ipost1/   → iPost1 数据
mailboxes_other/    → 其他服务商
```

## 实施步骤

### Phase 1: 数据模型迁移（1-2 小时）

```bash
# 1. 修改模型
vi apps/api/pkg/model/model.go
# 添加 Source 字段到 Mailbox 和 CrawlRun

# 2. 数据迁移脚本
# 为所有现有数据添加 Source="ATMB"
```

**迁移脚本示例**：
```go
// apps/api/cmd/migrate-source/main.go

func main() {
    ctx := context.Background()
    client := initFirestore(ctx)

    mailboxes, _ := client.Collection("mailboxes").Documents(ctx).GetAll()

    batch := client.Batch()
    for _, doc := range mailboxes {
        ref := doc.Ref
        batch.Update(ref, []firestore.Update{
            {Path: "source", Value: "ATMB"},
        })
    }

    batch.Commit(ctx)
    log.Println("迁移完成")
}
```

### Phase 2: 修改现有代码（2-3 小时）

1. **修改 MarkAndSweep**：添加 `source` 参数
2. **修改 ATMB 爬虫**：设置 `Source="ATMB"`
3. **修改 Repository**：支持按 `source` 查询
4. **修改 API Handlers**：支持 `source` 参数

### Phase 3: 实现 iPost1 爬虫（10-14 小时）

按照 [ipost1_final_implementation_plan.md](./ipost1_final_implementation_plan.md) 实现，确保：
- 所有数据设置 `Source="iPost1"`
- MarkAndSweep 传入 `"iPost1"`

### Phase 4: 测试（2-3 小时）

#### 测试用例

```bash
# 1. 测试 ATMB 爬取不影响 iPost1
curl -X POST http://localhost:8080/api/crawl/run
# 验证：iPost1 数据 Active 状态不变

# 2. 测试 iPost1 爬取不影响 ATMB
curl -X POST http://localhost:8080/api/crawl/ipost1/run
# 验证：ATMB 数据 Active 状态不变

# 3. 测试按来源查询
curl "http://localhost:8080/api/mailboxes?source=ATMB"
curl "http://localhost:8080/api/mailboxes?source=iPost1"

# 4. 测试统计
curl "http://localhost:8080/api/stats"
# 验证：分别统计 ATMB 和 iPost1 数量
```

## API 设计

### 独立端点

```
ATMB 爬虫:
  POST /api/crawl/run
  GET  /api/crawl/status?runId=RUN_xxx

iPost1 爬虫:
  POST /api/crawl/ipost1/run
  GET  /api/crawl/ipost1/status?runId=RUN_xxx

查询（通用）:
  GET /api/mailboxes?source=ATMB
  GET /api/mailboxes?source=iPost1
  GET /api/mailboxes                    # 返回所有
  GET /api/stats                        # 按来源分组统计
```

### 统计 API 增强

```go
type SystemStats struct {
    LastUpdated      time.Time
    TotalMailboxes   int
    BySource         map[string]SourceStats  // 新增
    ByState          map[string]int
}

type SourceStats struct {
    Total        int
    Commercial   int
    Residential  int
    AvgPrice     float64
}
```

**响应示例**：
```json
{
  "lastUpdated": "2025-12-10T12:00:00Z",
  "totalMailboxes": 6500,
  "bySource": {
    "ATMB": {
      "total": 2500,
      "commercial": 1200,
      "residential": 1300,
      "avgPrice": 15.99
    },
    "iPost1": {
      "total": 4000,
      "commercial": 1800,
      "residential": 2200,
      "avgPrice": 12.99
    }
  },
  "byState": {
    "CA": 1244,
    "TX": 813,
    ...
  }
}
```

## 前端影响

### 筛选器更新

```tsx
// Frontend: src/components/MailboxFilters.tsx

<select name="source">
  <option value="">所有来源</option>
  <option value="ATMB">AnytimeMailbox</option>
  <option value="iPost1">iPostal1</option>
</select>
```

### 表格显示

```tsx
// 在表格中显示来源
<td>{mailbox.source}</td>

// 添加来源徽章
{mailbox.source === 'ATMB' && <span class="badge-atmb">ATMB</span>}
{mailbox.source === 'iPost1' && <span class="badge-ipost1">iPost1</span>}
```

## 向后兼容

### 现有数据处理

```go
// Repository 查询时兼容无 Source 的旧数据
func (r *MailboxRepository) List(ctx context.Context, opts ListOptions) {
    query := r.client.Collection("mailboxes")

    if opts.Source != "" {
        query = query.Where("source", "==", opts.Source)
    }
    // 如果不指定 source，返回所有（包括 source 为空的旧数据）
}
```

### 迁移脚本执行时机

**选项 1**：一次性迁移（推荐）
```bash
# 部署前执行
go run cmd/migrate-source/main.go
```

**选项 2**：懒迁移
```go
// 读取数据时自动补充 Source
func (r *MailboxRepository) fetchMailbox(doc *firestore.DocumentSnapshot) model.Mailbox {
    var mb model.Mailbox
    doc.DataTo(&mb)

    // 兼容旧数据
    if mb.Source == "" {
        mb.Source = "ATMB"  // 假设旧数据都是 ATMB
    }

    return mb
}
```

## 总结

### ✅ 方案优势

1. **完全隔离**：ATMB 和 iPost1 互不干扰
2. **可扩展**：未来可轻松添加更多服务商
3. **向后兼容**：现有 ATMB 爬虫无需大改
4. **用户友好**：前端可按来源筛选
5. **统计清晰**：分来源统计数据质量

### ⚠️ 注意事项

1. **必须先迁移数据**：添加 Source 字段到现有数据
2. **必须修改 MarkAndSweep**：否则会互相干扰
3. **必须更新索引**：添加 source 相关索引
4. **测试覆盖**：充分测试隔离性

### 📊 工作量

| 任务 | 时间 |
|------|------|
| 数据模型修改 | 0.5h |
| 数据迁移脚本 | 1h |
| 修改现有代码 | 2-3h |
| 实现 iPost1 | 10-14h |
| 测试验证 | 2-3h |
| **总计** | **15.5-21.5h** |

---

**推荐实施顺序**：
1. 先修改数据模型和迁移脚本
2. 修改 MarkAndSweep 和现有 ATMB 代码
3. 充分测试现有功能
4. 再实现 iPost1 爬虫

这样可以确保现有功能不受影响。
