# Smarty US Street Address API - 调用逻辑 Review

> 文档创建日期: 2025-12-15
> 目的: 审查当前 Smarty API 调用逻辑，确认 API 返回格式，并提出优化建议以减少 API 请求

## 1. Smarty US Street API 官方规格

### 1.1 API 端点

```
https://us-street.api.smarty.com/street-address
```

### 1.2 请求方式

| 方式 | 用途 | 限制 |
|------|------|------|
| GET | 单地址验证 | URL 参数传递地址 |
| **POST** | **批量验证** | **最多 100 个地址/请求，或 32KB** |

### 1.3 批量请求格式 (POST)

```bash
curl -X POST 'https://us-street.api.smarty.com/street-address?auth-id=xxx&auth-token=xxx' \
  -H "Content-Type: application/json; charset=utf-8" \
  --data-binary '[
    {"street": "1 Santa Claus", "city": "North Pole", "state": "AK"},
    {"street": "1 infinite loop", "city": "cupertino", "state": "CA", "zipcode": "95014"}
  ]'
```

### 1.4 响应 JSON 结构

```json
[
  {
    "input_index": 0,
    "candidate_index": 0,
    "delivery_line_1": "7451 Auburn Blvd",
    "last_line": "Citrus Heights CA 95610-2992",
    "components": {
      "primary_number": "7451",
      "street_name": "Auburn",
      "street_suffix": "Blvd",
      "city_name": "Citrus Heights",
      "state_abbreviation": "CA",
      "zipcode": "95610",
      "plus4_code": "2992"
    },
    "metadata": {
      "record_type": "H",
      "county_name": "Sacramento",
      "rdi": "Commercial",           // ← RDI 在此处
      "latitude": 38.70151,
      "longitude": -121.29049
    },
    "analysis": {
      "dpv_match_code": "D",
      "dpv_cmra": "N",               // ← CMRA 在此处
      "dpv_vacant": "N",
      "active": "Y"
    }
  }
]
```

### 1.5 关键字段位置

| 字段 | 位置 | 可能值 | 说明 |
|------|------|--------|------|
| **CMRA** | `analysis.dpv_cmra` | "Y" / "N" / "" | Commercial Mail Receiving Agency |
| **RDI** | `metadata.rdi` | "Commercial" / "Residential" / "" | Residential Delivery Indicator |

> **注意**: 之前代码错误地从 `analysis.cmra` 和 `analysis.rdi` 读取，已于 2025-12-15 修复。

---

## 2. 当前实现分析

### 2.1 代码位置

| 文件 | 功能 |
|------|------|
| `internal/platform/smarty/client.go` | Smarty API 客户端 |
| `internal/business/crawler/scraper.go` | 爬虫 + 验证流程 |
| `internal/business/crawler/reprocess.go` | 重处理流程 |

### 2.2 当前调用方式

```go
// client.go - 单地址 GET 请求
func (c *Client) ValidateMailbox(ctx context.Context, mailbox model.Mailbox) (model.Mailbox, error)
```

**问题**: 当前每次只验证 **1 个地址**，效率低下。

### 2.3 现有优化点

#### Scraper 流程 (scraper.go:119-126)
```go
// ✅ 好的: 跳过已验证且数据未变的记录
if prev.DataHash == parsed.DataHash && prev.CMRA != "" {
    stats.Skipped++
    continue
}
```

#### Reprocess 流程 (reprocess.go:120-136)
```go
// ✅ 好的: 数据未变时保留现有 CMRA/RDI
if !needsRevalidation {
    reparsed.CMRA = mb.CMRA
    reparsed.RDI = mb.RDI
}
```

### 2.4 发现的问题

#### 问题 1: 无效的跳过检查 (scraper.go:128-131)

```go
// ❌ 无效: HTML 解析后 CMRA/RDI 永远为空
needsValidation := true
if parsed.CMRA != "" && parsed.RDI != "" {
    needsValidation = false
}
```

#### 问题 2: 单地址调用效率低

```
当前: 2000 条记录 = 2000 次 API 调用
批量: 2000 条记录 = 20 次 API 调用 (批量 100)
```

---

## 3. 优化建议

### 3.1 批量 API 调用 (最重要)

**预估收益**: 减少 **95%+** API 请求

#### 实现方案

