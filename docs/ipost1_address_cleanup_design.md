# iPost1 地址数据清洗方案设计

> 文档版本: v1.0
> 创建日期: 2025-12-12
> 状态: 已实现

---

## 1. 问题描述

### 1.1 现象

iPost1 爬虫抓取的邮箱数据中，地址字段包含 HTML 残留：

```json
{
  "addressRaw": {
    "street": "1601 29th St. Suite 1292                <\\/span>\n                \n                Boulder, CO 80301<\\/span>\n                United States<\\/span>            <\\/p>\n                    <\\/div>",
    "city": "Boulder",
    "state": "CO",
    "zip": "80301<\\/span>"
  },
  "link": "https:\\/\\/ipostal1.com\\/secure_checkout.php?stID=5351"
}
```

### 1.2 根因分析

```
数据流: iPost1 API → client.go → parser.go → Firestore
```

| 层级 | 问题 | 影响 |
|------|------|------|
| **API 响应** | JSON 中 HTML 闭合标签被转义为 `<\/span>` | 源数据格式问题 |
| **client.go** | 未将 `<\/` 转换为 `</` | goquery 无法解析 |
| **parser.go** | 解析失败时 fallback 返回原始 HTML | 脏数据被存储 |

### 1.3 影响范围

- **受影响记录**: ~600 条 iPost1 邮箱数据
- **字段**: `addressRaw.street`, `addressRaw.zip`, `link`, `standardizedAddress`
- **业务影响**: 前端显示异常，地址验证可能失败

---

## 2. 可行方案分析

### 方案 A: 完善爬虫解析逻辑

**思路**: 在 `parser.go` 中修复 HTML 解析，确保输出干净数据。

```
[爬虫] → [完美解析] → [干净数据] → [存储]
```

| 优点 | 缺点 |
|------|------|
| 从源头解决问题 | iPost1 API 格式可能随时变化 |
| 无需后处理 | 需要深入理解 API 响应结构 |
| | 历史脏数据仍需处理 |
| | 调试成本高，容易引入新 bug |

**评估**: ⚠️ 风险较高，且无法解决历史数据问题

---

### 方案 B: API 返回时清洗

**思路**: 在 API 层返回数据前进行清洗。

```
[存储脏数据] → [API 查询] → [清洗] → [返回干净数据]
```

| 优点 | 缺点 |
|------|------|
| 不修改存储数据 | 每次查询都有额外开销 |
| 实现简单 | 存储层仍是脏数据 |
| | CSV 导出等场景需要额外处理 |

**评估**: ⚠️ 治标不治本，性能有损耗

---

### 方案 C: 清洗层 + 迁移脚本（当前方案）

**思路**: 创建独立的清洗工具函数，用于：
1. 迁移脚本清理历史数据
2. 爬虫存储前调用清洗

```
┌─────────────────────────────────────────────────────────┐
│                    cleanAddress()                        │
│         统一清洗函数，服务于多个场景                        │
└─────────────────────────────────────────────────────────┘
         ↑                              ↑
         │                              │
┌────────┴────────┐          ┌─────────┴─────────┐
│  迁移脚本        │          │  爬虫存储前        │
│  清理历史数据    │          │  预防新脏数据      │
└─────────────────┘          └───────────────────┘
```

| 优点 | 缺点 |
|------|------|
| 历史数据一次性修复 | 需要执行迁移 |
| 新数据自动清洗 | 清洗逻辑需维护 |
| 单一职责，易测试 | |
| 不依赖爬虫解析完美 | |
| 符合 ETL 最佳实践 | |

**评估**: ✅ 推荐方案

---

## 3. 方案选择理由

### 3.1 为什么不选方案 A（完善爬虫）

1. **源数据不可控**: iPost1 是第三方网站，API 响应格式可能随时变化
2. **完美解析是陷阱**: 试图覆盖所有边界情况会导致代码复杂度爆炸
3. **历史债务**: 即使修复爬虫，已存储的 ~600 条脏数据仍需处理
4. **调试成本**: 涉及 chromedp、goquery、JSON 解析多层，定位问题困难

### 3.2 为什么不选方案 B（API 层清洗）

1. **性能开销**: 每次 API 请求都需要清洗，无谓消耗
2. **数据不一致**: 存储层和展示层数据不一致，容易引发问题
3. **覆盖不全**: CSV 导出、Firestore 直接查询等场景无法受益

### 3.3 为什么选择方案 C（清洗层）

1. **符合 ETL 最佳实践**: Extract → Transform → Load，清洗属于 Transform 阶段
2. **防御性编程**: 假设源数据永远可能有问题，在入库前统一处理
3. **单一职责**: `cleanAddress()` 只负责清洗，易于测试和维护
4. **一次投入，多处受益**: 同一函数服务于迁移和新数据
5. **可观测**: 迁移脚本提供 dry-run 模式，可预览变更

