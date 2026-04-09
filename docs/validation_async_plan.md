# 验证流程异步化架构升级计划 (Asynchronous Validation Execution Plan)

## 核心目标
将现有的通过 HTTP 请求直接阻塞执行的“验证流程（Start Validation）”升级为**异步后台任务（Background Job）**。
旨在彻底解决因为前台网络超时导致连接断开 (`context.Canceled`)，从而导致验证任务被异常迫停并卡在 `running` 状态的技术隐患。

---

## 具体实施步骤

### 阶段 1：后端路由层改造（上下文分离与异步触发）
*文件: `apps/api/internal/platform/http/router.go`*

当前的代码在 `runValidation` Handler 里直接调用服务层，如果浏览器超时断开，它的 `c.Request.Context()` 就会被自动注销（取消），从而阻断数据库写入和远程 API 请求。

1. **上下文脱离**: 创建一个独立的 `context.Background()`，该上下文的存活期与 HTTP 请求无关。
2. **Goroutine 异步抛出**: 将 `r.validationSvc.ProcessPendingValidations` 包装在 `go func() {...}()` 中后台执行。
3. **即时返回**: 主 HTTP 处理方法直接返回 `HTTP 200`，不仅不会超时，还让前端顺滑秒接并展现处理状态：
   ```json
   {
       "message": "Validation job started in the background. It will process all pending items."
   }
   ```

### 阶段 2：服务层并发安全改造（防抢占机制）
*文件: `apps/api/internal/business/validation/service.go`*

当我们将处理改为后台执行后，系统会面临“**用户连续狂点多次按钮**”或者“**定时任务刚好撞车**”导致触发并发验证的风险。由于现有的代码中 `ValidationService` 类通过内置局部变量 (`s.currentRunID`, `s.itemsSample`) 追踪状态，不加锁的并发会导致内存数据冲突 (Data Race)。

1. **引入互斥锁 (Mutex)**：在 `ValidationService` 结构体中添加 `mu sync.Mutex` 以及一个状态标志位 `isRunning bool`。
2. **防重入阻断**: 
   - 每次进入 `ProcessPendingValidations` 时，先检测是否 `isRunning == true`。
   - 如果已经在运行了，直接拦截并记录日志，安全退出本次重入调用：“There is already an active validation run going on.”。
3. **状态保证**: 利用 Go 的 `defer` 关键字确保该标志位在服务正常工作结束或发生 Panic 时自动恢复到 `false`。

### 阶段 3：前端联调适配体验
*文件: `apps/web/pages/Crawler.tsx`*

目前前端会等请求返回后直接弹出 `Validation completed`。但在异步升级后，任务其实只是刚开始。
现有其实已经带有基于 `runId` 监听的骨架了。我们只需要调优一下感知体验：
1. HTTP 侧收到 `message: Validation started...` 后直接结束按钮 Loading 状态，弹出此气泡提示。
2. Frontend 中自带的 `validationRuns` 状态如果感知到 `running`，它本来就会自己每 5 秒刷新任务板。
3. **补充**：为了在运行期间也能看到右上角的四个统计数字（`pending`, `needsRevalidation`, `validated` 等）实时拨动，我们可以给 `validationStats` 的获取加回那 10秒 轮询的机制，但增加条件拦截：**即“当且仅当存在状态为 `running` 的 Job 时，才保持对数据库的查询”**，完美兼顾实时展示与资源节省。

---

## 预期成效
- 解决大于百位数量级数据触发带来的“白屏、旋转卡死、进程中断卡 Bug”现象。
- 支持十万级以上的数据单次发送（扔进去后台慢慢跑，不锁死 UI 线程）。
- 保障 Smarty API 的发送可靠性，告别“手动任务死掉，完全依赖 Cron 捡漏补刀”的不合理机制。
