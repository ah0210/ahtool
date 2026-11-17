# Win11 本地构建与测试指南（ahtool v1.0.0）

本文对应源码/产物包 `ahtool_v1.0.0_full_src_exe_20261112.zip`。版本号仍是 **v1.0.0**，没有改动。

---

## 0. 包内有什么

```
ahtool/
├─ bin/ahtool_v1.0.0.exe      # 已构建好的 x64 GUI 产物
├─ 全部 Go 源码 / go.mod / go.sum
├─ design/                    # 前端真源
├─ frontend/dist/             # Wails 资产（index.html 必须保留）
├─ app.manifest / app.ico / rsrc.syso
├─ build.cmd / build.ps1 / build.sh
├─ BUILD_TEST_WIN11.md        # 本文件
├─ SHA256SUMS.txt             # 关键文件哈希
├─ docs/audit/01–09           # 审计、构建、交付说明
└─ README / CHANGELOG / LICENSE / NOTICE / THIRD-PARTY-NOTICES
```

---

## 1. 环境要求

| 项 | 要求 |
|---|---|
| 系统 | Windows 10/11 x64（本包 exe 为 amd64） |
| Go | **1.26+**（`golang.org/x/sys v0.48.0` 硬性要求） |
| WebView2 | Win10/11 通常自带；LTSC/精简系统如缺失需安装 Microsoft Edge WebView2 Runtime |
| 权限 | 构建普通用户即可；运行 exe 需要管理员（manifest `requireAdministrator`） |
| 网络 | 首次 `go mod download` 需要能访问 Go 模块代理 |
| 可选 | Node.js（只用于 `node --check` 检查前端语法，不参与构建） |
| 不需要 | Wails CLI、MinGW/gcc、Visual Studio C++ 工具链（`CGO_ENABLED=0`） |

> 本包不包含 Go 工具链，也不包含 `_toolchain/`。不要再用原项目的离线 `GOPROXY=off` 构建方式。

---

## 2. 解压与校验

```powershell
Expand-Archive .\ahtool_v1.0.0_full_src_exe_20261112.zip -DestinationPath .
cd .\ahtool
Get-FileHash .\bin\ahtool_v1.0.0.exe -Algorithm SHA256
Get-Content .\SHA256SUMS.txt
```

`bin\ahtool_v1.0.0.exe` 的 SHA256 应等于 `SHA256SUMS.txt` 中对应行。注意：**从源码重新构建出的 exe 哈希通常不会与预构建 exe 完全相同**（Go 版本、构建路径、VCS 信息等都会影响）；请以版本资源、运行行为和功能测试为准。构建脚本已加 `-buildvcs=false`、`-trimpath`，可减少差异。

如果文件是从浏览器/网盘下载的，右键 exe → 属性 → 勾选「解除锁定」，避免 SmartScreen/附件管理器拦截。

---

## 3. 安装/确认 Go 1.26

```powershell
go version
# 期望：go version go1.26.x windows/amd64
```

如果低于 1.26，先安装 Go 1.26+（https://go.dev/dl/ 或国内镜像），再重开终端。

如果 `go version` 触发了自动工具链下载且卡住：

```powershell
$env:GOTOOLCHAIN = "local"
go version
```

### 设置模块代理（国内网络建议）

```powershell
go env -w GOPROXY=https://goproxy.cn,direct
go env -w GOSUMDB=sum.golang.org
```

然后：

```powershell
go mod download
```

若下载失败，先确认 `go env GOPROXY`，再试：

```powershell
$env:GOPROXY = "https://proxy.golang.com.cn,direct"
go mod download
```

---

## 4. 构建

### 方式 A：build.cmd（最简单）

```bat
build.cmd
:: 产物：bin\ahtool_v1.0.0-dev.exe

build.cmd release
:: 产物：bin\ahtool_v1.0.0.exe
```

### 方式 B：build.ps1

```powershell
pwsh -File .\build.ps1
pwsh -File .\build.ps1 -Release
```

### 方式 C：手动 go build（推荐用于排查）

在项目根目录（有 `go.mod` 的目录）执行：

