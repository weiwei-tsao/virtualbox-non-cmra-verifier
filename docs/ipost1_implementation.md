# iPost1 虚拟邮箱爬虫 - 完整实施方案

## 1. 概述

**目标**: 从 iPost1.com 抓取 ~4000 个美国虚拟邮箱地址，与现有 ATMB 数据完全隔离。

**API 结构**（已验证）:
```
GET /locations_ajax.php?action=get_states_list&country_id=223
→ JSON Array (53 个州)

GET /locations_ajax.php?action=get_mail_centers&state_id={id}&country_id=223
→ JSON Object { display: "<HTML片段>" }
```

**技术栈**: chromedp + goquery + 现有 Mailbox 模型

## 2. 数据隔离设计（关键！）

### 问题

现有 `MarkAndSweep` 会影响**所有**数据，导致 ATMB 和 iPost1 互相干扰：

```go
// orchestrator.go - 现有代码有问题
func MarkAndSweep(ctx, repo, currentRunID) {
    all := repo.FetchAllMap()  // 所有数据
    for m := range all {
        if m.CrawlRunID != currentRunID {
            m.Active = false  // ⚠️ 会影响其他来源！
        }
    }
}
```

### 解决方案：添加 Source 字段

```go
type Mailbox struct {
    Source string `json:"source" firestore:"source"`  // "ATMB" | "iPost1"
    // ... 其他字段
}

type CrawlRun struct {
    Source string `json:"source" firestore:"source"`
    // ... 其他字段
}

// 修改后的 MarkAndSweep
func MarkAndSweep(ctx, repo, currentRunID, source string) {
    for m := range all {
        if m.Source == source && m.CrawlRunID != currentRunID {
            m.Active = false  // ✅ 只影响同源数据
        }
    }
}
```

## 3. 实施计划

### Phase 1: 数据模型改造（2-3h）

#### 1.1 修改 Model

```go
// apps/api/pkg/model/model.go

type Mailbox struct {
    ID                  string
    Source              string `json:"source,omitempty" firestore:"source,omitempty"`  // 新增
    Name                string
    // ... 其余不变
}

type CrawlRun struct {
    RunID       string
    Source      string `json:"source,omitempty" firestore:"source,omitempty"`  // 新增
    Status      string
    // ... 其余不变
}
```

#### 1.2 数据迁移脚本

```go
// apps/api/cmd/migrate-add-source/main.go
package main

import (
    "context"
    "log"
    "cloud.google.com/go/firestore"
)

func main() {
    ctx := context.Background()

    // 初始化 Firestore（复用现有配置）
    projectID := os.Getenv("FIREBASE_PROJECT_ID")
    client, err := firestore.NewClient(ctx, projectID)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // 更新所有现有 mailboxes
    iter := client.Collection("mailboxes").Documents(ctx)
    batch := client.Batch()
    count := 0

    for {
        doc, err := iter.Next()
        if err == iterator.Done {
            break
        }
        if err != nil {
            log.Fatal(err)
        }

        // 添加 source 字段（假设现有数据都是 ATMB）
        batch.Update(doc.Ref, []firestore.Update{
            {Path: "source", Value: "ATMB"},
        })
        count++

        // 每 500 条提交一次
        if count%500 == 0 {
            if _, err := batch.Commit(ctx); err != nil {
                log.Fatal(err)
            }
            batch = client.Batch()
            log.Printf("已迁移 %d 条记录", count)
        }
    }

    // 提交剩余
    if count%500 != 0 {
        batch.Commit(ctx)
    }

    log.Printf("✅ 迁移完成，总计 %d 条记录", count)

    // 更新 crawl_runs
    iter = client.Collection("crawl_runs").Documents(ctx)
    batch = client.Batch()
    count = 0

    for {
        doc, err := iter.Next()
        if err == iterator.Done {
            break
        }
        if err != nil {
            log.Fatal(err)
        }

        batch.Update(doc.Ref, []firestore.Update{
            {Path: "source", Value: "ATMB"},
        })
        count++

        if count%500 == 0 {
            batch.Commit(ctx)
            batch = client.Batch()
        }
    }

    if count%500 != 0 {
        batch.Commit(ctx)
    }

    log.Printf("✅ crawl_runs 迁移完成，总计 %d 条", count)
}
```

