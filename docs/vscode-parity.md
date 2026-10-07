# VS Code 功能覆盖与验收

目标基线：VS Code 1.141.0 的工作台和公开扩展 API。gocode 是 godesktop 的持续验收应用：新功能必须同时有模型/协议测试和适用平台的原生验收。表中的“部分”表示已实现子集，不能用于宣称支持 VS Code 所有功能。

当前公开/本机已验收版本为 v0.23.0，源码 `47623823dd88fbb45422d096b545631be7e6b77b`，公开核心 v0.16.0；[CI 37678679723](https://github.com/neko233-com/gocode/actions/runs/37678679723) 首次五平台通过。真实发布包、回滚和安装后验证见 [工程记录](../agent%20docs/status.md)。新 Auto Save/Revert 的原生产品验收范围为 Windows amd64。

| 功能 | 当前状态 | 已验证 / 尚缺 |
| --- | --- | --- |
| Windows amd64 | 已验证 | 两种 Server runner、GOAMD64=v1、race、严格 cgo、真实 HWND/GPU、PE 架构；不等于所有设备/显卡/输入法均通过 |
| macOS Intel / Apple Silicon | CI 验证 | 原生构建、Metal smoke、VSIX 编辑验收、官方 Copilot 协议握手；v0.12.0 已通过真实工作台普通/1.5/2 密度 Metal 像素、40 标签原生 wheel/drag/key/身份验收和截图；付费账号 UI 验收目前在 Windows 执行 |
| 原生 UI | 部分 | Dark Modern 工作台结构、标题栏、活动栏、标签/面包屑/行号/面板/状态栏，系统字体实测 Latin/CJK 标签、标签溢出/滚轮/拖动及最多九组原生分屏；v0.22.0 发布真实 File/Edit 弹出菜单/子菜单/键盘导航和控件/菜单/编辑器圆角、子元素 GPU 裁剪；v0.23.0 继承这些能力并验收真实 150% File/Settings/Revert/扩展详情 GPU 像素。缺固定/预览/折行标签、全部菜单贡献、拖放/停靠、布局持久化、多窗口、全面逐像素对照；原生 GPU 阴影尚未发布，见 agent docs/modern-ui-and-gallery.md |
| 文档编辑 | 部分 | UTF-16 坐标、版本、不可变快照、事务、LF/CRLF、选区、撤销重做、剪贴板、拖选/重复按键；缺多光标、snippet、完整 IME/字素簇/双向导航和水平滚动 |
| 文件/工作区 | 部分 | 单工作区、后台打开/Explorer 扫描、取消/焦点/别名保护、真实原生打开文件/文件夹与另存为、未保存关闭确认、监听/原子替换/重载和脏文档冲突、真实 GiB 有界只读浏览；v0.23.0 保存/另存为冻结路径保护通过实际 writer/Node VSIX/安装后验证。缺递归监听、diff/merge、multi-root、完整 Explorer 文件操作、大文件编辑/编码选择 |
| Auto Save | Windows 已验证 | v0.23.0 原生 File checked toggle、Settings 四种模式/延迟，真实 Unicode/CRLF 写盘与去重；发布/安装后的 console+GUI 实际最小化 IsIconic、save/didSave/clean 与 View/GPU 2→2 后再恢复均通过。untitled/大文件不自动另存为；缺保存参与者、format/code actions on save、设置作用域/热更新和按资源排除 |
| Revert File | Windows 已验证 | v0.23.0 原生 File 命令、dirty 确认/取消、真实异步读盘/恢复、128 watch cap 外第129个目标优先、版本/哈希/身份保护；取消/新编辑/删除/二进制/超限保留缓冲区。真实发布包及安装后 rounded Revert 验收通过；untitled/只读大文件禁用，完整 diff/merge 仍缺 |
| 搜索替换 | 部分 | v0.13.0 全文搜索、Unicode/Go 正则、Git/glob、未保存快照、虚拟结果及 UTF-16/超大文件跳转；v0.14.0 后台预览、完整缓冲区提交和保存/冲突保护；v0.15.0 整组撤销/重做、原生确认/当前文件拆分、旧回执/关闭重开/光标/组过期保护已通过五平台、Mac 三密度、真实 GiB、发行字节/回滚和本机安装验收；精确证据见 agent docs/search.md、replace.md、history.md、status.md；缺 PCRE2、全局忽略/编码/provider、逐项替换控制、跨关闭资源/复合撤销及完整 diff |
| 命令/快捷键 | 部分 | v0.22.0 发布原生内置 VS Code/JetBrains 两套键位、Settings/命令面板切换及持久化，菜单提示同步；v0.23.0 发布/安装后两套原生键位与幂等验证通过。标签支持 Ctrl MRU 顺序/反向/松开提交、PageUp/Down 与保留脏文档关闭；缺 MRU 浮层、when/context keys、自定义键位及全部菜单贡献 |
| 语言功能 | 部分 | VSIX/通用 LSP 补全、Problems、gopls 格式化/定义跳转/Output 悬停、原子附加导入编辑；缺语义着色、重构、多位置/浮层和 snippet UI |
| LSP | 部分 | 用户服务器配置、gopls 固定安装、UTF-16 能力协商、增量/完整同步、版本诊断/清除、save/close/取消/进程回收、独立崩溃重启/退避和未保存文档重放；新源码跨平台验证见 harness，文件监听、多工作区和完整语言能力仍缺 |
| VSIX 宿主 | 部分 | 本地安装、CommonJS 激活、文档/事务/事件、语言提供者、持久状态；v0.17.0 增加原生 visibleTextEditors/viewColumn/选区/可见范围事件、隐藏打开、preserveFocus、独立 reveal、关闭重开身份与延迟焦点保护，实际验证状态见 agent docs/status.md；完整 tabGroups、options/decorations/snippets/undo merging 和大量贡献点尚缺，未知 API 抛错 |
| 配置/主题 | 部分 | 扩展读取/更新 JSON，原生 Settings 已有键位、Auto Save 模式/延迟及更新路由配置，幂等保存与会话覆盖已验收；缺 JSONC、完整配置 UI/schema/defaults、作用域/热更新、主题/语法贡献及全部编辑器配置行为 |
| 终端 | 部分 | Windows ConPTY/macOS PTY、真实 shell 输入高亮、VT/真彩色/Unicode/备用屏、网格 resize、输入/中断/退出、标签/选区/滚动与有界历史；v0.20.0 发布真实 VSIX createTerminal/PID/环境/Unicode 输入、show/hide/焦点、生命周期事件与原生关闭/进程回收，通过五平台、发行和本机验收，见 agent docs/terminal.md；Pseudoterminal、完整扩展 API、shell integration、链接、IME/字形样式仍缺 |
| Git / SCM / Diff | 部分 | v0.21.0 真实仓库状态/初始化、单项及全部暂存/取消暂存、索引提交和原生只读左右差异；Unicode/CRLF/二进制/重命名/工作树/SHA256、脏文档和过期索引保护、子进程取消已通过五平台/发行/回滚/本机验收，见 agent docs/scm.md；缺编辑/逐块 diff、合并、历史/分支/远程、多仓库与扩展 SCM API |
| Debug / DAP | 未实现 | 需要会话/断点/变量/调用栈、launch.json、debug adapter 与扩展 API |
| Tasks / Testing | 未实现 | 需要任务和测试树/结果、进程/问题匹配器、tasks.json 与 testing API |
| Webviews / TreeViews | 未实现 | 需要原生视图协议、树的数据/事件，以及受控的 webview 渲染方案 |
| Remote / Notebooks | 未实现 | 需要远程文件系统/扩展宿主、容器/SSH、notebook 内核和视图 |
| Accessibility | 未实现 | Windows UI Automation、macOS Accessibility、读屏/高对比度/完整焦点语义 |
| Copilot 原生补全 | 真实验收 | 官方 LSP、UTF-16 版本同步、灰色建议、Tab 插入、实际接受后命令；尚缺完整 next-edit/partial acceptance 和配额/模型 UI |
| Copilot 原生聊天 | 真实验收 | 官方 Go SDK、流式文本、取消与取消后重试；当前禁用工作区工具，缺 Agent 权限/工具 UI、完整附件/模型/历史管理 |
| 官方 Copilot VSIX | 未通过 | native SDK/LSP 的成功不能替代 VSIX 激活/API 依赖验收 |
| 扩展商店 / Gallery | 部分 | 默认真实 Open VSX 搜索/Windows x64-universal 包选择/VSIX 安装，原生详情/贡献命令、启用/禁用/卸载及保存保护重启已发布并安装验收；原生 `-extension-gallery-url` 支持获授权服务的 VS Gallery 协议。2026-10-08 [官方 FAQ](https://code.visualstudio.com/docs/supporting/FAQ#extensions) 仍限制衍生产品访问微软 Marketplace，本项目没有单独授权，不能宣称官方商店已接入；自动更新/依赖/全部扩展 API 仍缺 |

## 本阶段验收

- v0.23.0：五平台精确源码 CI、独立公开核心 full Windows default-three strict-cgo/race、25项真实发布字节 console/GUI、签名完整 ZIP 三条实际请求（自动 direct、手动 direct、手动 ghfast.top）、实际 v0.4.0 GUI/VSIX 回滚及用户本机安装后的 Auto Save/Revert/GiB/菜单/键位/终端/Git/SDK-LSP 健康验证。当前安装健康检查 networkPromptSent=false，不能代替付费请求或官方 Copilot VSIX 验收。
- 扩展商店：安装版 `-extension-catalog-check golang` 只读查询真实 Open VSX，解析三个实际 win32-x64 版本，installed=false；`golang.go` 查询未解析到 Windows x64/universal 版本，保留为查询解析限制。版本、原始成功/失败报告及范围见 agent docs/modern-ui-and-gallery.md；这些查询不安装扩展，也不验证微软官方 Marketplace 或目标 VSIX 已兼容。
- 框架：UTF-16/CRLF、原子事务、快照、保存点、随机编辑/撤销 oracle；LSP 碎片帧、乱序并发、双向请求和取消/回收。
- 真实 VSIX：编辑未保存文档、UTF-16 代理对范围、增量事件、提供者观察新版本、Memento 在新宿主进程中保留。
- gocode：实际原生窗口中的 VSIX 编辑→CRLF 保存→撤销/重做→版本化补全；Windows 鼠标拖选和 Unicode 替换。
- 保存／关闭：共享后台队列、旧快照保留新编辑、外部文件修改拒绝覆盖、取消与队列边界；原生关闭保存／丢弃／取消／冲突，以及真实 VSIX Document.save 的磁盘确认和事件去重。源码／发行版的具体完成状态见 agent docs/status.md。
- 外部文件：v0.9.0 五平台 CI、真实原子替换/删除/父目录重建/风暴、脏文档保留、确认重载与新 VSIX 编辑竞争、Windows 打开句柄/长中文 emoji 路径/保存重试/取消、原生按钮/像素和实际 gopls 新文本悬停；发行字节更新/回滚和本机安装已验收。
- 大文件：真实 GiB 文本/单行的首/中/尾和内存记录；实际 GiB 原生窗口的长行尾部跳转/像素，源文件未修改。
- gopls：真实原生窗口格式化/悬停/定义/未保存前缀补全/诊断清除；补全的导入和主编辑一次撤销。
- 官方 Copilot：固定 npm/Go SDK 版本的进程握手；真实账号下在临时工作区绘制建议、Tab 接受、聊天、取消和重试。账号验收与无账号 CI 分开记录。

## 后续实现顺序

1. 深化已有免费安装包/CLI/更新验收，补齐 Auto Save 保存参与者、配置作用域/热更新、完整语言功能和递归工作区文件变化。
2. 深化已验证的 ConPTY/PTY 终端及其扩展 API，加入 Git/diff/merge、DAP、tasks/testing，以真实进程作为验收。
3. 扩展贡献点、JSONC/主题/snippet、webview/TreeView/SecretStorage，并逐一验收目标 VSIX 的依赖。
4. Copilot Agent 权限/工具及官方 VSIX、远程/多窗口、多光标/IME/accessibility，增加平台和视觉回归。

这是当前能力与缺口的记录，不承诺未通过验收的功能已经可用，也不作完成日期承诺。