```powershell
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"

go build -buildvcs=false -trimpath -tags desktop,production `
  -ldflags "-s -w -H windowsgui -X main.appVersion=1.0.0 -X main.buildDate=2026-11-12" `
  -o .\bin\ahtool_v1.0.0.exe .
```

关键点：

- `-tags desktop,production`：Wails v2 生产构建必须带。
- `-H windowsgui`：GUI 子系统，不弹控制台。
- `-X main.appVersion=1.0.0`：唯一版本真源；不要在前端/代码里另写版本号。
- `-buildvcs=false`：避免把本机 git 信息写进二进制，便于对比构建。
- `CGO_ENABLED=0`：Wails v2 的 Windows 后端可纯 Go 构建；不需要 gcc。
- `rsrc.syso` 已包含在包里（图标/版本资源/manifest）。不要删除；如果删了，Wails 仍能启动，但 exe 会没有版本资源/图标/manifest。

### 方式 D：只重新生成版本资源（可选）

只有改了图标或版本资源时才需要。安装 `go-winres` 后：

```powershell
go install github.com/tc-hib/go-winres@latest
go-winres simply --arch amd64 --out rsrc --no-suffix --manifest gui --admin `
  --icon app.ico --file-version 1.0.0.0 --product-version 1.0.0.0 `
  --file-description "ah系统优化工具箱" --product-name "ah系统优化工具箱 (ahtool)" `
  --original-filename "ahtool.exe" --copyright "GPLv3"
```

若生成的是 `rsrc_windows_amd64.syso`，重命名为 `rsrc.syso`。

---

## 5. 测试

### 5.1 编译期检查（安全、随时可跑）

```powershell
go vet ./...
go test -c -o .\bin\ahtool.test.exe .
```

`go test -c` 只编译测试，不运行；适合确认所有测试代码可编译。

运行默认安全测试集：

```powershell
go test ./...
```

默认集不扫真实磁盘、不写服务/注册表；`service_test.go` 的剪贴板测试在无交互桌面时会自动跳过。

### 5.2 前端语法（可选，需要 Node）

```powershell
Get-ChildItem .\design\*.js | ForEach-Object { node --check $_.FullName }
```

### 5.3 真机/管理员测试（**危险，只在 VM 中跑**）

这些测试标记了 `//go:build manual`，可能扫描真实磁盘、读写 HKCU/HKLM、启停服务、排重启删除队列：

```powershell
# 仅列出会跑哪些，不执行
go test -tags manual -list .

# 按需单个跑，例如：
go test -tags manual -run TestMftScanRealX -v -timeout 10m .
go test -tags manual -run TestStartupListRealX -v .
```

**不要**在主力机上直接 `go test -tags manual ./...`；先看测试文件头注释里的环境变量开关（如 `REALCLEAN=1`、`RP_LIVE=1`、`SCANPROBE=1`）。

### 5.4 关键回归测试（建议每次改动后跑）

```powershell
go test -run "TestEmbeddedAssetsHaveIndexHTML|TestMftNodeByPath|TestBuildUninstallCmdRejectsRawShell" -v .
go test -run "TestTool|TestServiceGuides|TestOptimizeCatalog" -v .
```

`TestEmbeddedAssetsHaveIndexHTML` 专门防止再次删掉 `frontend/dist/index.html`（这是上一版“双击无反应”的根因）。

---

## 6. 运行与自检

### 6.1 环境诊断（推荐第一步）

```powershell
.\bin\ahtool_v1.0.0.exe -diag
```

会弹出环境报告（版本、路径、管理员权限、WebView2 版本、日志路径），同时写入：

```
%TEMP%\ahtool_diag.txt
```

### 6.2 只读自检（不启 GUI）

```powershell
.\bin\ahtool_v1.0.0.exe -cli -nofull
```

GUI 子系统没有控制台，输出同时写入：

```
%TEMP%\ahtool_cli.txt
```

`-cli`（不带 `-nofull`）会包含 deepjunk 全盘遍历，耗时较长；建议先用 `-nofull`。

### 6.3 性能采样

```powershell
.\bin\ahtool_v1.0.0.exe -perf
```

输出同样写入 `%TEMP%\ahtool_cli.txt`。

### 6.4 正常启动