#### 1.3 更新 Firestore 索引

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

**部署索引**:
```bash
firebase deploy --only firestore:indexes
```

### Phase 2: 修改现有代码（2-3h）

#### 2.1 修改 MarkAndSweep

```go
// apps/api/internal/business/crawler/orchestrator.go

func MarkAndSweep(ctx context.Context, repo MailboxStore, currentRunID string, source string) error {
    all, err := repo.FetchAllMap(ctx)
    if err != nil {
        return err
    }
    var toUpdate []model.Mailbox
    for _, m := range all {
        // ✅ 只处理同源数据
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

// StartRun 也要添加 source
func StartRun(ctx context.Context, repo RunLifecycleRepo, runID string, source string, startedAt time.Time) error {
    return repo.CreateRun(ctx, model.CrawlRun{
        RunID:     runID,
        Source:    source,  // ✅ 新增
        Status:    "running",
        StartedAt: startedAt,
    })
}
```

#### 2.2 修改 ATMB 爬虫

```go
// apps/api/internal/business/crawler/service.go

func (s *Service) Start(ctx context.Context, links []string) (string, error) {
    // ...
    if err := StartRun(ctx, s.runs, runID, "ATMB", startTime); err != nil {  // ✅ 添加 source
        return "", err
    }
    // ...
}

func (s *Service) execute(ctx context.Context, runID string, links []string, startedAt time.Time) {
    // ...

    for _, link := range links {
        // ...
        parsed.Source = "ATMB"  // ✅ 设置来源
        parsed.CrawlRunID = runID
        // ...
    }

    // MarkAndSweep 只影响 ATMB
    if err := MarkAndSweep(ctx, s.mailboxes, runID, "ATMB"); err != nil {  // ✅ 添加 source
        status = "partial_halt"
        log.Printf("mark and sweep error: %v", err)
    }
}
```

#### 2.3 修改 Repository（如果需要按 source 查询）

```go
// apps/api/internal/repository/mailbox_repo.go

type ListOptions struct {
    Source   string  // 新增
    State    string
    CMRA     string
    RDI      string
    Active   *bool
    Page     int
    PageSize int
}

func (r *MailboxRepository) List(ctx context.Context, opts ListOptions) ([]model.Mailbox, int, error) {
    query := r.client.Collection("mailboxes")

    // 按 source 筛选
    if opts.Source != "" {
        query = query.Where("source", "==", opts.Source)
    }

    // ... 其余查询条件
}
```

#### 2.4 测试 Phase 2

```bash
# 1. 运行迁移脚本
export FIREBASE_PROJECT_ID=your-project-id
go run cmd/migrate-add-source/main.go

# 2. 测试 ATMB 爬虫
curl -X POST http://localhost:8080/api/crawl/run \
  -H "Content-Type: application/json" \
  -d '{"links": ["https://anytimemailbox.com/l/usa/ca"]}'

# 3. 验证数据
# 检查所有 mailboxes 都有 source="ATMB"
# 检查 Active 状态正确
```

### Phase 3: 实现 iPost1 爬虫（10-14h）

#### 3.1 目录结构

```bash
mkdir -p apps/api/internal/business/crawler/ipost1
cd apps/api/internal/business/crawler/ipost1
touch client.go parser.go discovery.go client_test.go
```

#### 3.2 实现 chromedp 客户端

