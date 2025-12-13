# iPost1 最终实现方案

## 发现总结（2025-12-10）

### ✅ API 结构已确认

用户手动测试确认了真实的 API 端点：

1. **获取州列表**: `GET /locations_ajax.php?action=get_states_list&country_id=223`
   - 返回：JSON 数组，53 个州/地区
   - 字段：`id`, `code`, `name`, `number_locations`

2. **获取地点列表**: `GET /locations_ajax.php?action=get_mail_centers&state_id={id}`
   - 返回：JSON 对象，包含 `display` 字段（HTML 片段）
   - 包含完整的地址、价格信息

### ⚠️ Cloudflare 保护（仍然存在）

- 手动浏览器访问：✅ 可以
- 自动化脚本访问：❌ 403 Forbidden

**原因**：需要浏览器环境（Cookies、Session、JavaScript 质询）

## 最终方案：chromedp + JSON API + HTML 解析

### 架构图

```
1. chromedp 启动浏览器
   ↓
2. 访问 iPost1 主页（获取 Session）
   ↓
3. 调用 API: 获取州列表
   → 解析 JSON
   ↓
4. For each state:
   调用 API: 获取地点列表
   → 解析 JSON 获取 HTML
   → goquery 解析 HTML
   ↓
5. 转换为 model.Mailbox
   ↓
6. Smarty 验证
   ↓
7. Firestore 保存
```

### 为什么需要 chromedp

1. **Cloudflare 绕过**: 真实浏览器环境
2. **Session 管理**: 自动处理 Cookies
3. **JavaScript 质询**: 自动完成

### 数据格式

**输入**: JSON + HTML 混合
**输出**: `model.Mailbox`

## Go 实现

### 文件结构

```
apps/api/internal/business/crawler/ipost1/
├── client.go       # chromedp 客户端
├── parser.go       # HTML 解析器
├── discovery.go    # 发现逻辑
└── client_test.go  # 测试
```

### 1. chromedp 客户端

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
    Display    string `json:"display"` // HTML
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

### 2. HTML 解析器

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

    // 城市、州、邮编
    cityStateZip := strings.TrimSpace(s.Find(".store-city-state-zip").Text())
    city, state, zip := parseCityStateZip(cityStateZip)

    // 价格
    priceText := s.Find(".store-plan-desktop b").Text()
    price := parsePrice(priceText)

    // 链接
    link, _ := s.Find("a[href*='secure_checkout']").Attr("href")
    if link == "" {
        link = fmt.Sprintf("https://ipostal1.com/secure_checkout.php?stID=%s", storeID)
    }

    name := fmt.Sprintf("iPostal1 - %s, %s", city, state)

    return model.Mailbox{
        Name: name,
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

### 3. 发现逻辑

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

    // 先访问主页建立 Session
    log.Println("访问 iPost1 主页...")
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
        log.Printf("正在抓取 %s (%s) - 预计 %s 个地点",
            state.Name, state.Code, state.NumberLocations)

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

### 4. 集成到 Service

```go
// apps/api/internal/business/crawler/service.go

func (s *Service) StartIPost1Crawl(ctx context.Context) (string, error) {
    startTime := time.Now().UTC()
    runID := generateRunID()

    if err := StartRun(ctx, s.runs, runID, startTime); err != nil {
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

    // 处理去重、验证、保存（复用现有逻辑）
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

    status = "success"
}
```

## 实施计划

### Phase 1: 核心实现（6-8 小时）

1. **创建目录和文件** (30 分钟)
   ```bash
   mkdir -p apps/api/internal/business/crawler/ipost1
   touch apps/api/internal/business/crawler/ipost1/{client.go,parser.go,discovery.go,client_test.go}
   ```

2. **实现 chromedp 客户端** (2-3 小时)
   - Session 管理
   - API 调用封装
   - 错误处理

3. **实现 HTML 解析器** (2-3 小时)
   - goquery 选择器
   - 字段提取
   - 数据清洗

4. **实现发现逻辑** (1-2 小时)
   - 州遍历
   - 并发控制
   - 进度日志

### Phase 2: 集成测试（2-3 小时）

1. **单元测试** (1 小时)
   - 测试 HTML 解析
   - 测试字段映射

2. **集成测试** (1-2 小时)
   - 测试完整流程
   - 验证数据质量
   - 检查去重逻辑

### Phase 3: 服务集成（2-3 小时）

1. **集成到 Service** (1 小时)
2. **添加 API 端点** (1 小时)
3. **更新文档** (1 小时)

### 总计：10-14 小时

## 依赖安装

```bash
go get github.com/chromedp/chromedp
go get github.com/PuerkitoBio/goquery
```

## 环境变量

```bash
# .env
IPOST1_ENABLED=true
IPOST1_HEADLESS=true
IPOST1_RATE_LIMIT=2s
```

## API 端点

```
POST /api/crawl/ipost1/run
GET /api/crawl/ipost1/status?runId=...
```

## 性能预估

- **总地点数**: ~4000
- **州数量**: 53
- **并发**: 不需要（串行足够快）
- **总耗时**: 15-20 分钟
  - 每州平均 2-3 秒（API 调用 + 解析）
  - 53 州 * 3 秒 = 159 秒 ≈ 3 分钟（地点获取）
  - 4000 * 1 秒 = 4000 秒 ≈ 67 分钟（Smarty 验证，如果需要）

## 风险管理

| 风险 | 缓解措施 |
|------|---------|
| Cloudflare 升级 | 监控；快速适配 chromedp 参数 |
| HTML 结构变化 | 版本化解析器；监控告警 |
| 内存消耗（chromedp） | 设置合理超时；定期重启浏览器 |
| Smarty 配额 | 多账号轮换；速率限制 |

## 成功标准

- ✅ 成功获取 3500+ 个地址（>90%）
- ✅ 覆盖 50+ 个州
- ✅ Smarty 验证成功率 > 95%
- ✅ 零数据丢失
- ✅ 完成时间 < 30 分钟（不含 Smarty 验证）

## 下一步

```bash
# 1. 开始实现
cd apps/api/internal/business/crawler
mkdir ipost1

# 2. 复制现有代码作为模板
# 参考 parser.go, fetcher.go 等

# 3. 根据本文档实现
# - client.go (chromedp)
# - parser.go (goquery)
# - discovery.go (发现逻辑)

# 4. 测试
go test ./internal/business/crawler/ipost1/...

# 5. 集成
# 在 service.go 中添加 StartIPost1Crawl

# 6. 运行
curl -X POST http://localhost:8080/api/crawl/ipost1/run
```

---

**文档版本**: v3.0 (Final Implementation Plan)
**状态**: ✅ 准备就绪，可以开始实现
**预计工时**: 10-14 小时
**优先级**: 高（API 已确认可用）
