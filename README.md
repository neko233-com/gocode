# gocode

[![CI](https://github.com/neko233-com/gocode/actions/workflows/ci.yml/badge.svg)](https://github.com/neko233-com/gocode/actions/workflows/ci.yml)

用 **Go 1.27 + [godesktop](https://github.com/neko233-com/godesktop)** 实现的原生编辑器工作区。Windows x86-64 / amd64 和 macOS Intel / Apple Silicon，UI 使用 Win32/Direct2D/DirectWrite 或 AppKit/Metal/CoreText。独立公开 Git 仓库，同时作为 `godesktop/gocode` 的 submodule。

界面按 VS Code Dark Modern 的布局实现：自定义标题栏、活动栏、文件树、标签页、面包屑、代码行号/着色、缩略图、底部面板、状态栏和命令列表。目标是接近其前端外观；当前是可运行原型，未宣称像素完全一致或完整替代 VS Code。

## 运行

需要 Go 1.27、cgo 和原生编译工具链。Windows 使用 64 位 MinGW-w64 的 gcc/g++；macOS 使用 Xcode Command Line Tools。扩展宿主需要 Node.js 22+（CI 使用 24）；缺少 Node 时仍可启动 UI，会显示扩展宿主不可用的原因。

```powershell
git clone https://github.com/neko233-com/gocode.git
cd gocode
$env:CGO_ENABLED='1'
go run . -workspace .
```

```sh
# macOS
CGO_ENABLED=1 go run . -workspace .
```

Windows 构建：

```powershell
go build -trimpath -ldflags="-s -w -H=windowsgui" -o bin/gocode.exe .
```

本仓库通过 `go.mod` 固定依赖 `godesktop v0.2.0`，没有本地 `replace`；可以独立 clone/build。在父仓库同时开发时，用父目录的 `go work init . ./gocode` 连接本地源码。应用代码在子仓库提交/推送，随后在父仓库提交 submodule 的新指针。

## 当前交互

- 点击文件树打开 UTF-8 文件，切换/关闭标签，未保存文件关闭时提示先保存。
- 基础字符输入、中文/emoji Unicode 字符、方向键、行拆分/合并、退格/Delete、四空格 Tab；点击代码行设置插入位置。
- Ctrl/Cmd+S 保存；Ctrl/Cmd+P 命令列表；Ctrl/Cmd+J 显示/隐藏底部面板；滚轮浏览代码。
- Search 按文件名过滤；Extensions 显示已安装扩展及其命令；消息、扩展输出和状态栏使用原生 UI。
- 标题栏空白区域可拖动，Windows 有最小化/最大化/关闭按钮；macOS 保留原生窗口按钮。

文件预览限制为 1 MiB UTF-8 文本，文件树最多 250 项；跳过 `.git`、`.cache`、node_modules、vendor、bin 和符号链接。基础编辑不包含选区、撤销、剪贴板、完整 IME 组合协议、精确复杂字形点击定位或 LSP；保存使用 LF。终端、调试器、Git 操作、Problems 和 Ports 面板目前为界面占位。

## 本地 VSIX

```powershell
go run . -install-extension ./your-extension.vsix
# 安装后重启编辑器
go run . -workspace .
```

扩展默认保存在用户配置目录 `gocode/extensions`，可通过 `-extensions-dir` 指定。内置 Hello Native 使用真实的 `require('vscode')`，通过标准 VSIX 路径安装；从扩展面板或命令列表执行，可以显示消息、输出、状态栏并打开工作区 README。

兼容的是 [godesktop 明确实现的 API 子集](https://github.com/neko233-com/godesktop/blob/main/docs/extensions.md)：命令注册/执行、部分 window/workspace API、基础类型和进程内 Memento。未知 API 明确报错。没有承诺全部现有 VS Code 扩展兼容；Marketplace、语言服务、调试、webview、终端和远程扩展尚不支持。只安装可信 VSIX，扩展宿主使用当前用户权限。

## 验证

```powershell
powershell -File scripts/test-windows-amd64.ps1
```

脚本执行三轮 race/随机顺序测试、vet、普通与 GUI EXE 构建及原生 smoke。Windows 真实 HWND 测试检查 amd64 PE 和系统 DLL、原生颜色、标题栏拖动/边框命中、中文/emoji 输入、保存文件、从 UI 执行 VSIX 命令、窗口缩放和关闭。测试只操作通过 PID/标题确认的自身窗口，截图保存在 `.cache/workbench-windows.png`。

CI 在 windows-2022/windows-2025 amd64、macos-15 arm64、macos-15-intel amd64 和 Ubuntu 上验证；Ubuntu 仅检查可移植模型。原生 smoke 要求真实绘制提交和已安装 VSIX 的命令返回值，Windows 使用 `GOAMD64=v1` 和严格 cgo 检查。Go 核心/原生桥接和扩展安装/宿主测试在 `godesktop` 仓库独立运行。构建产物、截图和覆盖率上传到 Actions artifacts。

Windows 像素验证需要可用且未被其他应用遮挡的桌面。测试启用保留帧内容的诊断模式；正常运行仍按需重绘。覆盖范围不代表全部 Windows 设备、显卡、缩放比例和输入法均已验证。

MIT License。VS Code 界面和 API 作为参考，项目与 Microsoft 无隶属关系。