```powershell
.\bin\ahtool_v1.0.0.exe
```

会弹 UAC。窗口标题应为「ah系统优化工具箱 v1.0.0」；设置 → 关于应显示 v1.0.0 / 构建日期 / GPL-3.0 / 当前 exe SHA256。

### 6.5 启动失败时看哪里

```
%LOCALAPPDATA%\ahtool\startup.log
```

`wails.Run` 的错误、启动阶段、WebView2 版本都会写在这里；致命错误还会弹 MessageBox。

---

## 7. 构建产物验证

```powershell
(Get-Item .\bin\ahtool_v1.0.0.exe).VersionInfo | Format-List FileVersion,ProductVersion,ProductName,FileDescription,OriginalFilename
Get-FileHash .\bin\ahtool_v1.0.0.exe -Algorithm SHA256
```

期望：

- `FileVersion` / `ProductVersion` = `1.0.0.0`
- `ProductName` = `ah系统优化工具箱 (ahtool)`
- `FileDescription` = `ah系统优化工具箱`
- `OriginalFilename` = `ahtool.exe`
- 文件类型为 x64 GUI（任务管理器里是 64 位进程）

---

## 8. 常见问题

### 8.1 双击无反应

1. 先跑 `ahtool_v1.0.0.exe -diag`，看弹窗与 `%TEMP%\ahtool_diag.txt`。
2. 看 `%LOCALAPPDATA%\ahtool\startup.log`。
3. 如果日志里出现 `no index.html could be found`：说明包不完整，确认 `frontend/dist/index.html` 存在；本包已包含。
4. 检查 Windows 安全中心 → 保护历史记录；检查 Smart App Control 是否拦截未签名 exe。
5. 右键 exe → 属性 → 解除锁定；再右键「以管理员身份运行」。
6. 检查 WebView2 Runtime 是否安装（`-diag` 会显示版本）。
7. 事件查看器 → Windows 日志 → 应用程序，看是否有 `Application Error` / `ahtool` 记录。

### 8.2 构建时报 Go 版本错误

```
module ... requires go >= 1.26.0
```

安装 Go 1.26+，或确认 `go version` 不是被旧工具链/`GOTOOLCHAIN` 覆盖。

### 8.3 `go mod download` 超时

```powershell
go env -w GOPROXY=https://goproxy.cn,direct
go env -w GOSUMDB=sum.golang.org
go mod download
```

仍失败时检查代理/防火墙；不要用 `GOPROXY=off`。

### 8.4 构建成功但 exe 启动闪退

- 先 `-diag`；
- 看 `startup.log`；
- 确认不是 32 位系统（本包只提供 amd64）；
- 确认 `rsrc.syso` 没被删（没有它 manifest/版本资源会缺失，但通常仍能启动）；
- 如果改过 `frontend/dist`，确认 `index.html` 还在。

### 8.5 测试失败但不知道是不是环境问题

先跑：

```powershell
go test ./...
```

若失败项是 `manual` 测试，不要在普通环境跑；若默认测试失败，把完整输出贴出来。默认测试里涉及 WMI/剪贴板的项在 CI/无桌面环境可能跳过或失败，以实际输出为准。

---

## 9. 干净重建

```powershell
go clean -cache
Remove-Item .\bin -Recurse -Force -ErrorAction SilentlyContinue
go mod download
build.cmd release
```

不要删除：

- `frontend/dist/index.html`（Wails 启动必需）
- `rsrc.syso`（版本资源/图标/manifest）
- `go.sum`

---

## 10. 已知限制

- 未在 Windows 真机上完成 GUI/Defender/工具执行回归；本文是构建与测试方法，不代表结果已验证。
- 未做代码签名；SmartScreen/Defender 仍可能提示未知发布者。
- 优化项“还原到用户原值”的全量快照仍未实现（当前是固定还原值；已修正 `EnableLUA`、`ForegroundLockTimeout` 两个明显不安全/不准确的值）。
- `SpaceDelete` 已有 MFT 会话白名单与单文件限制，但 TOCTOU 仍需句柄/FRN 删除才能彻底消除。

更完整的审计结论见 `docs/audit/01–09`。
