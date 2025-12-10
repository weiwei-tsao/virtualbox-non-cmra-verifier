# 地址验证 API 服务对比 (2025)

## 目标需求
- ✅ 美国地址验证和标准化
- ✅ **RDI (Residential Delivery Indicator)** - 区分住宅/商业地址
- ✅ **CMRA (Commercial Mail Receiving Agency)** - 识别虚拟邮箱/商业邮件接收机构

---

## 📊 主流服务对比

### 1. **Geocodio** ⭐ 推荐 - 最佳性价比

#### 定价
- **免费额度**: 2,500 次查询/天 (~75,000 次/月)
- **按需付费**: $0.50 / 1,000 次查询
- **无限制套餐**: $1,000/月 (最多 5M 次/天，批量处理优化)

#### RDI & CMRA 支持
- ✅ **RDI**: 自动包含在 ZIP+4 数据中 (2024年11月新增)
- ✅ **CMRA**: 支持识别
- ✅ 无需额外费用

#### 优点
- ✅ **无月度订阅要求** - 真正的按需付费
- ✅ **慷慨的免费额度** - 每天 2,500 次足够小型项目
- ✅ 简单明了的定价结构
- ✅ RDI 功能免费包含

#### 缺点
- ⚠️ RDI 功能相对较新 (2024年11月添加)
- ⚠️ 文档可能不如 Smarty 详细

#### 适用场景
- **小到中型项目** - 免费额度足够
- **初创公司** - 无月度承诺，成本可控
- **预算敏感项目** - 最低的按需付费价格

#### 成本估算
| 每月查询次数 | 月度成本 |
|------------|---------|
| 0 - 75,000 | **$0** (免费) |
| 100,000 | **$12.50** |
| 500,000 | **$212.50** |
| 1,000,000 | **$462.50** |

**参考**:
- [Geocodio 定价](https://www.geocod.io/pricing/)
- [Geocodio RDI 功能公告](https://www.geocod.io/updates/2024-11-04-added-residential-delivery-indicator-to-zip4/)

---

### 2. **Smarty (原 SmartyStreets)** - 功能最全面

#### 定价
- **免费额度**: 250 次查询/月
- **基础套餐**: $20/月 (500 次查询)
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
| 每月查询次数 | 月度成本 |
|------------|---------|
| 0 - 250 | **$0** (免费，但无 RDI) |
| 500 | **$20** |
| 5,000 | **$100** |
| 无限制 | **$1,000** |

**参考**:
- [Smarty 定价](https://www.smarty.com/pricing)
- [Smarty vs Geocodio 对比](https://www.geocod.io/geocodio-vs-smartystreets-comparison/)

---

### 3. **PostGrid** - 国际地址支持

#### 定价
- **起步价**: $18/月
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

### 4. **USPS 官方 API** - 基础免费

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
- ⚠️ 旧版 Web Tools API 将于 2026年1月25日退役

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

#### **推荐: Geocodio** ⭐⭐⭐⭐⭐

**理由：**
1. ✅ **完全满足需求**: RDI + CMRA 支持
2. ✅ **免费额度充足**: 每天 2,500 次 = 每月 75,000 次
3. ✅ **按需付费**: 不需要月度承诺
4. ✅ **价格透明**: $0.50/1,000 次查询
5. ✅ **性价比最高**: 相比 Smarty 便宜 ~90%

**预估成本 (基于当前数据库规模)：**
- 当前数据库: ~2,069 条记录
- 一次性验证: **$0** (在免费额度内)
- 持续验证 (假设每月新增 1,000 条): **$0** (仍在免费额度内)

**Go 集成：**
```go
// 可以使用 HTTP 直接调用，或寻找社区包装器
// API 文档: https://www.geocod.io/docs/
```

---

### 备选方案

#### **方案 A: Smarty**
- **何时选择**:
  - 查询量超大 (>500万/月)
  - 需要 55+ 元数据字段
  - 企业级稳定性要求
- **成本**: $1,000/月 (无限制)

#### **方案 B: USPS + 手动标注**
- **何时选择**:
  - 完全没有预算
  - 可以接受无 RDI/CMRA 数据
  - 手动标注虚拟邮箱
- **成本**: $0

---

## 📋 功能对比表

| 功能/服务 | Geocodio | Smarty | PostGrid | USPS |
|----------|---------|--------|----------|------|
| **RDI 支持** | ✅ | ✅ | ✅ | ❌ |
| **CMRA 支持** | ✅ | ✅ | ✅ | ❌ |
| **免费额度** | 2,500/天 | 250/月 | ❌ | 无限 (60/小时) |
| **按需付费** | ✅ | ❌ | ❌ | ✅ |
| **月度最低费用** | $0 | $20 | $18 | $0 |
| **国际地址** | 部分 | ✅ | ✅ | ❌ |
| **元数据丰富度** | 中 | 高 (55+) | 中 | 低 |
| **API 稳定性** | 高 | 非常高 | 高 | 中 |
| **文档质量** | 好 | 优秀 | 好 | 一般 |
| **Go SDK** | HTTP | HTTP | HTTP | 社区包 |

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

### 中期 (1-3个月后评估)
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

### 🥇 第一选择: **Geocodio**
- **价格**: 免费开始，$0.50/1,000 次查询
- **理由**: 性价比最高，功能完整，免费额度慷慨
- **风险**: 低 - 按需付费，无长期承诺

### 🥈 第二选择: **Smarty**
- **价格**: $20/月起
- **理由**: 企业级稳定性，功能最全
- **风险**: 中 - 需要月度订阅

### 🥉 第三选择: **USPS (临时方案)**
- **价格**: 免费
- **理由**: 临时使用，开发阶段
- **风险**: 高 - 无 RDI/CMRA，速率限制严格

---

**文档创建时间**: 2025-12-10
**最后更新**: 2025-12-10
**下次审查**: 2026-01-10 (或当查询量达到 50,000/月时)
