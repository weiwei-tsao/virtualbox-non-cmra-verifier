🏗️ 当前架构状态
功能 当前状态 能否实现你的需求

1. 增量爬取 ❌ 需要手动传入所有 links 不能 - 缺少发现机制
2. 单独解析 ❌ 没有保存原始 HTML 不能 - 无数据源
3. 断点续传 ⚠️ 有增量写入，但无版本控制 部分 - 无法识别"新数据"
   📋 详细分析与解决方案
   1️⃣ 增量爬取 - 及时更新上架/下架
   当前问题：
   查看 scraper.go:53-135 - 接收固定的 links 数组
   没有自动发现机制 - 无法知道 AnytimeMailbox 网站新增了哪些店
   每次都要爬全部链接
   实现方案：
   // 新增功能模块：Discovery（发现）

// Step 1: 爬取列表页，获取所有 mailbox 链接
func DiscoverAllLinks(ctx context.Context) ([]string, error) {
// 爬取 https://www.anytimemailbox.com/l (列表页)
// 提取所有 /s/xxx 链接
}

// Step 2: 对比差异
func FindChanges(discovered []string, existing map[string]Mailbox) Changes {
return Changes{
New: discovered - existing.links, // 新上架
Removed: existing.links - discovered, // 下架
}
}

// Step 3: 增量处理
func IncrementalCrawl(ctx context.Context) {
discovered := DiscoverAllLinks(ctx)
existing := store.FetchAllMap(ctx)
changes := FindChanges(discovered, existing)

    // 只爬新增的
    ScrapeAndUpsert(ctx, changes.New)

    // 标记下架（设置 Active=false）
    store.MarkInactive(ctx, changes.Removed)

}
新增 API：
POST /api/crawl/discover - 只发现差异，不爬取
POST /api/crawl/incremental - 自动发现 + 增量爬取
2️⃣ 单独解析 - 无需重新爬取
当前问题：
model.go:19-33 - 没有 RawHTML 字段
每次调整解析器都要重新请求 AnytimeMailbox
实现方案：
// Step 1: 扩展数据模型
type Mailbox struct {
// ... 现有字段 ...

    // 新增字段
    RawHTML        string    `json:"-" firestore:"rawHTML,omitempty"`  // 原始HTML（不返回给前端）
    ParserVersion  string    `firestore:"parserVersion,omitempty"`     // 解析器版本（如 "v1.0"）
    LastParsedAt   time.Time `firestore:"lastParsedAt,omitempty"`      // 最后解析时间

}

// Step 2: 爬取时保存 HTML
func ScrapeAndUpsert(...) {
body, _ := fetcher.Fetch(ctx, link)
html, _ := io.ReadAll(body) // 读取 HTML

    parsed, _ := ParseMailboxHTML(bytes.NewReader(html), link)
    parsed.RawHTML = string(html)           // ✅ 保存原始HTML
    parsed.ParserVersion = "v1.0"           // ✅ 标记版本
    parsed.LastParsedAt = time.Now()

}

// Step 3: 单独重新解析
func ReprocessFromDB(ctx context.Context, targetVersion string) {
mailboxes, \_ := store.FetchAll(ctx)

    for _, mb := range mailboxes {
        // 只处理有HTML且版本不匹配的
        if mb.RawHTML != "" && mb.ParserVersion != targetVersion {
            // 重新解析
            reparsed, _ := ParseMailboxHTML(
                strings.NewReader(mb.RawHTML),
                mb.Link,
            )

            // 保留 ID、RawHTML、CrawlRunID
            reparsed.ID = mb.ID
            reparsed.RawHTML = mb.RawHTML
            reparsed.ParserVersion = targetVersion  // ✅ 更新版本
            reparsed.LastParsedAt = time.Now()

            store.Update(ctx, reparsed)
        }
    }

}
新增 API：
POST /api/crawl/reprocess - 从数据库重新解析
{
"targetVersion": "v1.1",
"onlyOutdated": true // 只处理旧版本
}
3️⃣ 断点续传 + 只解析新数据
当前问题：
已有增量写入（每 100 条），但无法识别"哪些是新数据"
重新解析会处理所有记录
实现方案：
// 方案 A: 基于版本号（推荐）
func ReprocessFromDB(ctx context.Context, opts ReprocessOptions) {
query := store.Query()

    if opts.OnlyOutdated {
        // ✅ 只处理版本不匹配的
        query = query.Where("parserVersion", "!=", opts.TargetVersion)
    }

    if opts.SinceTime != nil {
        // ✅ 只处理特定时间后更新的
        query = query.Where("lastValidatedAt", ">", opts.SinceTime)
    }

    // 增量处理 + 断点续传
    const batchSize = 100
    for {
        batch := query.Limit(batchSize).Offset(processed)

        for _, mb := range batch {
            reparsed := ParseMailboxHTML(...)
            store.Update(ctx, reparsed)
            processed++

            // 每100条写入进度
            if processed % 100 == 0 {
                SaveProgress(processed)  // ✅ 断点续传
            }
        }

        if len(batch) < batchSize {
            break  // 处理完毕
        }
    }

}
断点续传机制：
// 在 CrawlRun 中记录进度
type CrawlRun struct {
// ... 现有字段 ...
ProcessedCount int `firestore:"processedCount,omitempty"`
LastProcessedID string `firestore:"lastProcessedID,omitempty"`
}

// 恢复时从断点继续
func ResumeReprocess(runID string) {
run := store.GetCrawlRun(runID)
startFrom := run.LastProcessedID // ✅ 从上次中断的地方继续
}
🎯 推荐实现优先级
我建议按这个顺序实现：
阶段 1：单独解析（最高优先级）⭐⭐⭐
原因：立即解决当前痛点 - 不需要重新爬 2073 条数据
✅ 添加 RawHTML, ParserVersion, LastParsedAt 字段
✅ 修改 scraper.go 保存 HTML
✅ 创建 ReprocessFromDB 函数
✅ 添加 POST /api/crawl/reprocess API
⚠️ 重要：重新爬取一次保存 HTML（之后就不需要了）
收益：解析器调试时间从 30 分钟（重新爬取）→ 2 分钟（重新解析）
阶段 2：断点续传 + 增量解析 ⭐⭐
原因：提升可靠性和效率
✅ 添加 ProcessedCount, LastProcessedID 到 CrawlRun
✅ 实现版本过滤逻辑
✅ 实现恢复机制
收益：解析中断后可继续，避免重复处理
阶段 3：增量爬取（长期优化）⭐
原因：目前 mailbox 数量不大，全量爬取尚可接受
实现 DiscoverAllLinks（爬列表页）
实现差异对比逻辑
添加 POST /api/crawl/incremental API
添加定时任务（每天自动增量爬取）
收益：从全量 2073 条 → 增量 10-50 条/天
