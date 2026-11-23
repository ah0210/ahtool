# ah系统优化工具箱（ahtool）· Windows 清理与系统维护

一个基于 Go + Wails v2 的单文件 Windows 桌面工具：垃圾清理、应用卸载与残留清理、
系统优化、启动项管理、磁盘空间分析、驱动导出、蓝屏转储解析、硬件信息读取，以及
36 个系统原生工具。

- 版本：**v1.0.1**（二次开发基线，基于原项目 v1.1.0，GPLv3）
- 平台：Windows 10 / 11 · amd64
- 形态：单文件 exe，免安装，启动即请求管理员权限
- 前端：原生 HTML/CSS/JS，`go:embed` 嵌入，无 Node 构建步骤

> 本分支已解除原作者服务端绑定：**默认不检查在线更新、不连接原作者服务器**，
> 不含官网 / 社群 / 赞赏 / 远程服务入口。详见 [`NOTICE`](NOTICE) 与
> [`docs/audit/01-安全性与真实性评估报告.md`](docs/audit/01-安全性与真实性评估报告.md)。

---

## 功能

| 模块 | 说明 |
|---|---|
| 总览 | 开机体检：垃圾、启动项、磁盘、内存、待重启汇总为健康分 |
| 本机配置 | CPUID / SMBIOS / WMI / DXGI 直读；网络与音频页含 IP/MAC/网关/DNS/DHCP/代理/hosts/防火墙/监听端口等深度信息 |
| 垃圾清理 | 9 组 93 项规则；路径级去重；痕迹类默认不勾 |
| 系统优化 | 154 项注册表 / 服务开关，每项有独立还原动作 |
| 应用卸载 | 桌面软件 + UWP；卸后扫注册表与目录残留；卸载器退出但列表项未清时自动转残留清理，不再干等 |
| 磁盘空间 | 直读 NTFS MFT；按子树下钻；全量哈希查重；保护文件禁删；磁盘健康摘要与静音/均衡/极速 I/O 模式 |
| 启动项管理 | Run / StartupApproved / 服务 / 计划任务 / 启动文件夹 |
| 驱动管理 | 按类别导出驱动包；添加驱动；删除驱动需另行确认 |
| 蓝屏分析 | 本地解析 Minidump / MEMORY.DMP；新增异常关机/蓝屏/WHEA 时间线与转储配置检查 |
| 工具库 | 36 个工具，5 个分类；实时输出、可中断、部分带备份；含 DNS 解析监控 / 清缓存 |
| 网络洞察 | 网卡/IP/网关/DNS/DHCP、路由/代理、WiFi、连接与进程聚合、端口暴露面、ARP 邻居、共享/发现、分段连通性、SVG 拓扑、快照对比 |
| 帮助与诊断 | 本地自检清单 + 纯本机诊断报告（不自动上传） |

## 联网行为（如实说明）

默认构建**不会**在启动时联网，也没有遥测或账号体系。唯一可能发起网络请求的是
用户主动点击的「宽带测速」工具：它会访问内置的公共镜像 / Cloudflare / Ookla
测速端点，用于测下载、上传、延迟与丢包。原版通过明文 HTTP 查询公网 IP 归属地的
逻辑已停用。

在线更新代码仍保留在 `update.go`，但 `updateEnabled=false` 且清单地址为空；
二次开发时可由维护者改为自己的 HTTPS 发布站。

## 构建

需要 Go 1.26+（依赖见 `go.mod`）。Windows 10/11 上从环境准备到测试的完整步骤见
[`BUILD_TEST_WIN11.md`](BUILD_TEST_WIN11.md)。

快速构建：

```bat
build.cmd            :: bin\ahtool_v1.0.1-dev.exe
build.cmd release    :: bin\ahtool_v1.0.1.exe
```

或 PowerShell 7：

```powershell
pwsh -File build.ps1 -Release
```

跨平台（Linux/macOS 交叉编译）：

```bash
GOPROXY=https://goproxy.cn,direct ./build.sh release
```

版本号只在构建脚本的 `APPVER` 中维护，通过
`-X main.appVersion` / `-X main.buildDate` 注入，前端不再硬编码。

> `build.cmd` / `build.ps1` 只负责编译，不发布、不上传、不改写任何清单。

## 测试

```bash
go test ./...          # 纯逻辑 / 临时目录测试，不扫真实磁盘
go test -tags manual ./...   # 真机诊断测试：管理员权限、真实磁盘、注册表读写
```

需要真机或管理员权限的测试已统一标记 `//go:build manual`。

## 二次开发

- 目录结构与扩展指南：[`docs/audit/05-清理清单与二次开发基线.md`](docs/audit/05-清理清单与二次开发基线.md)
- 新增工具：在 `tools_*.go` 的 `init()` 里 `regTool(...)`，同步
  `toolkit_test.go` 的数量断言与 `testdata/golden_tools.json`。
- 新增优化项：改 `optimize_data.go` / `optimize_extra.go`；注意生成器不在仓库内，
  数据需手工维护并补还原动作。
- 审计报告：[`docs/audit/`](docs/audit/)

## 许可

本项目以 [GPLv3](LICENSE) 发布：

- 可自由使用、修改、再发布，也可用于商业用途；
- 分发修改版须同样以 GPLv3 开源，并保留原作者版权声明（见 [`NOTICE`](NOTICE)）；
- 不提供任何担保，使用风险自负。

原作者名称、Logo、赞赏码不在 GPLv3 授权范围内；本分支已替换为中性标识。
第三方组件许可见 [`THIRD-PARTY-NOTICES.md`](THIRD-PARTY-NOTICES.md)。
