# VS Code 功能覆盖与验收

目标基线：VS Code 1.140.0 的工作台和公开扩展 API。gocode 是 godesktop 的持续验收应用：新功能必须同时有模型/协议测试和适用平台的原生验收。表中的“部分”表示已实现子集，不能用于宣称支持 VS Code 所有功能。

| 功能 | 当前状态 | 已验证 / 尚缺 |
| --- | --- | --- |
| Windows amd64 | 已验证 | 两种 Server runner、GOAMD64=v1、race、严格 cgo、真实 HWND/GPU、PE 架构；不等于所有设备/显卡/输入法均通过 |
| macOS Intel / Apple Silicon | CI 验证 | 原生构建、Metal smoke、VSIX 编辑验收、官方 Copilot 协议握手；付费账号 UI 验收目前在 Windows 执行 |
| 原生 UI | 部分 | Dark Modern 工作台结构、标题栏、活动栏、标签/面包屑/行号/面板/状态栏；缺完整菜单、拖放、停靠/分屏、多窗口和逐像素对照 |
| 文档编辑 | 部分 | UTF-16 坐标、版本、不可变快照、事务、LF/CRLF、选区、撤销重做、剪贴板、拖选/重复按键；缺多光标、snippet、完整 IME/字素簇/双向导航和水平滚动 |
| 文件/工作区 | 部分 | 单工作区、文本树/标签、后台快照/原子保存、未保存关闭确认、已打开可编辑文档的原生监听/原子替换/自动重载、脏文档冲突确认/哈希校验覆盖、版本/EOL/撤销保持、真实 GiB 有界只读浏览；缺递归工作区监听、diff/merge、异步普通文件打开/multi-root、文件操作、大文件编辑/编码选择 |
| 搜索替换 | 部分 | 文件名过滤；缺全文、正则、替换、忽略规则、搜索结果导航 |
| 命令/快捷键 | 部分 | 核心快捷键和扩展命令；缺 when/context keys、自定义键位及完整菜单贡献 |
| 语言功能 | 部分 | VSIX/通用 LSP 补全、Problems、gopls 格式化/定义跳转/Output 悬停、原子附加导入编辑；缺语义着色、重构、多位置/浮层和 snippet UI |
| LSP | 部分 | 用户服务器配置、gopls 固定安装、UTF-16 能力协商、增量/完整同步、版本诊断/清除、save/close/取消/进程回收、独立崩溃重启/退避和未保存文档重放；新源码跨平台验证见 harness，文件监听、多工作区和完整语言能力仍缺 |
| VSIX 宿主 | 部分 | 本地安装、CommonJS 激活、活动文档/事务/事件、语言提供者、持久状态；大量 API 和贡献点尚缺，未知 API 抛错 |
| 配置/主题 | 部分 | 扩展读取/更新 JSON 配置；缺 JSONC、配置 UI、schema/defaults、主题/语法贡献和配置实际应用到全部编辑器行为 |
| 终端 | 部分 | Windows ConPTY/macOS PTY、真实 shell 输入高亮、VT/真彩色/Unicode/备用屏、网格 resize、输入/中断/退出、标签/选区/滚动与有界历史；终端扩展 API、shell integration、链接、完整 IME/字形样式待实现；各平台新源码验证状态见 harness |
| Git / SCM / Diff | 未实现 | 需要仓库状态、stage/commit、diff/merge、历史与扩展 SCM API |
| Debug / DAP | 未实现 | 需要会话/断点/变量/调用栈、launch.json、debug adapter 与扩展 API |
| Tasks / Testing | 未实现 | 需要任务和测试树/结果、进程/问题匹配器、tasks.json 与 testing API |
| Webviews / TreeViews | 未实现 | 需要原生视图协议、树的数据/事件，以及受控的 webview 渲染方案 |
| Remote / Notebooks | 未实现 | 需要远程文件系统/扩展宿主、容器/SSH、notebook 内核和视图 |
| Accessibility | 未实现 | Windows UI Automation、macOS Accessibility、读屏/高对比度/完整焦点语义 |
| Copilot 原生补全 | 真实验收 | 官方 LSP、UTF-16 版本同步、灰色建议、Tab 插入、实际接受后命令；尚缺完整 next-edit/partial acceptance 和配额/模型 UI |
| Copilot 原生聊天 | 真实验收 | 官方 Go SDK、流式文本、取消与取消后重试；当前禁用工作区工具，缺 Agent 权限/工具 UI、完整附件/模型/历史管理 |
| 官方 Copilot VSIX | 未通过 | native SDK/LSP 的成功不能替代 VSIX 激活/API 依赖验收 |
| Marketplace | 未实现 | 当前仅本地 VSIX；还需 registry 配置、更新/禁用/依赖、许可与账号授权适配 |

## 本阶段验收

- 框架：UTF-16/CRLF、原子事务、快照、保存点、随机编辑/撤销 oracle；LSP 碎片帧、乱序并发、双向请求和取消/回收。
- 真实 VSIX：编辑未保存文档、UTF-16 代理对范围、增量事件、提供者观察新版本、Memento 在新宿主进程中保留。
- gocode：实际原生窗口中的 VSIX 编辑→CRLF 保存→撤销/重做→版本化补全；Windows 鼠标拖选和 Unicode 替换。
- 保存／关闭：共享后台队列、旧快照保留新编辑、外部文件修改拒绝覆盖、取消与队列边界；原生关闭保存／丢弃／取消／冲突，以及真实 VSIX Document.save 的磁盘确认和事件去重。源码／发行版的具体完成状态见 agent docs/status.md。
- 外部文件：v0.9.0 五平台 CI、真实原子替换/删除/父目录重建/风暴、脏文档保留、确认重载与新 VSIX 编辑竞争、Windows 打开句柄/长中文 emoji 路径/保存重试/取消、原生按钮/像素和实际 gopls 新文本悬停；发行字节更新/回滚和本机安装已验收。
- 大文件：真实 GiB 文本/单行的首/中/尾和内存记录；实际 GiB 原生窗口的长行尾部跳转/像素，源文件未修改。
- gopls：真实原生窗口格式化/悬停/定义/未保存前缀补全/诊断清除；补全的导入和主编辑一次撤销。
- 官方 Copilot：固定 npm/Go SDK 版本的进程握手；真实账号下在临时工作区绘制建议、Tab 接受、聊天、取消和重试。账号验收与无账号 CI 分开记录。

## 后续实现顺序

1. 免费安装包/CLI/更新与本机安装，完善 LSP 重启/语言功能、全文搜索替换和编辑文件外部修改处理。
2. 深化已验证的 ConPTY/PTY 终端及其扩展 API，加入 Git/diff/merge、DAP、tasks/testing，以真实进程作为验收。
3. 扩展贡献点、JSONC/主题/snippet、webview/TreeView/SecretStorage，并逐一验收目标 VSIX 的依赖。
4. Copilot Agent 权限/工具及官方 VSIX、远程/多窗口、多光标/IME/accessibility，增加平台和视觉回归。

这是当前能力与缺口的记录，不承诺未通过验收的功能已经可用，也不作完成日期承诺。
