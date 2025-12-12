# Mailboxes 页面加载优化 - 技术方案总结

## 1. 问题分析

| 问题 | 现象 | 根因 |
|------|------|------|
| **加载速度慢** | 页面响应时间长 | 后端分页查询需要遍历所有文档计算 total（O(n)） |
| **重复请求** | 每次加载 2 次 API 请求 | 前端同时调用 `/api/stats`（获取州列表）和 `/api/mailboxes`（获取数据） |
| **无缓存** | 切换筛选条件后返回，重新请求 | 原生 fetch + useState，无缓存机制 |

---

## 2. 可行方案对比

### 问题1：后端分页计数慢

| 方案 | 描述 | 优点 | 缺点 |
|------|------|------|------|
| **A. Firestore Count API** ✅ | 使用原生聚合查询 | O(1) 复杂度，不读取文档内容 | 需要 SDK v1.11+ |
| B. 预计算统计 | 爬虫运行后更新 stats 文档 | 查询极快 | 需要维护一致性，筛选条件组合多 |
| C. 估算总数 | 使用采样估算 | 快 | 不精确，用户体验差 |

### 问题2：两次 API 请求

| 方案 | 描述 | 优点 | 缺点 |
|------|------|------|------|
| A. 合并 API | 新建 `/api/mailboxes-init` | 减少一次往返 | 增加后端复杂度 |
| **B. 硬编码选项** ✅ | 前端常量化州/RDI/Source | 完全消除请求，零延迟 | 新增州需手动更新（极少发生） |
| C. 首次加载缓存 | localStorage 存储州列表 | 后续访问快 | 首次仍需请求 |

### 问题3：无缓存机制

| 方案 | 描述 | 优点 | 缺点 |
|------|------|------|------|
| **A. React Query** ✅ | 内存缓存 + 自动管理 | 开箱即用，生态成熟 | 引入新依赖 |
| B. SWR | Vercel 的数据获取库 | 轻量 | 功能不如 React Query 丰富 |
| C. 手动 localStorage | 自己实现缓存逻辑 | 无依赖 | 需要处理序列化、过期、失效等 |
| D. Redux + RTK Query | 全局状态 + 缓存 | 功能强大 | 过于重量级 |

---

## 3. 方案选择理由

### 选择 Firestore Count API
- **匹配度高**：Firestore SDK v1.20.0 原生支持
- **改动小**：仅修改 ~10 行代码
- **效果显著**：从 O(n) 降为 O(1)

### 选择硬编码选项
- **业务特点**：美国州列表固定（50州+6领地），几乎不变
- **零成本**：完全消除网络请求
- **可维护**：集中在 `constants.ts`，清晰易改

### 选择 React Query
- **业务匹配**：月度更新的数据，非常适合长缓存策略
- **功能完善**：staleTime、gcTime、placeholderData 等开箱即用
- **社区成熟**：React 生态主流方案，长期维护

---

## 4. 实现细节

### 4.1 后端：Firestore Count API

**文件**: `apps/api/internal/repository/mailbox_repo.go`

```go
// 优化前：迭代所有文档计数 O(n)
total := 0
iter := query.Documents(ctx)
for {
    _, err := iter.Next()
    if err == iterator.Done { break }
    total++
}

// 优化后：Firestore Count API O(1)
countQuery := query.NewAggregationQuery().WithCount("total")
countResult, err := countQuery.Get(ctx)
countValue := countResult["total"].(*firestorepb.Value)
total := int(countValue.GetIntegerValue())
```

### 4.2 前端：React Query 配置

**文件**: `apps/web/index.tsx`

```typescript
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30 * 60 * 1000, // 30 minutes - data rarely changes (monthly crawl)
      gcTime: 60 * 60 * 1000,    // 1 hour - keep in cache even after unmount
      refetchOnWindowFocus: false,
      refetchOnReconnect: false,
    },
  },
});
```

| 参数 | 值 | 含义 |
|------|-----|------|
| `staleTime` | 30 分钟 | 数据在 30 分钟内被认为是"新鲜的"，不会重新请求 |
| `gcTime` | 1 小时 | 组件卸载后缓存保留 1 小时（用户返回页面时可复用） |
| `refetchOnWindowFocus` | false | 切换浏览器标签页回来时不重新请求 |
| `refetchOnReconnect` | false | 网络重连时不重新请求 |

### 4.3 前端：硬编码常量

**文件**: `apps/web/constants.ts`

```typescript
export const US_STATES = [
  { code: 'AL', name: 'Alabama' },
  { code: 'CA', name: 'California' },
  // ... 50 states + 6 territories
] as const;

export const SOURCE_OPTIONS = ['ATMB', 'iPost1'] as const;
export const RDI_OPTIONS = ['Residential', 'Commercial'] as const;
```

---

## 5. 验证与量化

### 5.1 后端优化验证

```bash
# 启动 API 服务
cd apps/api && go run ./cmd/server

# 测试分页请求（观察响应时间）
curl -w "\nTime: %{time_total}s\n" \
  "http://localhost:8080/api/mailboxes?page=1&pageSize=10&active=true"
```

**预期指标**：

| 指标 | 优化前 | 优化后 |
|------|--------|--------|
| 响应时间（4000条数据） | ~500-800ms | ~100-200ms |
| Firestore 读取次数 | n + pageSize | 1 (count) + pageSize |

### 5.2 前端优化验证

1. **打开 DevTools → Network 面板**

2. **验证请求数量**：
   - 优化前：2 次请求（`/api/stats` + `/api/mailboxes`）
   - 优化后：1 次请求（`/api/mailboxes`）

3. **验证缓存命中**：
   - 切换到 State = "CA"，观察请求
   - 切换到 State = "NY"，观察请求
   - 切换回 State = "CA"，**应该没有新请求**（缓存命中）

4. **验证 gcTime**：
   - 跳转到其他页面
   - 返回 Mailboxes 页面
   - **应该没有新请求**（30分钟内）

### 5.3 量化指标总结

| 指标 | 优化前 | 优化后 | 改善 |
|------|--------|--------|------|
| 页面加载请求数 | 2 | 1 | **-50%** |
| 后端计数复杂度 | O(n) | O(1) | **显著** |
| 重复筛选条件请求 | 每次都请求 | 30分钟内复用 | **-100%** |
| 页面返回请求 | 每次都请求 | 1小时内复用 | **-100%** |

---

## 6. 修改文件清单

| 文件 | 修改类型 | 说明 |
|------|---------|------|
| `apps/api/internal/repository/mailbox_repo.go` | 修改 | 使用 Firestore Count API |
| `apps/web/package.json` | 修改 | 添加 @tanstack/react-query |
| `apps/web/index.tsx` | 修改 | 配置 QueryClientProvider |
| `apps/web/constants.ts` | 新增 | 硬编码常量（州、Source、RDI） |
| `apps/web/pages/Mailboxes.tsx` | 修改 | React Query + 使用常量 |
