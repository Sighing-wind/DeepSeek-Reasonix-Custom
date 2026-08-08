# 需求：AI 标题按钮（自动总结对话并修改对话名称）

> 版本：v1（2026-08-07 讨论拍板）
> 适用范围：Reasonix 定制版（fork 自 esengine/DeepSeek-Reasonix，定制 commit `ffd97c3` 之上新增）

## 1. 背景与目标

官方版已有"自动标题"机制（`desktop/tabs.go` 的 `maybeAutoTitleTopic` → `autoTitleTopicFromSession`），但它是纯文本规则：从用户消息里挑一条截断到 18 字当标题，**不调用 LLM**。用户手动改过标题（会话 meta 的 `CustomTitle` 非空）后，该机制对该会话永久停用。

本需求新增一个"AI 标题"按钮：点击后让模型总结最近的对话内容，生成一句话标题，弹窗预览后由用户确认应用。标题质量远高于"截取用户消息"。

## 2. 功能规格

### 2.1 位置与可见性
- 位置：每条 AI 回答下方的操作栏 `TurnActions` 组件（`desktop/frontend/src/components/Message.tsx`），与"分叉对话（fork）"按钮并排。
- **只在最后一轮回答显示**（即该回答是对话中最后一条 assistant 消息时）。`TurnActions` 已有 `isLastTurn` prop（Message.tsx:648），直接复用，无需新逻辑。
- 对话运行中（有进行中的 turn / 待处理 prompt / 后台任务）按钮禁用，提示"对话运行中"。后端必须复用 `topicHasActiveRuntimeWork` 做二次校验。

### 2.2 交互流程
1. 用户点击"AI 标题"按钮（图标建议 `Wand2` 或 `Sparkles`，文案"AI 标题"）。
2. 前端调用后端新绑定（如 `SuggestTopicTitle(scope, workspaceRoot, topicID)`）。
3. 后端读取会话**最近约 10 轮用户+助手消息**（不是整个对话），构造 prompt 调用 LLM（非流式一次性调用），要求模型只输出一句话标题，≤20 字。
4. 后端返回候选标题（字符串）给前端。
5. 前端弹出确认对话框：展示候选标题 + 当前标题，用户点"应用"才生效；可编辑候选标题后再应用；点"取消"不生效。
6. 应用时调用 `RenameTopic`（已存在）或后端在新绑定里一并写入，更新侧边栏与打开 tab 的标题，toast 提示成功。

### 2.3 标题来源标记与防覆盖
- 新标题的来源标记为**新枚举值**（建议 `topicTitleSourceAI`，字符串值如 `"ai"`），不要复用 `auto` 或 `manual`：
  - 官方自动标题机制只在 source == `auto` 时运行（`maybeAutoTitleTopic`，tabs.go:4479），新 source 保证 AI 标题**不会被自动机制覆盖**；
  - 区别于 `manual`：本按钮**可重复生成并覆盖旧标题**，包括用户手动改过的标题（即不做 `sessionHasManualDisplayTitle` 那种保护，用户已拍板）。
- 需排查新 source 对现有判断点的影响：`topicTitleFallbackForOpen`（tabs.go:4593）、`shouldApplyAutoTopicTitle`（tabs.go:4562）、`sessionHasManualDisplayTitle`（tabs.go:4576）、`isDefaultTopicTitle` 相关路径，确保新 source 不会被误判为需要 fallback/覆盖。

### 2.4 边界与异常
- 会话为空（无用户消息）：按钮禁用或提示。
- LLM 调用失败/超时：toast 报错，不改变现有标题。
- 生成标题为空或为低信号词（可复用 `lowSignalTopicTitle`，tabs.go:4702 的过滤词表）：提示"未能生成有效标题"，不应用。
- 运行中的对话：前端禁用 + 后端 `topicHasActiveRuntimeWork` 拒绝。
- 标题长度：模型输出超过 20 字时截断或要求重试（前端展示时可编辑，用户可自行删改）。

## 3. 技术实现要点