```go
// apps/api/internal/business/crawler/ipost1/client.go
package ipost1

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    "github.com/chromedp/chromedp"
)

const baseURL = "https://ipostal1.com/locations_ajax.php"

type Client struct {
    ctx context.Context
}

func NewClient(ctx context.Context) *Client {
    return &Client{ctx: ctx}
}

type StateResponse struct {
    ID              string `json:"id"`
    Code            string `json:"code"`
    Name            string `json:"name"`
    NumberLocations string `json:"number_locations"`
}

type LocationsResponse struct {
    NumResults int    `json:"num_results"`
    Display    string `json:"display"`  // HTML 片段
}

func (c *Client) GetStates() ([]StateResponse, error) {
    url := baseURL + "?action=get_states_list&country_id=223"

    var responseBody string
    err := chromedp.Run(c.ctx,
        chromedp.Navigate(url),
        chromedp.WaitReady("body"),
        chromedp.Sleep(2*time.Second),
        chromedp.InnerHTML("body", &responseBody),
    )
    if err != nil {
        return nil, err
    }

    var states []StateResponse
    if err := json.Unmarshal([]byte(responseBody), &states); err != nil {
        return nil, fmt.Errorf("parse states: %w", err)
    }
    return states, nil
}

func (c *Client) GetLocationsByState(stateID string) (LocationsResponse, error) {
    url := fmt.Sprintf("%s?action=get_mail_centers&country_id=223&state_id=%s&exactMatch=0",
        baseURL, stateID)

    var responseBody string
    err := chromedp.Run(c.ctx,
        chromedp.Navigate(url),
        chromedp.WaitReady("body"),
        chromedp.Sleep(1*time.Second),
        chromedp.InnerHTML("body", &responseBody),
    )
    if err != nil {
        return LocationsResponse{}, err
    }

    var result LocationsResponse
    if err := json.Unmarshal([]byte(responseBody), &result); err != nil {
        return LocationsResponse{}, fmt.Errorf("parse locations: %w", err)
    }
    return result, nil
}
```

#### 3.3 实现 HTML 解析器

```go
// apps/api/internal/business/crawler/ipost1/parser.go
package ipost1

import (
    "fmt"
    "regexp"
    "strconv"
    "strings"
    "github.com/PuerkitoBio/goquery"
    "github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

func ParseLocationsHTML(htmlContent string) ([]model.Mailbox, error) {
    doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
    if err != nil {
        return nil, fmt.Errorf("parse html: %w", err)
    }

    var mailboxes []model.Mailbox
    doc.Find("article.mail-center-card").Each(func(i int, s *goquery.Selection) {
        mb := parseArticle(s)
        mailboxes = append(mailboxes, mb)
    })

    return mailboxes, nil
}

func parseArticle(s *goquery.Selection) model.Mailbox {
    storeID, _ := s.Attr("store-id")

    // 街道地址
    street := strings.TrimSpace(s.Find(".store-street-address").Text())
    street = strings.ReplaceAll(street, "\n", " ")
    street = regexp.MustCompile(`\s+`).ReplaceAllString(street, " ")

    // 城市、州、邮编："Anchorage, AK 99503"
    cityStateZip := strings.TrimSpace(s.Find(".store-city-state-zip").Text())
    city, state, zip := parseCityStateZip(cityStateZip)

    // 价格："''Standard'' address from $9.99"
    priceText := s.Find(".store-plan-desktop b").Text()
    price := parsePrice(priceText)

    // 链接
    link, _ := s.Find("a[href*='secure_checkout']").Attr("href")
    if link == "" {
        link = fmt.Sprintf("https://ipostal1.com/secure_checkout.php?stID=%s", storeID)
    }

    name := fmt.Sprintf("iPostal1 - %s, %s", city, state)

    return model.Mailbox{
        Source: "iPost1",  // ✅ 设置来源
        Name:   name,
        AddressRaw: model.AddressRaw{
            Street: street,
            City:   city,
            State:  state,
            Zip:    zip,
        },
        Price:  price,
        Link:   link,
        Active: true,
    }
}

func parseCityStateZip(text string) (city, state, zip string) {
    parts := strings.Split(text, ",")
    if len(parts) < 2 {
        return
    }
    city = strings.TrimSpace(parts[0])
    stateZip := strings.Fields(strings.TrimSpace(parts[1]))
    if len(stateZip) >= 1 {
        state = stateZip[0]
    }
    if len(stateZip) >= 2 {
        zip = stateZip[1]
    }
    return
}

func parsePrice(text string) float64 {
    re := regexp.MustCompile(`\$(\d+\.?\d*)`)
    matches := re.FindStringSubmatch(text)
    if len(matches) < 2 {
        return 0
    }
    price, _ := strconv.ParseFloat(matches[1], 64)
    return price
}
```

#### 3.4 实现发现逻辑