---

## 4. 实现方案

### 4.1 核心组件

```
apps/api/
├── pkg/util/
│   └── address_cleaner.go      # 清洗工具函数
├── cmd/
│   └── migrate-clean-addresses/
│       └── main.go             # 迁移脚本
└── internal/business/crawler/ipost1/
    └── discovery.go            # 集成清洗调用
```

### 4.2 清洗逻辑

```go
func cleanField(s string) string {
    // 1. 修复转义 HTML 标签: <\/ -> </
    s = strings.ReplaceAll(s, `<\/`, `</`)

    // 2. 修复转义斜杠
    s = strings.ReplaceAll(s, `\/`, `/`)

    // 3. 移除 HTML 标签
    s = htmlTagPattern.ReplaceAllString(s, "")

    // 4. 解码 HTML 实体
    s = strings.ReplaceAll(s, "&amp;", "&")
    // ...

    // 5. 移除无关文本
    s = strings.ReplaceAll(s, "United States", "")

    // 6. 规范化空白
    s = multiSpacePattern.ReplaceAllString(s, " ")

    return strings.TrimSpace(s)
}
```

### 4.3 调用时机

| 场景 | 调用位置 | 说明 |
|------|----------|------|
| **历史数据迁移** | `migrate-clean-addresses` | 一次性执行 |
| **新爬取数据** | `discovery.go` 存储前 | 自动清洗 |

---

## 5. 验证与量化

### 5.1 迁移前验证（Dry-Run）

```bash
# 预览将要清洗的数据
make migrate-clean-dry
```

**输出示例**:
```
=== Address Cleanup Migration [DRY-RUN] ===
Source filter: iPost1
==========================================

Scanning mailboxes collection...
Found 612 mailbox documents

--- Sample 1: iPost1 - Boulder, CO ---
BEFORE:
  Street: "1601 29th St. Suite 1292                </span>..."
  Zip:    "80301</span>"
AFTER:
  Street: "1601 29th St. Suite 1292"
  Zip:    "80301"

=== Analysis Summary ===
Total documents:    612
Need cleanup:       612
Already clean:      0

[DRY-RUN] Would update 612 documents.
```

### 5.2 量化指标

| 指标 | 测量方法 | 预期结果 |
|------|----------|----------|
| **清洗覆盖率** | `NeedsCleanup()` 函数检测 | 迁移后 0 条需要清洗 |
| **数据完整性** | 比较清洗前后字段非空率 | 无数据丢失 |
| **字段长度** | 统计 `street` 字段平均长度 | 显著减少（去除 HTML） |
| **前端渲染** | 人工检查 Mailboxes 页面 | 地址显示正常 |

### 5.3 回归验证

```bash
# 1. 执行迁移
make migrate-clean

# 2. 验证 API 返回
curl "http://localhost:8080/api/mailboxes?source=iPost1&pageSize=5" | jq '.items[0].addressRaw'

# 3. 检查是否还有脏数据
make migrate-clean-dry  # 应显示 "Need cleanup: 0"
```

### 5.4 预期清洗效果

| 字段 | 清洗前 | 清洗后 |
|------|--------|--------|
| `street` | `1601 29th St</span>...` | `1601 29th St. Suite 1292` |
| `zip` | `80301</span>` | `80301` |
| `link` | `https:\/\/ipostal1.com\/...` | `https://ipostal1.com/...` |

---

## 6. 操作手册

### 6.1 清理历史数据

```bash
# Step 1: 预览（必须先执行）
make migrate-clean-dry

# Step 2: 确认输出无误后执行
make migrate-clean

# Step 3: 验证
make migrate-clean-dry  # 确认 "Need cleanup: 0"
```

### 6.2 后续爬取

无需额外操作，清洗已集成到爬虫流程：

```bash
# 触发爬取，数据会自动清洗后存储
curl -X POST http://localhost:8080/api/crawl/ipost1/run
```

---

## 7. 总结

| 维度 | 说明 |
|------|------|
| **问题** | iPost1 数据包含 HTML 残留 |
| **方案** | 清洗层 + 迁移脚本 |
| **选择理由** | 防御性编程、符合 ETL 实践、一次投入多处受益 |
| **验证方法** | dry-run 预览 + 迁移后回归检查 |
| **影响范围** | ~612 条历史数据 + 后续所有新数据 |

---

## 附录：相关文件

- [address_cleaner.go](../apps/api/pkg/util/address_cleaner.go) - 清洗工具函数
- [migrate-clean-addresses/main.go](../apps/api/cmd/migrate-clean-addresses/main.go) - 迁移脚本
- [discovery.go](../apps/api/internal/business/crawler/ipost1/discovery.go) - 爬虫集成点
