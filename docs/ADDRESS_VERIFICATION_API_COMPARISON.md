# 地址验证 API 服务对比 (2025)

## 目标需求

- ✅ 美国地址验证和标准化
- ✅ **RDI (Residential Delivery Indicator)** - 区分住宅/商业地址
- ✅ **CMRA (Commercial Mail Receiving Agency)** - 识别虚拟邮箱/商业邮件接收机构

---

## 📊 主流服务对比

### 1. **Geocodio** - 最佳性价比 (仅 RDI)

#### 定价

- **免费额度**: 2,500 次查询/天 (~75,000 次/月)
- **按需付费**: $0.50 / 1,000 次查询
- **无限制套餐**: $1,000/月 (最多 5M 次/天，批量处理优化)

#### RDI & CMRA 支持

- ✅ **RDI**: 通过 `zip4` 字段 append 获取
  - `residential: true` = 住宅地址
  - `residential: false` = 商业地址
  - `residential: null` = 未知
- ❌ **CMRA**: **不支持** (经文档确认)

#### API 示例

```bash
GET https://api.geocod.io/v1.9/geocode?q=750+W+Dimond+Blvd,+Anchorage+AK&fields=zip4&api_key=YOUR_KEY
```

```json
{
  "zip4": {
    "residential": false,
    "record_type": { "code": "S", "description": "Street" },
    "valid_delivery_area": true,
    "exact_match": true
  }
}
```

#### 优点

- ✅ **无月度订阅要求** - 真正的按需付费
- ✅ **慷慨的免费额度** - 每天 2,500 次足够小型项目
- ✅ 简单明了的定价结构
- ✅ RDI 功能免费包含

#### 缺点

- ❌ **不支持 CMRA 检测** - 无法识别虚拟邮箱
- ⚠️ RDI 功能相对较新 (2024 年 11 月添加)
- ⚠️ 文档可能不如 Smarty 详细

#### 适用场景

- **小到中型项目** - 免费额度足够
- **初创公司** - 无月度承诺，成本可控
- **预算敏感项目** - 最低的按需付费价格

#### 成本估算

| 每月查询次数 | 月度成本      |
| ------------ | ------------- |
| 0 - 75,000   | **$0** (免费) |
| 100,000      | **$12.50**    |
| 500,000      | **$212.50**   |
| 1,000,000    | **$462.50**   |

**参考**:

