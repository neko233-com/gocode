# gocode

[![CI](https://github.com/neko233-com/gocode/actions/workflows/ci.yml/badge.svg)](https://github.com/neko233-com/gocode/actions/workflows/ci.yml)

Go 1.27 的原生编辑器，以 [godesktop](https://github.com/neko233-com/godesktop) 为核心框架，支持 Windows x86-64/amd64 和 macOS Intel/Apple Silicon。UI 使用 Direct3D 12/DirectWrite 或 Metal/CoreText，没有 WebView。独立公开仓库，同时作为 godesktop 的 Git submodule 和持续验收应用。

目标是覆盖 VS Code 工作台、编辑器和扩展能力。目前已贯通版本化编辑、VSIX 编辑/语言提供者和官方 Copilot 接入，完整覆盖情况见 [功能矩阵](docs/vscode-parity.md)。Dark Modern 布局是实现参考，尚未通过和 VS Code 的逐像素对照验收。

## 安装与更新

发布包位于 [Releases](https://github.com/neko233-com/gocode/releases)：Windows x64
提供免费 MSI 和便携 ZIP；macOS Intel/Apple Silicon 提供 .app 压缩包。发布后
`distribution/install.ps1`、`distribution/install-macos.sh` 包含该版本的固定 SHA256。
下载脚本后运行 `powershell -ExecutionPolicy Bypass -File install.ps1` 或
`sh install-macos.sh`。可用 `-Route mirror -Mirror https://前缀/` 或 macOS 环境变量
`GOCODE_UPDATE_MODE=mirror GOCODE_UPDATE_MIRROR=https://前缀/` 指定加速。

设置图标打开原生更新页：自动检查、GitHub 直连、自动路由和手动镜像。
更新验证免费 Ed25519 发布元数据及 SHA256，检查新程序版本/来源后，仅切换下次
启动的版本；当前编辑窗口继续运行。命令为 `gocode -update-check`、`gocode -update`
和 `gocode -update-rollback`。`-configure-updates -update-mode auto/direct/mirror
-update-mirror https://前缀/ -updates-auto true/false` 保存用户设置。

发布流程维护 `distribution/winget/`、`bucket/` 和 `Casks/`，供本地 winget manifest、
自定义 Scoop bucket 和 Homebrew tap 使用。公共包注册表的收录状态单独记录；
生成 manifest 不等于已上架。详见 [分发契约](agent%20docs/distribution.md)。

可选扩展/AI 宿主需要 Node.js 24/npm。安装后运行 `gocode -install-copilot` 和
`gocode -install-gopls`，工具装入用户目录，之后无需源码目录。MSI 提供用户 PATH、
开始菜单和桌面快捷方式。安装包和更新均不要求付费 OS 签名证书。

## 从源码运行

需要 Go 1.27、cgo、Node.js 24。Windows 使用 x86-64 MinGW-w64 gcc/g++，macOS 使用 Xcode Command Line Tools。

```powershell
git clone https://github.com/neko233-com/gocode.git
cd gocode
$env:CGO_ENABLED='1'
go run . -install-copilot
go run . -workspace .
```

macOS 使用 `CGO_ENABLED=1 go run . -workspace .`。Windows GUI 构建：`go build -trimpath -ldflags="-s -w -H=windowsgui" -o bin/gocode.exe .`。

`-window-width 1024 -window-height 728` 可指定启动窗口的 DIP 尺寸，默认 1280×820；原生标题栏仍可拖动、缩放和最大化。

go.mod 固定依赖公开发布的 godesktop v0.12.0，没有本地 replace，可以独立 clone/build。开发两个仓库时，可用父目录的 go.work；独立验收必须设置 GOWORK=off。更新子仓库后，在父仓库提交新的 gitlink。

## 编辑与扩展

- UTF-16 坐标、中文/emoji、Shift 方向键/点击选区、鼠标拖选、原生剪贴板、版本化事务和撤销重做。
- v0.19.0 使用公开核心保留 Windows 与 Mac 彩色 emoji 的原生 RGBA、透明度和裁剪；普通文字与彩色字形混合批次仍有界。Windows ConPTY 的实际彩色 emoji 输出和编辑器分栏 GPU 像素验收见 [彩色字形记录](agent%20docs/color-glyphs.md)。
- 原生标题栏复用 Code-OSS 图片纹理；标签按系统字体实测宽度，完整显示 README.md 和中文文件名。源代码参照和原生像素验收见 [UI 工程记录](agent%20docs/ui.md)。
- 标签过多时使用原生滚动视口、可见项布局和可拖动滚动条；滚轮按指针位置分流。Ctrl+Tab／Ctrl+Shift+Tab 按最近使用顺序切换，松开 Ctrl 结束一次切换；Ctrl/Cmd+PageUp／PageDown 按标签顺序切换，Ctrl/Cmd+W 保留未保存关闭确认。
- v0.17.0 将真实 VSIX 的 visibleTextEditors、viewColumn、选区和 reveal 连接原生分屏，支持最多九组和 Ctrl/Cmd+1..9。隐藏打开不会抢焦点，延迟结果受来源回执保护；各视图共享文档并独立滚动，关闭后旧引用失效。右侧/下方分屏、共享历史、有界超大文件索引和独占脏文档确认的精确范围与验证见 [分屏工程记录](agent%20docs/groups.md)。
- v0.10.0 将普通打开和有界 Explorer 扫描放到后台；慢读取期间可继续输入、缩放和取消，旧结果不会抢回新标签或覆盖未保存内容。VSIX/Problems/定义跳转等待实际目标，扩展命令等待文档同步确认；五平台和实际安装证据见 harness。
- 未保存的窗口／标签页关闭会显示保存、丢弃、取消；关闭保存使用后台不可变快照，
  保存前检查磁盘内容，检测外部变更时保留原文件和未保存缓冲区。
  当前发行版的版本／真实安装验收状态见 agent docs/status.md。
- Ctrl/Cmd+S 保存；Ctrl/Cmd+Z 撤销；Ctrl/Cmd+Shift+Z 或 Ctrl+Y 重做；Ctrl/Cmd+A/C/X/V；保存保留 LF/CRLF。
- 工作区替换支持整组撤销/重做；撤销时可选所有文件、当前文件或取消。后台准备期间的新编辑、光标变化和关闭重开会使旧结果失效；文件有后续编辑时拆为当前文件操作，撤销不会自动写回磁盘。源码验证和发行状态见 [工作区历史](agent%20docs/history.md)。
- Ctrl/Cmd+P 命令列表；Ctrl/Cmd+J 面板；Ctrl+Space 请求 VSIX 补全和 Copilot；Enter 选择补全，Tab 接受 Copilot 建议。
- Ctrl/Cmd+Shift+F 打开原生全文搜索；支持大小写、Unicode 整词、Go 正则、包含／排除 glob、Git 忽略规则和未保存文档。搜索在单一可取消后台工作器中流式执行，结果点击后核对版本、内容哈希和 UTF-16 位置；超大文件跳到真实字节视图。Enter 搜索，方向键／Enter 或 F4 导航。PCRE2 扩展语法仍需实现，具体边界见 [搜索工程记录](agent%20docs/search.md)。
- 已安装 VSIX 可以读取未保存的活动文档、执行原生编辑/保存、接收文档事件、注册补全/悬停/定义、发布 Problems 诊断、持久保存 workspaceState/globalState。
- Ctrl/Cmd+Shift+H 展开工作区替换。Preview 在后台核验并显示替换前后；Replace all 提交完整文档批次，再使用后台保存队列。正则支持捕获组与转义，未保存内容和撤销历史保留；源文档变化会拒绝旧预览，保存冲突保留脏文档和错误。单文件 8 MiB、整批源加输出 64 MiB／128 文件；大文件仍只读，磁盘持久化按文件完成。源码／发行验证及兼容缺口见 [替换契约](agent%20docs/replace.md)。

`go run . -install-extension ./extension.vsix` 安装可信本地 VSIX。内置 Hello Native 经标准 VSIX 安装，验证命令、版本化编辑、补全、诊断、输出和消息。未知 VS Code API 明确报错；[框架 API 范围](https://github.com/neko233-com/godesktop/blob/main/docs/extensions.md) 列出已实现的子集。

普通文件最多 8 MiB 使用版本化编辑；较大的 UTF-8 文件进入有界内存的只读浏览。Ctrl/Cmd+G 输入行号或 `:字节位置`，滚轮/PageUp/PageDown/拖动滚动条导航；超长单行通过字节视图浏览，Ctrl/Cmd+C 复制当前页面。真实 1 GiB 文本和单行文件均有验收。[大文件策略](docs/large-files.md) 记录内存、速度和边界。

文件树目前最多 250 项，普通编辑视图单行显示最多 400 个 rune。跳过 .git、.cache、node_modules、vendor、bin 和符号链接。完整 IME、字素簇/双向文本导航、多光标、Git 操作、DAP、webview、远程扩展等仍需实现。

## 原生终端

Windows 使用真实 ConPTY/PowerShell；macOS 使用 PTY 和用户 shell，默认 zsh。
Windows 发行版内嵌固定版本的微软 MIT 授权 ConPTY，避开旧系统吞掉备用屏
通知的问题。校验后解包到 gocode 自己的缓存；离线可用，不替换系统组件。
源码开发可运行 `go run ./cmd/gocode-terminaltools` 或 `gocode -install-terminal-tools`。
Ctrl/Cmd+` 聚焦或新建终端，+ 新建标签，× 回收进程；拖动面板上边缘改变高度，
窗口缩放会更新真实终端网格。支持 ANSI/真彩色、Unicode、备用屏、历史滚动、
鼠标选区、Ctrl+Shift+C（Mac Cmd+C）复制、Ctrl+V/Cmd+V 粘贴和 Ctrl+C 中断。

PowerShell 7 和 Windows PowerShell 5.1 使用 PSReadLine 输入高亮；zsh 使用内嵌
BSD 授权的 zsh-syntax-highlighting 0.8.0，在现有用户配置后加载。只写入 gocode
拥有的临时会话配置，不改用户 .zshrc/PowerShell profile。无需安装 Oh My Zsh。
已有 Oh My Zsh 配置会被读取。完整终端扩展 API、shell integration、链接点击、
IME/完整字形样式仍待实现。详见 [终端契约](agent%20docs/terminal.md)。

## 通用语言服务

`go run . -install-gopls` 在 gocode 的用户工具目录安装固定版本的官方 gopls；下次启动自动连接。也可以通过 `-lsp-config <JSON>` 配置其他标准语言服务器，或用 `-lsp=false` 关闭。支持原生补全、版本化诊断、Shift+Alt+F 格式化、F12 定义跳转、Ctrl/Cmd+K 悬停信息。[配置与策略](docs/language-servers.md) 说明协议边界和配置格式。

语言服务器意外退出后会独立退避重启，重新发送最新未保存文档；旧诊断和请求结果
会失效。三分钟内四次重试仍失败则停止自动重启，可以用原生命令列表的
Restart Language Servers 重试。当前源码／发行版验收状态见 agent docs/status.md。

标准 LSP、VSIX、Copilot 同步的源码快照上限为 2 MiB，以保证有界协议消息；超大文件不会被整体发送给这些服务。编辑和浏览模式不会将截断内容保存回原文件。

## GitHub Copilot

采用官方 Language Server 做补全，官方 Go SDK 做聊天；没有把当前接入当作官方 Copilot VSIX 已兼容。[使用与验收](docs/copilot.md) 说明固定版本、账号登录和测试方式。

Ctrl/Cmd+I 打开原生聊天面板，Enter 发送，Cancel/Esc 取消。语言服务器的设备登录入口位于聊天面板和命令列表。聊天需要官方 CLI 可用的 GitHub/Copilot 登录状态；当前模式禁用工作区工具，提供文本聊天和取消/重试。Agent 工具执行和完整 Copilot VSIX 是后续目标。

运行时通过项目内 `tools/copilot-runtime` 安装，或使用 `-copilot-runtime <目录>` 指定；`-copilot=false` 关闭连接。核心编辑器可在未安装 Copilot 运行时的情况下启动。

## 自动验收

```powershell
powershell -ExecutionPolicy Bypass -File scripts/test-windows-amd64.ps1
go run . -editor-smoke -extensions-dir .cache/acceptance-extensions
go run . -tabs-smoke
go run . -search-smoke -search-smoke-mib 1024
go run . -largefile-smoke -largefile-smoke-mib 1024
go run . -lsp-smoke
go run . -copilot-check
# 以下使用合成样例发送真实 Copilot 请求，需要有可用账号：
go run -race . -copilot-ui-smoke -copilot-runtime tools/copilot-runtime
```

Windows 测试使用 GOAMD64=v1、race、严格 cgo，检查 AMD64 PE、真实 HWND/GPU 像素、Unicode 输入、鼠标拖选和替换、保存、VSIX 命令、最大化/还原/最小化/关闭。截图仅来自拥有的 HWND 的 D3D12 帧。`-editor-smoke` 在临时工作区中验证 VSIX→Go 编辑、CRLF 保存、撤销重做和新版本补全。

CI 覆盖 windows-2022/windows-2025 amd64、macos-15 arm64、macos-15-intel amd64，以及 Ubuntu 的可移植测试。所有原生平台安装固定的官方 Copilot 运行时并验证协议握手，CI 不发送付费模型请求。真实账号下的补全、聊天、取消和重试另由 `-copilot-ui-smoke` 验收。

MIT License。VS Code 界面/API 作为参考，项目与 Microsoft 无隶属关系。官方 Copilot 运行时按其上游许可通过 npm 单独安装。

工程记录维护在 [agent docs/](agent%20docs/README.md)。VS Code 参考固定到 2026-10-07 main `a4a3dff`（stable 1.140.0）；窗口/程序/安装资源使用该仓库的 Code-OSS 图标，资源固定版本与 MIT 通知见 [assets/code-oss](assets/code-oss/PROVENANCE.md)。
