# 爬虫 Job 业务逻辑整理

## 1. Job 入口与 API 端点

| HTTP 方法 | 端点                            | 说明                 |
| :-------- | :------------------------------ | :------------------- |
| **POST**  | `/api/crawl/run`                | 启动 ATMB 爬虫任务   |
| **POST**  | `/api/crawl/ipost1/run`         | 启动 iPost1 爬虫任务 |
| **POST**  | `/api/crawl/reprocess`          | 重新处理已爬取的邮箱 |
| **GET**   | `/api/crawl/status`             | 获取指定任务状态     |
| **GET**   | `/api/crawl/runs`               | 列出最近的爬虫任务   |
| **POST**  | `/api/crawl/runs/:runId/cancel` | 取消运行中的任务     |

---

## 2. Job 状态生命周期

**状态流转：**
`running` &rarr; `success` | `failed` | `partial_halt` | `timeout` | `cancelled`

| 状态             | 说明                               |
| :--------------- | :--------------------------------- |
| **running**      | 任务正在执行                       |
| **success**      | 任务成功完成                       |
| **failed**       | 任务失败（无成功记录）             |
| **partial_halt** | 部分失败（如 mark-and-sweep 出错） |
| **timeout**      | 任务运行超过 45 分钟被标记为超时   |
| **cancelled**    | 用户手动取消                       |

---

## 3. Job 执行流程

```
HTTP 请求
↓
Service.Start() / StartIPost1Crawl()
↓
异步 Goroutine (30 分钟超时上下文)
├── StartRun() - 创建 DB 记录 (status="running")
├── execute() / executeIPost1()
│ ├── 1. Discovery (发现链接)
│ ├── 2. Fetch HTML (带重试: 3 次, 指数退避)
│ ├── 3. Parse HTML (解析数据)
│ ├── 4. Deduplication (通过 DataHash 去重)
│ ├── 5. Smarty Validation (地址验证)
│ ├── 6. Incremental Save (每 20 条批量写入)
│ ├── 7. MarkAndSweep (标记旧数据为 inactive)
│ └── 8. AggregateSystemStats (更新统计)
└── FinishRun() - 更新最终状态和统计
```

---

## 4. 核心组件与文件

| 组件          | 文件                                                                       | 职责                           |
| :------------ | :------------------------------------------------------------------------- | :----------------------------- |
| Service       | [service.go](../apps/api/internal/business/crawler/service.go)             | Job 启动与协调                 |
| Orchestrator  | [orchestrator.go](../apps/api/internal/business/crawler/orchestrator.go)   | 并发控制 (5 workers)           |
| Fetcher       | [fetcher.go](../apps/api/internal/business/crawler/fetcher.go)             | HTTP 请求 (20s 超时, 3 次重试) |
| Parser        | [parser.go](../apps/api/internal/business/crawler/parser.go)               | ATMB HTML 解析                 |
| iPost1 Parser | [ipost1/parser.go](../apps/api/internal/business/crawler/ipost1/parser.go) | iPost1 数据解析                |
| Scraper       | [scraper.go](../apps/api/internal/business/crawler/scraper.go)             | 批量处理与写入                 |
| Reprocess     | [reprocess.go](../apps/api/internal/business/crawler/reprocess.go)         | 重新解析已存储数据             |
| RunRepository | [run_repo.go](../apps/api/internal/repository/run_repo.go)                 | Job 状态持久化                 |

---

## 5. 超时处理机制

| 层级          | 超时时间 | 实现位置                             |
| :------------ | :------- | :----------------------------------- |
| Job 执行超时  | 30 分钟  | context.WithTimeout in service.go:56 |
| HTTP 请求超时 | 20 秒    | fetcher.go:19                        |
| 僵尸任务检测  | 45 分钟  | run_repo.go:14                       |

---

## 6. 取消机制的问题 ⚠️

当前实现存在缺陷（对应你选中的 issue）:

```Go
// run_repo.go:102-124
func (r \*RunRepository) CancelRun(ctx context.Context, runID string) error {
// 仅更新数据库状态为 "cancelled"
run.Status = "cancelled"
return r.UpdateRun(ctx, run)
}
```

问题:

- 只在数据库中标记状态为 cancelled
- 不会真正停止正在运行的 goroutine
- 爬虫会继续执行直到自然完成或 45 分钟超时

---

## 7. 数据模型

CrawlRun (Job 记录):

```Go
type CrawlRun struct {
    RunID      string        // "RUN_<timestamp>"
    Source     string        // "ATMB" | "iPost1"
    Status     string        // 状态
    Stats      CrawlRunStats // Found/Validated/Skipped/Failed
    StartedAt  time.Time
    FinishedAt time.Time
    ErrorSample []ErrorSample
}
```

Mailbox (爬取的邮箱数据):

```Go
type Mailbox struct {
    ID, Source, Name, Link string
    AddressRaw, StandardizedAddress
    Price float64
    CMRA, RDI string          // 验证结果
    DataHash string           // 去重用
    CrawlRunID string         // 关联的 Job ID
    Active bool               // mark-and-sweep 标记
    RawHTML string            // 原始 HTML (用于 reprocess)
    ParserVersion string      // 解析器版本
}
```

---

## 8. 关键特性总结

| 特性     | 实现方式                            |
| :------- | :---------------------------------- |
| 异步执行 | 立即返回 runID，后台 goroutine 执行 |
| 进度追踪 | 每处理 25 条更新一次状态            |
| 容错性   | panic recovery + defer finalize     |
| 增量写入 | 每 20 条批量写入，防止数据丢失      |
| 去重     | DataHash 检查避免重复处理           |
| 数据保留 | 存储 RawHTML 支持 reprocess         |
| 源隔离   | ATMB 和 iPost1 数据分开管理         |
