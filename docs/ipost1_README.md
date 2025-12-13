# iPost1 虚拟邮箱地址抓取 - 快速开始

## 概述

从 iPost1.com 抓取美国境内 ~4000 个虚拟邮箱地址，通过 Smarty API 验证 CMRA/RDI 状态。

## 实现方案

详见：[ipost1_final_implementation_plan.md](./ipost1_final_implementation_plan.md)

**技术栈**：
- chromedp（浏览器自动化，绕过 Cloudflare）
- JSON API + HTML 解析（goquery）
- 完全复用现有 `Mailbox` 数据模型

**预计工时**：10-14 小时

## API 结构（已验证）

### 1. 获取州列表
```
GET /locations_ajax.php?action=get_states_list&country_id=223
返回: JSON Array (53 个州)
```

### 2. 获取州的地点列表
```
GET /locations_ajax.php?action=get_mail_centers&state_id={id}&country_id=223
返回: JSON Object (包含 HTML 片段)
```

**关键发现**：第二个 API 返回的 `display` 字段包含 HTML，需要用 goquery 解析。

## 快速开始

### 1. 安装依赖

```bash
go get github.com/chromedp/chromedp
go get github.com/PuerkitoBio/goquery
```

### 2. 创建文件

```bash
mkdir -p apps/api/internal/business/crawler/ipost1
cd apps/api/internal/business/crawler/ipost1
touch client.go parser.go discovery.go client_test.go
```

### 3. 实现代码

参考 [ipost1_final_implementation_plan.md](./ipost1_final_implementation_plan.md) 中的完整代码示例：
- `client.go` - chromedp 客户端
- `parser.go` - HTML 解析器
- `discovery.go` - 发现逻辑

### 4. 集成到 Service

在 `apps/api/internal/business/crawler/service.go` 中添加：

```go
func (s *Service) StartIPost1Crawl(ctx context.Context) (string, error) {
    // 见实现方案文档
}
```

### 5. 添加 API 端点

```go
// apps/api/internal/platform/http/router.go
ipost1 := crawl.Group("/ipost1")
{
    ipost1.POST("/run", handleIPost1Run(crawlSvc))
    ipost1.GET("/status", handleIPost1Status(crawlSvc))
}
```

### 6. 测试运行

```bash
# 启动服务
go run cmd/server/main.go

# 触发爬取
curl -X POST http://localhost:8080/api/crawl/ipost1/run

# 查看状态
curl http://localhost:8080/api/crawl/ipost1/status?runId=RUN_xxx
```

## 数据字段映射

| iPost1 HTML | 选择器 | Mailbox 字段 |
|------------|--------|--------------|
| 街道地址 | `.store-street-address` | AddressRaw.Street |
| 城市州邮编 | `.store-city-state-zip` | City/State/Zip |
| 价格 | `.store-plan-desktop b` | Price |
| 链接 | `a[href*="secure_checkout"]` | Link |

## 预期结果

- 地点数量：~4000 个
- 覆盖范围：53 个州/地区（包括 DC、波多黎各、关岛）
- 完成时间：15-20 分钟（不含 Smarty 验证）

## 注意事项

1. **Cloudflare 保护**：必须先访问主页建立 Session
2. **HTML 解析**：地点数据在 HTML 片段中，需要 goquery
3. **速率限制**：建议每个州间隔 2-3 秒
4. **内存管理**：chromedp 消耗较多内存，设置合理超时

---

**完整实现方案**：[ipost1_final_implementation_plan.md](./ipost1_final_implementation_plan.md)
**参考文档**：[US_VirtualBox_Non-CMRA_Verification_prd_en.md](./US_VirtualBox_Non-CMRA_Verification_prd_en.md)