```go
// apps/api/internal/business/crawler/ipost1/discovery.go
package ipost1

import (
    "context"
    "fmt"
    "log"
    "time"
    "github.com/chromedp/chromedp"
    "github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

func DiscoverAllLocations(ctx context.Context) ([]model.Mailbox, error) {
    // 创建 chromedp 上下文
    allocCtx, cancel := chromedp.NewExecAllocator(context.Background(),
        chromedp.NoDefaultBrowserCheck,
        chromedp.Flag("headless", true),
        chromedp.Flag("disable-gpu", true),
        chromedp.Flag("no-sandbox", true),
        chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"),
    )
    defer cancel()

    browserCtx, cancel := chromedp.NewContext(allocCtx)
    defer cancel()

    // 访问主页建立 Session
    log.Println("访问 iPost1 主页建立 Session...")
    err := chromedp.Run(browserCtx,
        chromedp.Navigate("https://ipostal1.com/"),
        chromedp.WaitReady("body"),
        chromedp.Sleep(3*time.Second),
    )
    if err != nil {
        return nil, fmt.Errorf("访问主页失败: %w", err)
    }

    client := NewClient(browserCtx)

    // 1. 获取州列表
    states, err := client.GetStates()
    if err != nil {
        return nil, fmt.Errorf("获取州列表: %w", err)
    }
    log.Printf("发现 %d 个州", len(states))

    var allMailboxes []model.Mailbox

    // 2. 遍历每个州
    for _, state := range states {
        log.Printf("正在抓取 %s (%s) - 预计 %s 个地点", state.Name, state.Code, state.NumberLocations)

        resp, err := client.GetLocationsByState(state.ID)
        if err != nil {
            log.Printf("获取州 %s 失败: %v", state.Code, err)
            continue
        }

        if resp.NumResults == 0 {
            continue
        }

        // 解析 HTML
        mailboxes, err := ParseLocationsHTML(resp.Display)
        if err != nil {
            log.Printf("解析州 %s HTML 失败: %v", state.Code, err)
            continue
        }

        log.Printf("州 %s: 成功解析 %d 个地点", state.Code, len(mailboxes))
        allMailboxes = append(allMailboxes, mailboxes...)

        // 延迟避免速率限制
        time.Sleep(2 * time.Second)
    }

    return allMailboxes, nil
}
```

#### 3.5 集成到 Service

```go
// apps/api/internal/business/crawler/service.go

import (
    "github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/business/crawler/ipost1"
)

func (s *Service) StartIPost1Crawl(ctx context.Context) (string, error) {
    startTime := time.Now().UTC()
    runID := generateRunID()

    if err := StartRun(ctx, s.runs, runID, "iPost1", startTime); err != nil {  // ✅ source="iPost1"
        return "", err
    }

    runCtx, cancel := context.WithTimeout(context.Background(), 90*time.Minute)
    go func() {
        defer cancel()
        s.executeIPost1(runCtx, runID, startTime)
    }()

    return runID, nil
}

func (s *Service) executeIPost1(ctx context.Context, runID string, startedAt time.Time) {
    status := "running"
    stats := model.CrawlRunStats{}

    defer func() {
        if rec := recover(); rec != nil {
            status = "failed"
            log.Printf("ipost1 crawl panic run %s: %v", runID, rec)
        }
        if err := FinishRun(ctx, s.runs, runID, stats, status, startedAt); err != nil {
            log.Printf("finish run %s: %v", runID, err)
        }
    }()

    // 发现所有地址
    mailboxes, err := ipost1.DiscoverAllLocations(ctx)
    if err != nil {
        status = "failed"
        log.Printf("discover locations error: %v", err)
        return
    }

    stats.Found = len(mailboxes)
    log.Printf("发现 %d 个 iPost1 地址", len(mailboxes))

    // 去重、验证、保存
    existing, _ := s.mailboxes.FetchAllMetadata(ctx)
    var toSave []model.Mailbox

    for _, mb := range mailboxes {
        mb.DataHash = util.HashMailboxKey(mb.Name, mb.AddressRaw)
        mb.CrawlRunID = runID
        mb.ParserVersion = CurrentParserVersion
        mb.LastParsedAt = time.Now()

        if prev, ok := existing[mb.Link]; ok {
            if prev.DataHash == mb.DataHash && prev.CMRA != "" {
                stats.Skipped++
                continue
            }
            mb.ID = prev.ID
        }

        // Smarty 验证
        if s.validator != nil {
            validated, err := s.validator.ValidateMailbox(ctx, mb)
            if err != nil {
                stats.Failed++
                log.Printf("validate %s error: %v", mb.Link, err)
            } else {
                mb = validated
                stats.Validated++
            }
        }

        toSave = append(toSave, mb)
    }

    // 批量保存
    if len(toSave) > 0 {
        if err := s.mailboxes.BatchUpsert(ctx, toSave); err != nil {
            status = "failed"
            log.Printf("batch upsert error: %v", err)
            return
        }
    }

    // MarkAndSweep 只影响 iPost1
    if err := MarkAndSweep(ctx, s.mailboxes, runID, "iPost1"); err != nil {  // ✅ source="iPost1"
        status = "partial_halt"
        log.Printf("mark and sweep error: %v", err)
    }

    status = "success"
}
```