### 3.1 后端（Go，新增方法，建议放独立文件 `desktop/topic_ai_title.go`）
- 签名：`func (a *App) SuggestTopicTitle(scope, workspaceRoot, topicID string) (string, error)`，返回候选标题；应用侧复用现有 `RenameTopic`（或本方法直接写入并返回成功）。
- 读会话：`agent.LoadSession(sessionPath)`（定制版 `desktop/topic_merge_move.go` 已示范用法）；路径解析复用 `a.findTopicSessionForTarget`（同文件已有）或 `a.topicTrashTargets`。
- 取最近 N 轮：从 `sess.Snapshot()` 尾部向前收集，跳过系统消息，取最后约 10 轮 user+assistant 消息作为总结上下文（截断到合理 token 上限，如 8000 字）。
- 调 LLM：**非流式一次性调用**，不经过 agent 运行时（不要创建新 turn、不要写入对话历史）。确认 `internal/provider` 包现成的一次性调用接口（定制版 merge 代码已 import `reasonix/internal/provider`，可参考其消息结构；具体非流式调用入口需在开发时确认，如 `provider.Client.Chat` 一类）。模型选择：跟随当前对话模型，或独立配置一个轻量模型（后者更省，需用户拍板，暂定跟随当前模型）。
- 写标题：`setTopicTitleWithSource(workspaceRoot, topicID, title, topicTitleSourceAI)`（tabs.go 已有，同族函数 `setTopicTitleWithSource` 在定制版 merge 代码里也用到）。
- 刷新 UI：`a.updateOpenTopicTitle(topicID, title, source)` + `a.emitProjectTreeMetadataChanged()`（参考 `maybeAutoTitleTopic` 的收尾，tabs.go:4493-4499）。
- 二次校验：方法开头复用 `a.topicHasActiveRuntimeWork(topicID)` 拒绝运行中的对话。

### 3.2 前端（React）
- `Message.tsx` `TurnActions`：在 fork 按钮旁加一个按钮；`isLastTurn` 为 false 时不渲染。
- 新绑定声明：`desktop/frontend/src/lib/bridge.ts` 的 `AppBindings` 加 `SuggestTopicTitle`（注意同时更新 `bridgeBreadcrumb` 的正则和 `makeMockApp` 的 mock 实现，仿照定制版 MergeTopics 的做法）。
- 确认对话框：复用现有对话框组件（搜索项目内已有的 confirm dialog 模式，如合并对话框、重放确认框样式），展示候选标题（可编辑输入框）+ 当前标题 + 应用/取消。
- locales：`zh.ts` / `en.ts` / `zh-TW.ts` 三份都加（按钮文案、对话框文案、错误提示、toast）。
- 样式：`desktop/frontend/src/styles.css`，仿 `turn-actions__btn` 样式。
- 若新增 locale 导致 zh bundle 超预算，需同步调整 `desktop/frontend/scripts/check-bundle-budget.mjs` 的预算（定制版已做过一次，见 ffd97c3）。

### 3.3 测试
- Go 单测：标题生成（mock provider）、source 标记持久化、运行中拒绝、低信号词拒绝、空会话。
- 前端测试：按钮仅在最后一轮显示、确认对话框交互、mock 绑定（`makeMockApp`）。
- 参考现有测试：`desktop/tabs_topic_test.go` 的 `autoTitleTopicFromSession` 测试组、`desktop/app_session_dedup_test.go`。

## 4. 验收标准
1. 任意对话最后一条回答下方出现"AI 标题"按钮；中间轮回答下方不出现。
2. 点击后生成一句 ≤20 字标题并在弹窗预览；应用后侧边栏、打开 tab、项目树立即刷新。
3. 应用后标题不再被官方自动标题机制覆盖；再次点击按钮可生成并覆盖新标题（包括用户手动改过的）。
4. 对话运行中按钮禁用；空会话/LLM 失败/低信号词均有明确提示且不破坏现有标题。
5. 三语言文案齐全，`pnpm test`、`go test ./desktop/...` 通过，Windows 打包（wails build）成功。

## 5. 参考代码索引
| 用途 | 位置 |
|---|---|
| 按钮栏组件 | `desktop/frontend/src/components/Message.tsx`（`TurnActions`，约 627-850 行） |
| 绑定声明/mock | `desktop/frontend/src/lib/bridge.ts`（`AppBindings`，MergeTopics 在 550 行附近） |
| 自动标题机制 | `desktop/tabs.go`（`maybeAutoTitleTopic` 4462、`autoTitleTopicFromSession` 4503、`topicTitleFromText` 4713） |
| 标题写入口 | `desktop/tabs.go`（`setTopicTitleWithSource` 族）、`desktop/topic_merge_move.go`（`moveTopicTitleEntry` 示范 map 读写） |
| 运行中校验 | `desktop/topic_merge_move.go`（`topicHasActiveRuntimeWork` 使用处） |
| 会话读取 | `internal/agent`（`LoadSession` / `Snapshot`），见 `desktop/topic_merge_move.go` |
| 低信号词表 | `desktop/tabs.go`（`lowSignalTopicTitle`，4702 行） |
