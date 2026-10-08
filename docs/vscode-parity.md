# VS Code 功能覆盖与验收

对照基线：VS Code 1.141.0 的工作台和公开扩展 API。当前目标是覆盖常用 IDE 工作流及真实 Go/TypeScript 语言扩展；完整 VS Code 功能/API 对等仍是后续工作。gocode 是 godesktop 的持续验收应用：模型/协议、原生窗口、发布包和用户安装分别记录，表中的“部分”不能用于宣称支持 VS Code 所有功能。

公开/用户安装应用仍为 v0.23.0，源码 `47623823dd88fbb45422d096b545631be7e6b77b`，该版本使用公开核心 v0.16.0；[CI 37678679723](https://github.com/neko233-com/gocode/actions/runs/37678679723) 首次五平台通过。真实发布包、回滚和安装后验证见 [工程记录](../agent%20docs/status.md)。Auto Save/Revert 的原生产品验收范围为 Windows amd64。

当前 VERSION 为 v0.24.0 的未发布 Windows amd64 候选，独立使用公开核心 v0.17.0，GOWORK=off、无 replace。核心不可变源码为 `996b5ff16189d95ea72298ee48bd03366b27757e`，其五平台 CI 和独立公开模块校验已通过；应用的 macOS 包/渠道继续保持 v0.23.0，不能继承为本次 Windows 候选的 Mac 验收。

本机源码 `f4a795d4a4cc09ce67f72a93364f15363c077d48` 的五项源码检查、真实 MSI/ZIP 构建及原始 ZIP 字节的35项有序验收已通过。该包验收使用隔离测试签名；修正的 MSI 专用启动工具另外通过实际原包完整安装生命周期及中文/空格目录，原包和工具哈希分别绑定。生产签名、最终修正源码的包验收、真实旧版签名回滚和用户安装晋级仍待完成，f4a 记录保持历史绑定。根据2026-10-08用户指令，push/PR自动 Actions 和远程发布构建已停用，后续版本在本机集中构建、验证和上传，使用不可变标签，不移动已发布标签。详细边界见 [本机发布流程](../agent%20docs/local-publication-runner.md) 与 [工程记录](../agent%20docs/status.md)。

| 功能 | 当前状态 | 已验证 / 尚缺 |
| --- | --- | --- |
| Windows amd64 | 本机候选已验证 | v0.24.0/public17 的 GOAMD64=v1、三轮 race/严格 cgo、真实 HWND/GPU/PE、hardware/WARP 弹层和真实包字节验收通过；两种 Windows Server runner 是 v0.23.0 历史 CI 证据。本次候选不等于所有设备/显卡/输入法均通过 |
| macOS Intel / Apple Silicon | 已发布 v0.23.0 历史验收 | 原生构建、Metal smoke、VSIX 编辑验收、官方 Copilot 协议握手；历史工作台普通/1.5/2 密度像素、40标签 wheel/drag/key/身份及截图已通过。v0.24.0 仅面向 Windows，本次 Go/TS adapter、弹层和安装流程未在 Mac 验收或重新打包 |
| 原生 UI | 部分 | Dark Modern 结构、系统字体 Latin/CJK、标题栏/活动栏/标签/面包屑/行号/面板/状态栏、标签溢出/滚轮/拖动及最多九组分屏；已发布圆角与子元素 GPU 裁剪。公开核心17已有原生 GPU 阴影，Windows24候选将其接入 File/子菜单、Quick Input 和确认框，真实 hardware/WARP 像素、正文/祖先 hit test 和 idle/关闭验收通过。缺固定/预览/折行标签、全部菜单贡献、拖放/停靠、布局持久化、多窗口及全面逐像素对照；应用24尚未发布，见 agent docs/modern-ui-and-gallery.md |
| 扩展详情编辑器 | Windows 候选子集已验证 | v0.24.0 固定标题/元数据与操作头部、36-DIP 导航、独立原生裁剪/焦点/滚动；真实字体 Unicode wrap 和窄窗 reflow，64 KiB 描述/512描述行/4096命令/最多1024物化行（含 overscan）有界虚拟化；tab→End 同 UI turn 准备 extent，真实私有 VSIX 注册2000回调并执行最后 command1999，两套键位 page/close/reset、启停持久化、四文件复用和私有 scratch 清空均通过。f4a整套及console/GUI包字节详情验收已通过。缺真实 VSIX 图标、README/CHANGELOG/Markdown、多种贡献编辑器及 scrollbar drag；最终生产签名/安装待验收，见 agent docs/extension-details.md |
| 文档编辑 | 部分 | UTF-16 坐标、版本、不可变快照、事务、LF/CRLF、选区、撤销重做、剪贴板、拖选/重复按键；缺多光标、snippet、完整 IME/字素簇/双向导航和水平滚动 |
| 文件/工作区 | 部分 | 单工作区、后台打开/Explorer 扫描、取消/焦点/别名保护、真实原生打开文件/文件夹与另存为、未保存关闭确认、监听/原子替换/重载和脏文档冲突、真实 GiB 有界只读浏览；v0.23.0 保存/另存为冻结路径保护通过实际 writer/Node VSIX/安装后验证。缺递归监听、diff/merge、multi-root、完整 Explorer 文件操作、大文件编辑/编码选择 |
| Auto Save | Windows 已验证 | v0.23.0 原生 File checked toggle、Settings 四种模式/延迟，真实 Unicode/CRLF 写盘与去重；发布/安装后的 console+GUI 实际最小化 IsIconic、save/didSave/clean 与 View/GPU 2→2 后再恢复均通过。untitled/大文件不自动另存为；缺保存参与者、format/code actions on save、设置作用域/热更新和按资源排除 |
| Revert File | Windows 已验证 | v0.23.0 原生 File 命令、dirty 确认/取消、真实异步读盘/恢复、128 watch cap 外第129个目标优先、版本/哈希/身份保护；取消/新编辑/删除/二进制/超限保留缓冲区。真实发布包及安装后 rounded Revert 验收通过；untitled/只读大文件禁用，完整 diff/merge 仍缺 |
| 搜索替换 | 部分 | v0.13.0 全文搜索、Unicode/Go 正则、Git/glob、未保存快照、虚拟结果及 UTF-16/超大文件跳转；v0.14.0 后台预览、完整缓冲区提交和保存/冲突保护；v0.15.0 整组撤销/重做、原生确认/当前文件拆分、旧回执/关闭重开/光标/组过期保护已通过五平台、Mac 三密度、真实 GiB、发行字节/回滚和本机安装验收；精确证据见 agent docs/search.md、replace.md、history.md、status.md；缺 PCRE2、全局忽略/编码/provider、逐项替换控制、跨关闭资源/复合撤销及完整 diff |
| 命令/快捷键 | 部分 | v0.22.0 发布原生内置 VS Code/JetBrains 两套键位、Settings/命令面板切换及持久化，菜单提示同步；v0.23.0 发布/安装后两套原生键位与幂等验证通过。v0.24.0 候选 Quick Input 输入/结果行/弹层使用原生圆角裁剪，Save All 在保存或文件操作忙碌时禁用，详情页继承 VS Code Ctrl+W/JetBrains Ctrl+F4 关闭。标签支持 Ctrl MRU 顺序/反向/松开提交、PageUp/Down 与保留脏文档关闭；缺 MRU 浮层、when/context keys、自定义键位及全部菜单贡献 |
| 语言功能 | 部分 | Windows24候选实际安装原始 golang.Go0.50.0+gopls v0.23.0，以及 vscode.typescript-language-features1.95.3/tsserver5.6.3+typescript-language-server6.0.1，通过原生 adapter 提供补全、Problems、格式化、悬停和定义；真实原生编辑/诊断清除/重启/未保存重放与包字节检查通过。支持 TypeScript/JavaScript/React 协议语言 ID，保留补全附加导入原子编辑。缺语义着色、重构、多位置/浮层、snippet UI 及官方语言扩展全部 JS 功能 |
| LSP | 部分 | UTF-16协商、增量/完整同步、诊断/清除、save/close/取消、崩溃重启/退避及未保存文档重放；Windows adapter 服务使用 suspended-before-resume Job/句柄白名单和有界关闭回执，实际 TS 根/tsserver/worker 全部回收。显式用户配置保持优先和会话身份；Go安装后只移除自动 fallback，禁用/卸载不会自动复活，TS-only不移除Go fallback；重装保留Disabled并清除自身待卸载。最多16服务器、2 MiB语言文档及队列有界；Node最低22.22.2、实际24.14.0。新adapter仅有Windows证据，文件监听、多工作区和完整语言能力仍缺，见 agent docs/language-extensions.md |
| VSIX 宿主 | 部分 | 本地安装、CommonJS 激活、文档/事务/事件、语言提供者、持久状态及原生编辑器身份/可见范围/延迟焦点保护已实现子集。识别的Go/TS包走原生LSP adapter，其原始 JS activate() 和manifest命令不冒充可执行回调。完整 tabGroups、options/decorations/snippets/undo merging、大量贡献点、任意 vscode-languageclient 和完整 JS API 兼容仍缺，未知 API 抛错；实际版本/能力见 agent docs/status.md |
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
| 扩展商店 / Gallery | 部分 | 默认真实 Open VSX 搜索/Windows x64-universal 包选择/VSIX 安装及基本管理；候选支持 publisher.extension/@id 精确查询、named latest→immutable version身份校验、stale取消和原installed/enabled/disabled过滤。真实解析的golang.Go0.56.1原始manifest271556B超过公开核心256 KiB上限，选择latest仍明确拒绝；显式“安装Go/TypeScript支持”命令展示并安装上述固定原包，不静默降级catalog选择。原生 `-extension-gallery-url` 支持另行获授权的 VS Gallery 协议及 exact ExtensionName=7，真实TLS/私有VSIX/Node已验收。[官方 FAQ](https://code.visualstudio.com/docs/supporting/FAQ#extensions) 的微软Marketplace衍生产品限制仍适用，本项目没有单独授权；真实扩展图标/README、自动更新/通用依赖及全部扩展API仍缺 |
| 分发 / 更新 / 安装 | 候选分层验收 | 公开/用户安装仍为23；Windows24的f4a五项本机源码检查、原始MSI/ZIP构建、35项包字节验收已通过，均绑定实际source/hash/进程关闭。生产签名、完整MSI安装/升级/损坏回滚/降级/卸载、真实旧版回滚和用户晋级仍待最终工具源码验收；Mac沿用23。后续本机发布并使用不可变tag，自动push/PR Actions已停用 |

## 本阶段验收

- 本机f4a源码：public17/GOWORK=off的五项有序检查均通过，包括Windows strict-cgo2/race/default-three1036.470s（main660.025s/34.5%）、独立no-cgo三轮295.196s、vet、actionlint/ShellCheck/PowerShell解析及五项分发测试。实际MSI16584704B、ZIP19739522B已构建；`.cache/local-release/0.24.0-f4a795d4a4cc-r1/prepared-candidate.json` SHA256 `32a0371364fca41581cc767b830ef6ff5421d758329a7a82fe6a6081b518eef0` 仍为prepared=true/complete=false。各进程root/tree已回收、私有TMP/config/stage已清空；后续修改不能复用为新源码验收。
- f4a原始ZIP字节：`.cache/native-combined24/unsigned-native-f4a-r2/execution/current.json` 的35项有序console/GUI检查全部实际通过，总111793ms，报告SHA256 `b7d38bfe3b63fff7af56f35afe23bd617fd56660ece75a621fbf83ef5963576b`。包含详情/圆角阴影、Auto Save及真实最小化、Go/TS原生和已安装服务、真实GiB浏览/搜索/分屏；每项启动路径与原始ZIP entry实际字节绑定，root/tree关闭，原VSIX/tool profile不变，私有目录已删除。这是测试隔离签名下的包字节验收；productionPublisherSignatureVerified、signedWithProductionKey、completeRelease、published均为false。
- 原生功能：public17 hardware/WARP popup三轮通过108.134s，18个Window Runs/144次实际GPU capture，保留RGB容差、三completed-frame floor、零inflight、hit test、250ms idle及关闭断言。真实Go/TS原生编辑各13阶段和GPU验证在r3通过5114/5576ms，真实旧Job关闭/未保存重放和TypeScript状态栏已检查；f4a整套及35项包检查给出后续独立证据。原包/工具hash、CLI幂等重装和显式用户优先/自动fallback来源控制见 [语言扩展记录](../agent%20docs/language-extensions.md)。
- 精确查询：Open VSX named/immutable metadata将golang.go及@id:GoLang.Go解析为golang.Go0.56.1/universal；这条metadata查询没有安装latest包。真实TLS、私有VSIX/Node、身份/version/platform/404/cancel/limits测试及原始回执见 [详情记录](../agent%20docs/extension-details.md)。随后真实安装和激活的是明确展示版本的Go0.50.0及TS1.95.3原始VSIX，不能将原生adapter成功当成最新Go JS扩展或微软官方Marketplace兼容。
- 失败历史保留：public16时期首次整套765.457s的旧坐标/过早callback-tree检查失败、后续72.035s定向和833.454s整套成功均保留原source边界；TS r1在90s仍有真实6133/Hint，精确协议探针后仅修fixture export值；race-instrumented GiB原30s worker失败也未改写。详见 [工程记录](../agent%20docs/status.md) 和相应功能记录。
- f4a MSI首步install-prior在120037ms超时，没有MSI日志；exit2是owned Job终止码。不存在MSI的同ownership对照显示全token引号5026ms超时/无日志，raw syntax83ms返回1619并写5806B日志。窄MSI启动修复另外通过原包实际安装/损坏升级回退/升级/拒绝降级/native/uninstall（10.984s，真实中文/空格目录）；原包/用户PATH/工作区/注册保持完整。真实解析控制102项通过，Type19交易前中止回显原样引号/等号/Unicode，注册状态-1保持，普通进程控制95项×3通过。最终修正源码绑定的签名/旧版回滚/用户晋级仍待完成；证据见本机发布记录，不重标原f4a失败。
- v0.23.0：五平台精确源码 CI、独立公开核心 full Windows default-three strict-cgo/race、25项真实发布字节 console/GUI、签名完整 ZIP 三条实际请求（自动 direct、手动 direct、手动 ghfast.top）、实际 v0.4.0 GUI/VSIX 回滚及用户本机安装后的 Auto Save/Revert/GiB/菜单/键位/终端/Git/SDK-LSP 健康验证。当前安装健康检查 networkPromptSent=false，不能代替付费请求或官方 Copilot VSIX 验收。
- 扩展商店历史：安装版v0.23.0 `-extension-catalog-check golang` 只读查询真实 Open VSX，解析三个实际 win32-x64 版本，installed=false；其free-text `golang.go` 合法空结果未解析到 Windows x64/universal，原始报告保留为当时的查询限制。v0.24.0 候选 named exact lookup 的新成功不改写旧失败。版本、报告及范围见 agent docs/modern-ui-and-gallery.md；两类 metadata 查询均不安装扩展，也不验证微软官方 Marketplace 或目标 VSIX 已兼容。
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