#### 3.6 添加 API 端点

```go
// apps/api/internal/platform/http/router.go

func SetupRouter(cfg config.Config, crawlSvc *crawler.Service, ...) *gin.Engine {
    r := gin.Default()

    api := r.Group("/api")
    {
        crawl := api.Group("/crawl")
        {
            // ATMB
            crawl.POST("/run", handleCrawlRun(crawlSvc))
            crawl.GET("/status", handleCrawlStatus(crawlSvc))

            // iPost1
            ipost1Group := crawl.Group("/ipost1")
            {
                ipost1Group.POST("/run", handleIPost1Run(crawlSvc))
                ipost1Group.GET("/status", handleIPost1Status(crawlSvc))
            }
        }
    }

    return r
}

func handleIPost1Run(svc *crawler.Service) gin.HandlerFunc {
    return func(c *gin.Context) {
        runID, err := svc.StartIPost1Crawl(c.Request.Context())
        if err != nil {
            c.JSON(500, gin.H{"error": err.Error()})
            return
        }
        c.JSON(200, gin.H{"runId": runID})
    }
}

func handleIPost1Status(svc *crawler.Service) gin.HandlerFunc {
    return func(c *gin.Context) {
        runID := c.Query("runId")
        // 查询 crawl_runs collection
        // ...
    }
}
```

#### 3.7 测试 Phase 3

```bash
# 1. 安装依赖
go get github.com/chromedp/chromedp
go get github.com/PuerkitoBio/goquery

# 2. 启动服务
cd apps/api
go run cmd/server/main.go

# 3. 触发 iPost1 爬取
curl -X POST http://localhost:8080/api/crawl/ipost1/run

# 4. 查看状态
curl "http://localhost:8080/api/crawl/ipost1/status?runId=RUN_xxx"

# 5. 验证数据
# - 检查 Firestore 中有 source="iPost1" 的数据
# - 检查 ATMB 数据的 Active 状态未受影响
# - 检查数据量约 4000 条

# 6. 测试隔离性
# 同时运行 ATMB 和 iPost1，验证互不干扰
curl -X POST http://localhost:8080/api/crawl/run &
curl -X POST http://localhost:8080/api/crawl/ipost1/run &
```

## 4. 部署检查清单

- [ ] Phase 1: 数据模型修改完成
- [ ] Phase 1: 数据迁移脚本执行成功
- [ ] Phase 1: Firestore 索引部署
- [ ] Phase 2: MarkAndSweep 修改完成
- [ ] Phase 2: ATMB 爬虫设置 Source
- [ ] Phase 2: ATMB 功能测试通过
- [ ] Phase 3: iPost1 客户端实现
- [ ] Phase 3: HTML 解析器实现
- [ ] Phase 3: 集成到 Service
- [ ] Phase 3: API 端点添加
- [ ] Phase 3: 完整流程测试通过
- [ ] Phase 3: 隔离性测试通过

## 5. API 参考

### ATMB 爬虫
```
POST /api/crawl/run
GET  /api/crawl/status?runId=RUN_xxx
```

### iPost1 爬虫
```
POST /api/crawl/ipost1/run
GET  /api/crawl/ipost1/status?runId=RUN_xxx
```

### 查询（未来扩展）
```
GET /api/mailboxes?source=ATMB
GET /api/mailboxes?source=iPost1
GET /api/mailboxes              # 所有
```

## 6. 预期结果

- **ATMB**: ~2500 个地址
- **iPost1**: ~4000 个地址
- **总计**: ~6500 个地址
- **完全隔离**: Active 状态互不影响

---

**预计总工时**: 14-19 小时
**当前状态**: 准备实施
