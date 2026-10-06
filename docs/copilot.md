# 官方 GitHub Copilot 接入

补全采用 [GitHub 官方 Language Server](https://github.com/github/copilot-language-server-release)，聊天采用 [官方 Go SDK](https://github.com/github/copilot-sdk/tree/main/go)。工作台始终由 godesktop 原生 GPU UI 绘制，侧车进程负责 AI 协议。直接运行官方 Copilot VSIX 是另一项兼容目标，目前尚未通过。

## 固定运行时

| 组件 | 项目固定版本 |
| --- | --- |
| @github/copilot-language-server | 1.551.2 |
| @github/copilot npm 包 | 1.0.92，平台二进制版本由其依赖和 lockfile 固定 |
| github.com/github/copilot-sdk/go | v1.0.16 |

`npm ci --prefix tools/copilot-runtime` 安装 lockfile 中的官方包和平台可选依赖；不要禁用 optional dependencies。Node.js 使用 24，LSP 以 --stdio 启动，SDK 使用官方 native CLI 二进制，以 --headless/--stdio/--no-auto-update 启动。没有全局 npm 更新或手写模型接口。

运行可指定 `-copilot-runtime <含 node_modules 的目录>`；高级场景可设置 `GOCODE_COPILOT_CLI` 为显式 CLI 可执行文件。按需查找可执行文件旁的 copilot-runtime 或当前目录 tools/copilot-runtime。

## 账号与操作

补全的 Sign in 使用官方 LSP 的设备流程，在原生面板显示验证码并打开 GitHub 登录页面。SDK 聊天需要官方 CLI/gh 可用的 GitHub 登录及 Copilot 访问权。SDK 的状态目录位于用户配置目录 gocode/copilot，可用同一 COPILOT_HOME 配置官方 CLI 的 login。语言服务器登录与 CLI/SDK 登录不应假定一定共享凭据；`-copilot-check` 显示 SDK 是否已认证。

Ctrl+Space 请求补全；输入后的请求带短暂 debounce，后台响应必须匹配路径、版本、光标和请求序号。Tab 应用 UTF-16 范围后才调用服务端接受命令。修改和关闭文档同步到语言服务器，过期响应被丢弃。当前 UI 显示建议首行，缺完整多行/next-edit/partial acceptance 交互。

Ctrl/Cmd+I 打开原生聊天，Enter 发送，Cancel/Esc 取消。同一会话保留对话上下文；每次 UI 请求有独立序号，取消后的旧流不会污染新回答。SDK 使用 ModeEmpty 并关闭发现配置/技能/文件 hooks/工作区工具，权限请求不自动批准。当前是文本聊天；Agent 工具、附件/模型/历史 UI 尚缺。

## 验收

`-copilot-check` 验证真实 LSP 初始化、官方 SDK 连接和认证状态，不发送模型 prompt。Windows/macOS CI 使用这个模式；未认证是正常可报告状态，协议错误会失败。

`-copilot-smoke` 在临时目录中发送合成聊天 prompt 和加法函数补全，要求返回预期结果。`-copilot-ui-smoke` 进一步启动真实原生窗口，要求绘制可接受的建议、Tab 修改真实缓冲区、官方 SDK 聊天、取消和重试均通过。这两个模式需要可用账号/额度并发送真实模型请求，不读取待开发工作区作为测试内容。

运行时版本由 lockfile/模块版本固定；测试不保证模型输出永远确定，也不保证全部账号、网络、代理、企业策略、显卡和输入法已验证。结果以实际命令和 Actions artifacts 为准。