- [Geocodio 定价](https://www.geocod.io/pricing/)
- [Geocodio RDI 功能公告](https://www.geocod.io/updates/2024-11-04-added-residential-delivery-indicator-to-zip4/)

---

### 2. **Smarty (原 SmartyStreets)** - 功能最全面

#### 定价

- **免费额度**: 250 次查询/月
- **基础套餐**: $50/月 (5000 次查询)
- **无限制**: $1,000/月

#### RDI & CMRA 支持

- ✅ **RDI**: 包含在 55+ 元数据字段中
- ✅ **CMRA**: 完整支持
- ✅ 成熟稳定的功能

#### 优点

- ✅ 最全面的元数据 (55+ 字段)
- ✅ 功能最成熟，文档最完善
- ✅ 企业级稳定性
- ✅ 优秀的 API 设计
- ✅ 包含经纬度、县 FIPS 代码、carrier route 等

#### 缺点

- ❌ **需要付费订阅才能获取 RDI/CMRA** (免费 250 次不包含)
- ❌ 相对较贵
- ❌ 免费额度极小

#### 适用场景

- **企业级应用** - 需要稳定性和完整功能
- **高查询量** - 无限制套餐性价比高
- **需要丰富元数据** - 55+ 字段

#### 成本估算

| 每月查询次数 | 月度成本                |
| ------------ | ----------------------- |
| 0 - 250      | **$0** (免费，但无 RDI) |
| 500          | **$20**                 |
| 5,000        | **$100**                |
| 无限制       | **$1,000**              |

**参考**:

- [Smarty 定价](https://www.smarty.com/pricing)
- [Smarty vs Geocodio 对比](https://www.geocod.io/geocodio-vs-smartystreets-comparison/)

---

### 3. **PostGrid** - 国际地址支持

#### 定价

- **起步价**: $30/月 (2,000 lookups per month, $0.03 per additional lookup)
- 具体定价需联系销售

#### RDI & CMRA 支持

- ✅ **RDI**: 支持
- ✅ **CMRA**: 支持
- ✅ CASS (USPS) 认证

#### 优点

- ✅ 支持 245+ 个国家 (国际地址验证)
- ✅ CASS 和 SERP (Canada Post) 认证
- ✅ 功能丰富

#### 缺点

- ❌ 定价不透明，需要联系销售
- ❌ 可能对小型项目过于复杂

#### 适用场景

- **国际业务** - 需要验证多国地址
- **企业级** - 需要合规认证
- **加拿大业务** - SERP 认证

**参考**:

- [PostGrid 定价](https://www.postgrid.com/pricing-address-verification/)
- [PostGrid vs Smarty 对比](https://www.getapp.com/all-software/a/postgrid-address-verification/compare/smarty-1/)

---

### 4. **Experian EDQ** - 企业级方案

#### 定价

- **起步价**: $1,000/年
- **定价模式**: 订阅制，按使用量计费
- **免费试用**: 30 天，无需信用卡
- 具体定价需联系销售

#### RDI & CMRA 支持

- ✅ **RDI**: 支持 (通过 CASS 认证工具)
- ✅ **CMRA**: 应该支持 (CASS + DPV 验证)
- ✅ 250+ 国家/地区覆盖

#### 优点

- ✅ 企业级稳定性 (Experian 大品牌)
- ✅ 250+ 国家/地区覆盖
- ✅ 丰富的数据源 (USPS, Royal Mail, Australia Post)
- ✅ 预建 CRM/电商集成 (Salesforce 等)
- ✅ 同时支持云端和本地部署

#### 缺点

- ❌ **起步价高** ($1,000/年，比 Smarty 贵 4 倍)
- ❌ **定价不透明** - 需要联系销售
- ❌ **无按需付费选项**
- ⚠️ 用户评价一般 (4.0/5, 评论较少)

#### 适用场景

- **大型企业** - 需要全球地址验证
- **Salesforce 用户** - 有预建集成
- **高合规要求** - 需要多重认证

#### 成本估算

| 场景     | 年度成本    |
| -------- | ----------- |
| 基础套餐 | **$1,000+** |
| 企业定制 | 需联系销售  |

**参考**:

- [Experian Address Verification (Capterra)](https://www.capterra.com/p/267356/Experian-Address-Verification/)
- [Experian RDI 说明](https://www.edq.com/blog/how-can-i-use-address-verification-to-identify-business-versus-residential-addresses/)

---

### 5. **USPS 官方 API** - 基础免费

#### 定价

- **完全免费**
- 需要注册获取 API key

#### RDI & CMRA 支持

- ❌ **RDI**: 不直接提供 (需要单独的付费产品)
- ❌ **CMRA**: 不直接提供
- ✅ 基础地址标准化

#### 优点

- ✅ 完全免费
- ✅ 官方权威数据源
- ✅ 适合基础地址验证

#### 缺点

- ❌ **不包含 RDI/CMRA 数据**
- ❌ 严格的速率限制 (60 次/小时)
- ❌ 仅支持美国地址
- ❌ 功能有限，无元数据
- ⚠️ 旧版 Web Tools API 将于 2026 年 1 月 25 日退役

#### 适用场景

- **仅需地址标准化** - 不需要 RDI/CMRA
- **极低查询量** - 60 次/小时足够
- **预算为零** - 完全免费

**参考**:

- [USPS API 文档](https://developers.usps.com/apis)
- [USPS API 变更公告](https://www.postgrid.com/usps-api-changes-with-rate-limiting-to-60-addresses-minute-what-it-means-why-matter/)

---

## 🏆 推荐方案

### 对于本项目 (VirtualBox Verifier)

> ⚠️ **重要发现**: 经过详细文档调研，Geocodio **不支持 CMRA 检测**，只支持 RDI。
> 如果需要同时检测 RDI 和 CMRA，必须选择 Smarty 或 PostGrid。

---

#### **方案 A: 需要 RDI + CMRA** ⭐ 推荐

##### **推荐: Smarty**

- **价格**: $20/月 起 ($240/年)
- **理由**:
  1. ✅ **完全满足需求**: RDI + CMRA 都支持
  2. ✅ **价格合理**: 比 Experian ($1,000/年) 便宜 75%
  3. ✅ **功能成熟**: 55+ 元数据字段
  4. ✅ **文档优秀**: API 设计清晰
- **预估成本**: $20/月 = $240/年

##### **备选: PostGrid**

- **价格**: $18/月 起 ($216/年)
- **理由**: 价格略低，国际地址支持更好
- **缺点**: 定价不透明

---

#### **方案 B: 只需要 RDI (不需要 CMRA)**

##### **推荐: Geocodio**

- **价格**: $0 (免费额度内)
- **理由**:
  1. ✅ **免费额度充足**: 每天 2,500 次 = 每月 75,000 次
  2. ✅ **按需付费**: $0.50/1,000 次查询
  3. ✅ **性价比最高**: 相比 Smarty 便宜 ~90%
- **限制**: ❌ 不支持 CMRA 检测

**预估成本 (基于当前数据库规模)：**

- 当前数据库: ~2,069 条记录
- 一次性验证: **$0** (在免费额度内)
- 持续验证 (假设每月新增 1,000 条): **$0** (仍在免费额度内)

---

#### **方案 C: 混合方案 (推荐用于成本优化)**

##### **Geocodio (RDI) + 自建 CMRA 黑名单**

- **价格**: $0
- **实现方式**:
  1. 使用 Geocodio 获取 RDI 数据 (免费)
  2. 维护一个已知 CMRA 地址的黑名单 (如 UPS Store, FedEx Office 等)
  3. 对于新地址，手动/半自动标注 CMRA 状态
- **优点**: 完全免费
- **缺点**: CMRA 检测不完整，需要人工维护

---

#### **方案 D: 临时开发方案**

##### **Mock 模式**

- **价格**: $0
- **配置**: `SMARTY_MOCK=true`
- **理由**: 快速完成功能开发和测试
- **限制**: 非真实数据，仅用于开发

---

## 📋 功能对比表

| 功能/服务        | Geocodio | Smarty   | PostGrid | Experian  | USPS    |
| ---------------- | -------- | -------- | -------- | --------- | ------- |
| **RDI 支持**     | ✅       | ✅       | ✅       | ✅        | ❌      |
| **CMRA 支持**    | ❌       | ✅       | ✅       | ✅        | ❌      |
| **免费额度**     | 2,500/天 | 250/月   | ❌       | 30 天试用 | 60/小时 |
| **按需付费**     | ✅       | ❌       | ❌       | ❌        | ✅      |
| **起步费用**     | $0       | $20/月   | $18/月   | $1,000/年 | $0      |
| **国际地址**     | 部分     | ✅       | ✅       | ✅ (250+) | ❌      |
| **元数据丰富度** | 中       | 高 (55+) | 中       | 高        | 低      |
| **API 稳定性**   | 高       | 非常高   | 高       | 非常高    | 中      |
| **文档质量**     | 好       | 优秀     | 好       | 好        | 一般    |
| **Go SDK**       | HTTP     | HTTP     | HTTP     | HTTP      | 社区包  |

---

## 🚀 实施建议

### 短期 (立即实施)

1. **注册 Geocodio 账户** - 免费，无需信用卡
2. **获取 API Key**
3. **更新后端配置**:
   ```bash
   # .env.local
   GEOCODIO_API_KEY=your_api_key_here
   ```
4. **实现 Geocodio 客户端** (类似现有 Smarty 客户端)
5. **运行 reprocess** 使用 Geocodio API

### 中期 (1-3 个月后评估)

1. **监控 API 使用量**
2. **评估数据质量**
3. **对比与 Smarty 的差异**
4. **根据业务增长调整方案**

### 长期 (生产环境)

- 如果免费额度不够，考虑：
  - **继续使用 Geocodio** 按需付费 (成本最低)
  - **升级到 Smarty** 如果需要更多元数据
  - **混合方案** Geocodio (日常) + USPS (备份)

---

## 💡 其他考虑

### API 稳定性

- **Geocodio**: 99.9% uptime SLA
- **Smarty**: 99.999% uptime SLA (企业级)
- **USPS**: 无 SLA 保证

### 数据更新频率

- **所有服务** 都基于 USPS 官方数据
- **更新周期**: 月度更新 (跟随 USPS)
- **数据质量**: 基本一致 (都是 CASS 认证)

### 合规性

- **CASS 认证**: Geocodio, Smarty, PostGrid 都有
- **HIPAA**: Smarty 和 PostGrid 支持
- **SOC 2**: Smarty 和 PostGrid 认证

---

## 📚 参考资源

### 官方文档

- [Geocodio API 文档](https://www.geocod.io/docs/)
- [Smarty API 文档](https://www.smarty.com/docs)
- [PostGrid API 文档](https://www.postgrid.com/api/)
- [USPS API 文档](https://developers.usps.com/apis)

### 对比和评测

- [Geocodio vs Smarty 对比](https://www.geocod.io/geocodio-vs-smartystreets-comparison/)
- [PostGrid vs Smarty 对比](https://www.getapp.com/all-software/a/postgrid-address-verification/compare/smarty-1/)

### RDI 和 CMRA 资源

- [什么是 RDI?](https://www.postgrid.com/residential-delivery-indicator/)
- [什么是 CMRA?](https://www.postgrid.com/glossary/cmra/)
- [USPS RDI 官方说明](https://postalpro.usps.com/address-quality-solutions/residential-delivery-indicator-rdi)
- [USPS CMRA 官方说明](https://faq.usps.com/s/article/Commercial-Mail-Receiving-Agency-CMRA)

---

## 📊 最终推荐

### 如果需要 RDI + CMRA (完整功能)

| 排名 | 服务         | 年度成本         | 推荐理由                     |
| ---- | ------------ | ---------------- | ---------------------------- |
| 🥇   | **Smarty**   | $240/年 ($20/月) | 功能完整，价格合理，文档优秀 |
| 🥈   | **PostGrid** | $216/年 ($18/月) | 价格略低，国际支持好         |
| 🥉   | **Experian** | $1,000+/年       | 企业级，但太贵               |

### 如果只需要 RDI (不需要 CMRA)

| 排名 | 服务         | 年度成本 | 推荐理由                   |
| ---- | ------------ | -------- | -------------------------- |
| 🥇   | **Geocodio** | **$0**   | 免费额度充足，按需付费便宜 |
| 🥈   | **Smarty**   | $240/年  | 功能更全，但需要付费       |

### 临时开发方案

| 方案      | 成本 | 说明                      |
| --------- | ---- | ------------------------- |
| Mock 模式 | $0   | 快速开发测试，非真实数据  |
| USPS API  | $0   | 仅地址标准化，无 RDI/CMRA |

---

## 🎯 针对本项目的最终建议

**推荐路径**:

1. **短期**: 使用 Mock 模式完成功能开发和测试
2. **中期**: 订阅 Smarty ($20/月) 获取真实 RDI + CMRA 数据
3. **长期**: 根据使用量评估是否需要升级或更换服务

**原因**:

- 本项目核心需求是识别 **非 CMRA 的住宅地址** (Non-CMRA Residential)
- 这需要同时具备 **RDI** (区分住宅/商业) 和 **CMRA** (识别虚拟邮箱) 功能
- Geocodio 虽然免费，但**不支持 CMRA**，无法满足核心需求
- Smarty $20/月 是满足完整需求的最经济选择

---

**文档创建时间**: 2025-12-10
**最后更新**: 2025-12-13
**下次审查**: 2026-01-10 (或当查询量达到 50,000/月时)
**调研确认**: Geocodio 官方文档确认不支持 CMRA 检测
