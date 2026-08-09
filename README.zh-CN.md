<p align="center">
  <img src="docs/logo.svg" alt="Reasonix" width="640"/>
</p>

> ⚠️ **非官方定制版** —— 基于 [esengine/DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix)（`main-v2` 分支，MIT 协议）的二次开发，与官方项目无关。若你只想用稳定官方版，请前往官方仓库。

<p align="center">
  <strong>简体中文</strong>
</p>

---

# Reasonix 定制版（DeepSeek-Reasonix-Custom）

一个为对话管理做了大量增强的 Reasonix 桌面客户端。核心思路：**对话应该可以被整理** —— 排序、合并、移动、一键沉浸，而不是堆在一个列表里吃灰。

## 功能亮点

| 功能 | 说明 |
| --- | --- |
| 🗂️ **对话拖拽排序** | 侧边栏对话可上下拖动调整顺序，顺序本地保存 |
| 🔗 **合并对话** | 两个对话合成一个。支持**右键菜单**选择目标，或**拖拽悬停**自动归位、高亮两个对话、**双击**确认主对话（5 秒可选期，其他拖动可打断）。跨工作区禁止合并。合并后可 `Ctrl+Z` 撤销 |
| 📦 **跨项目移动** | 把对话移到另一个工作区，自动关闭原标签、注入系统通知提示，失败自动回滚 |
| 🤖 **AI 标题** | 最后一轮消息旁点「AI 标题」，AI 总结最近几轮对话生成新标题，预览确认后应用，可撤回 |
| ⏪ **统一撤销** | 合并 / AI 标题共用一条撤销历史，`Ctrl+Z` 连续撤销（输入框内不受影响） |
| 🧘 **纯净模式** | 标题栏「⛶」一键进入沉浸聊天：所有栏（顶栏/侧边栏/右侧面板/状态栏/工具栏）全部消失、聊天区铺满，只留消息流和输入框，`Esc` 退出并恢复原布局 |
| 🐋 **定制图标** | DeepSeek 鲸鱼图标，方便与官方版区分 |

## 安装（Windows）

### 方式一：直接下载（推荐）

1. 到 [Releases](../../releases) 下载最新的 `reasonix-desktop-custom.exe`；
2. **首次运行前**设置数据目录隔离（避免和官方版数据混在一起），新建 `ReasonixCustom.cmd` 内容如下：

   ```cmd
   @echo off
   set REASONIX_HOME=%APPDATA%\reasonix-custom
   start "" "%~dp0reasonix-desktop-custom.exe"
   ```

3. 双击运行。**API key 单独配置**（见下）。

### 方式二：源码编译

需要 Go 1.21+、Node 20+、pnpm、wails v2。

```bash
git clone https://github.com/Sighing-wind/DeepSeek-Reasonix-Custom.git
cd DeepSeek-Reasonix-Custom/desktop
wails build
# 产物: build/bin/reasonix-desktop.exe
```

## 试用 key（可选）

作者可能通过朋友圈/私聊提供**共享试用 key**。使用方式：

1. 在 [Releases](../../releases) 下载 **「模板 zip」**（不含 key，安全）并解压；
2. 双击 **「启动定制版-双击我.cmd」**，按提示**粘贴 key 后回车**（无需编辑文件）；
3. 启动器自动写入配置并启动，之后正常使用。

试用 key 为共享额度、数量有限、用尽即停；**正式使用请自行注册**（见下）。

## 首次使用：配置 API key

Reasonix 是客户端，AI 能力来自**你自己注册模型平台**拿到的 API key（不需要 Reasonix 账号）：

| 平台 | 用途 | 获取方式 |
| --- | --- | --- |
| DeepSeek 开放平台 | 主力对话模型 | platform.deepseek.com 注册 → 充值 → 创建 API key |
| Moonshot（Kimi） | 视觉/备选模型 | platform.moonshot.cn 注册 → 创建 API key |
| 智谱 AI | 备选模型 | open.bigmodel.cn 注册 → 创建 API key |

把 key 填进 `%APPDATA%\reasonix\\.env`（`DEEPSEEK_API_KEY=sk-...` 格式），或在应用内设置页配置。

## 与官方版共存 / 数据隔离

- 定制版使用独立的 `REASONIX_HOME`（如 `%APPDATA%\reasonix-custom`），**不读写官方版数据**，两者可同时安装互不干扰；
- 卸载：删除 exe 与 `%APPDATA%\reasonix-custom` 目录即可，无残留服务。

## 已知问题与风险

1. **蓝屏风险（重要）**：个别机器若装有**虚拟显示驱动**（如向日葵 OrayIddDriver、AskLink 等远程/串流软件），启动本程序可能触发蓝屏 `0xBE`（ATTEMPTED_WRITE_TO_READONLY_MEMORY）。遇到请先禁用/卸载相关虚拟显示驱动再使用（设备管理器 → 显示适配器 → 禁用）。与 Reasonix 本体无直接关系，但请知悉；
2. **合并/移动是数据操作**：虽然支持 `Ctrl+Z` 撤销，仍建议操作前在设置里备份数据目录；
3. Windows 对**未签名 exe** 可能弹出 SmartScreen 提示，选择「仍要运行」即可；
4. 基于 `main-v2` 分支开发，官方后续更新需要手动同步，功能可能落后于官方最新版；
5. 仅验证过 Windows；macOS/Linux 未测试。

## 免责声明

本项目为第三方个人修改版，**与 Reasonix 官方、DeepSeek 官方均无关联**。按 MIT 协议提供，不提供任何担保；使用过程中产生的数据丢失、系统异常等风险由使用者自行承担。官方文档与支持请前往 [esengine/DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix)。

## License

MIT —— 继承自上游 [esengine/DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix)。定制部分同样以 MIT 发布。