```go
// 新增批量验证方法
func (c *Client) ValidateMailboxBatch(ctx context.Context, mailboxes []model.Mailbox) ([]model.Mailbox, error) {
    const maxBatchSize = 100

    // 分批处理
    for i := 0; i < len(mailboxes); i += maxBatchSize {
        end := min(i+maxBatchSize, len(mailboxes))
        batch := mailboxes[i:end]

        // 构建 POST 请求体
        reqBody := make([]map[string]string, len(batch))
        for j, mb := range batch {
            reqBody[j] = map[string]string{
                "street":  mb.AddressRaw.Street,
                "city":    mb.AddressRaw.City,
                "state":   mb.AddressRaw.State,
                "zipcode": mb.AddressRaw.Zip,
            }
        }

        // POST 请求
        resp, err := c.postBatch(ctx, reqBody)
        // ... 处理响应，通过 input_index 匹配结果
    }
}
```

#### 官方 Go SDK 支持

Smarty 提供官方 Go SDK，内置批量支持:

```go
import "github.com/smartystreets/smartystreets-go-sdk/us-street-api"

batch := street.NewBatch()
batch.Append(&street.Lookup{Street: "123 Main", City: "Dover", State: "DE"})
batch.Append(&street.Lookup{Street: "456 Oak", City: "Newark", State: "DE"})

client.SendBatch(batch)  // 一次请求验证多个地址
```

### 3.2 修复无效的跳过逻辑

```go
// 修改 scraper.go:119-131
if prev, ok := existing[parsed.Link]; ok {
    if prev.DataHash == parsed.DataHash && prev.CMRA != "" {
        stats.Skipped++
        continue
    }
    parsed.ID = prev.ID

    // ✅ 新增: 如果已有 CMRA/RDI 且数据未变，继承现有值
    if prev.CMRA != "" && prev.RDI != "" && prev.DataHash == parsed.DataHash {
        parsed.CMRA = prev.CMRA
        parsed.RDI = prev.RDI
        parsed.StandardizedAddress = prev.StandardizedAddress
        parsed.LastValidatedAt = prev.LastValidatedAt
    }
}
```

### 3.3 优化优先级

| 优先级 | 优化项 | 预估收益 | 复杂度 | 状态 |
|--------|--------|---------|--------|------|
| 🔴 高 | 批量 API 调用 | 减少 95%+ 请求 | 中等 | 待实现 |
| 🟡 中 | 修复跳过逻辑 | 减少重复调用 | 简单 | 待实现 |
| 🟢 低 | 地址级别缓存 | 有限收益 | 复杂 | 可选 |

---

## 4. API 费用优化

### 4.1 Smarty 计费方式

- 按 **请求数** 计费，不是按地址数
- 批量请求 100 个地址 = 1 次请求费用
- 单独请求 100 个地址 = 100 次请求费用

### 4.2 当前数据规模

| 数据源 | 记录数 | 当前请求数 | 优化后请求数 |
|--------|--------|------------|--------------|
| ATMB | ~2,073 | 2,073 | ~21 |
| iPost1 | ~2,035 | 2,035 | ~21 |
| **总计** | **~4,108** | **~4,108** | **~42** |

### 4.3 避免重复调用的策略

1. **首次爬取**: 批量调用 Smarty API
2. **增量更新**: 仅对 DataHash 变化或 CMRA 为空的记录调用
3. **重处理**: 使用 `ForceRevalidate=false`，仅在数据变化时重新验证

---

## 5. 已修复的问题

### 5.1 JSON 解析错误 (2025-12-15)

**问题**: CMRA 和 RDI 字段读取位置错误

| 字段 | 错误位置 | 正确位置 |
|------|----------|----------|
| CMRA | `analysis.cmra` | `analysis.dpv_cmra` |
| RDI | `analysis.rdi` | `metadata.rdi` |

**修复文件**: `internal/platform/smarty/client.go`

```go
// 修复前
mailbox.CMRA = first.Analysis.CMRA
mailbox.RDI = first.Analysis.RDI

// 修复后
mailbox.CMRA = first.Analysis.DPVCMRA
mailbox.RDI = first.Metadata.RDI
```

---

## 6. 参考资料

- [Smarty US Street Address API 文档](https://www.smarty.com/docs/cloud/us-street-api)
- [Smarty Go SDK](https://pkg.go.dev/github.com/smartystreets/smartystreets-go-sdk/us-street-api)
- [US Address Verification 产品页](https://www.smarty.com/products/us-address-verification)

---

## 7. 下一步行动

1. [ ] 实现批量 API 调用方法 `ValidateMailboxBatch`
2. [ ] 修复 scraper.go 中的无效跳过逻辑
3. [ ] 运行 reprocess 更新现有数据的 CMRA/RDI 值
4. [ ] 考虑集成官方 Smarty Go SDK
