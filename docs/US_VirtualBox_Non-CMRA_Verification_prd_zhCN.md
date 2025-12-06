## 美国虚拟地址验证系统（React + Go + Firestore）

### 1. 背景 & 目标

本系统用于自动化爬取 AnytimeMailbox（ATMB）上的美国地址数据，并通过 Smarty API 进行地址验证（尤其是 CMRA / RDI），最终提供可视化管理后台与 CSV 导出。

本次升级目标：

- 前端迁移到 React + Vite + Tailwind + DaisyUI  
- 后端采用 Go + Firestore NoSQL  
- 重写 API 结构，使系统更现代化、轻量、易部署  
- 使用全部免费方案实现在线部署  

### 2. 系统架构（新版）

```
Frontend (React + Vite + Tailwind)
        ↓ REST API
Backend (Go + Gin/Fiber)
        ↓ Firestore SDK
Firestore (NoSQL)
```

部署方案：

- Frontend → Vercel（免费）
- Backend → Render Free Instance（Go）
- Firestore → Firebase Free Tier

### 3. 功能概述（与旧版一致但重构数据流程）

#### 3.1 自动爬取地址数据
- 按州爬取 ATMB 门店列表  
- 并行爬取门店详情页  
- 清洗数据：名称、地址、价格、链接等  
- 去重并更新 Firestore  

#### 3.2 Smarty 地址验证
- 调用 Smarty Address API  
- 返回：CMRA（是否商业地址）、RDI（Residential/Commercial）、标准化地址（ZIP+4）  
- 多账号轮询、配额冷却机制  
- 验证失败时自动重试  

#### 3.3 后台管理页面（React）
- 地址列表表格（分页、搜索、过滤）  
- 查看 CMRA / RDI 标记  
- 导出 CSV  
- 控制后台爬虫执行  
- 展示爬虫运行日志  

### 4. Firestore 数据结构（Document NoSQL）

#### 4.1 `mailboxes` 集合
```json
{
  "name": "ABC Mailbox Store",
  "street": "123 Main St",
  "city": "San Francisco",
  "state": "CA",
  "zip": "94105",
  "price": 12.99,
  "link": "https://anytimemailbox.com/...",
  "cmra": "N",
  "rdi": "Residential",
  "standardizedAddress": {
    "deliveryLine1": "",
    "lastLine": ""
  },
  "lastValidatedAt": "2025-01-01T12:00:00Z",
  "crawlRunId": "RUN_2025_01_01_001"
}
```

索引：`state, city` / `cmra, rdi` / `crawlRunId`

#### 4.2 `crawl_runs` 集合
```json
{
  "startedAt": "2025-01-01T00:00:00Z",
  "finishedAt": "2025-01-01T00:12:00Z",
  "status": "success",
  "totalFound": 2300,
  "totalValidated": 2200,
  "totalFailed": 100,
  "errorsSample": [
    { "link": "...", "reason": "Smarty 402" }
  ]
}
```

### 5. 后端设计（Go + Firestore）

#### 5.1 目录结构（示例）
```
backend/
  cmd/
    server/main.go
  internal/
    http/
      handler/
      router.go
    service/
      crawl/
      mailbox/
    repository/
      mailbox_repo.go
      crawl_repo.go
    client/
      smarty/
      atmb/
    config/
    model/
  pkg/
    logger/
    csv/
```

#### 5.2 API 设计（REST）
1) 获取地址列表  
`GET /api/mailboxes`

参数：

| 参数 | 示例值 | 说明 |
| ---- | ------ | ---- |
| page | 1 | 分页 |
| pageSize | 50 | 每页数量 |
| state | CA | 过滤地区 |
| cmra | N | 是否商业地址 |
| rdi | Residential | 住宅/商业 |

响应：
```json
{ "items": [...], "total": 2280, "page": 1 }
```

2) 导出 CSV  
`GET /api/mailboxes/export` 返回 `text/csv`

3) 手动触发爬虫  
`POST /api/crawl/run`  
响应：`{ "runId": "RUN_001" }`

4) 获取爬虫状态  
`GET /api/crawl/status`

#### 5.3 Go 后端关键组件
- Firestore Client（`firestore.NewClient(ctx, projectID)`）  
- Worker Pool 实现爬虫并发（控制州页/详情页并发，限制 Smarty 频率）  
- Smarty API 多账号轮询（429/402 冷却，最多重试 3 次）  
- 单一二进制部署（Render）：`main.go` 启动 HTTP server，内含 Firestore SDK、Cron（可选）、Crawler Service  

### 6. 前端设计（React + Vite + Tailwind + DaisyUI）

#### 6.1 目录结构
```
frontend/
  src/
    api/
    components/
    pages/
    hooks/
    types/
    utils/
    main.tsx
    App.tsx
```

#### 6.2 UI 技术选型

| 技术 | 用途 |
| ---- | ---- |
| Vite | 极速开发 |
| React + Hooks | UI 框架 |
| Tailwind CSS | 原子化 CSS |
| DaisyUI | UI 组件库 |
| React Icons | 图标库 |
| TanStack Table（可选） | 高性能表格 |

#### 6.3 页面规划
- MailboxesPage：表格展示地址列表；过滤 state/cmra/rdi；分页；按州统计（可选）；导出 CSV  
- CrawlStatusPage：展示最近几次爬虫运行；查看错误日志；手动触发爬虫按钮  

### 7. 部署方案（全免费）

#### 7.1 前端部署（Vercel）
- 连接 GitHub 自动部署  
- 环境变量指向 Render 后端 API  

#### 7.2 后端部署（Render，Go 免费实例）
- 使用 Render 免费 Go 服务  
- 自动构建 Go 程序  
- 配置 Firestore 凭证 JSON（Base64 格式）  

#### 7.3 Firestore（Firebase Free Tier）
- 5GB 存储，50k reads/day 免费，足够使用  

### 8. 后续可扩展功能
- 全局错误监控（Sentry）  
- 分布式限流（用于 Smarty 多账号）  
- 添加管理员登录（Firebase Auth）  
- 数据可视化仪表盘（按州聚合）  
