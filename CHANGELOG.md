# 更新记录 / Changelog

## v1.0.1（2026-11-23）

构建与发布流程工程化，不改变功能行为。

### CI / 发布
- `build.yml` 增加 gofmt / vet / test 三道质量门禁、固定资源校验和、最小权限与并发控制；版本号统一由 `APPVER` 注入。
- 新增 `release.yml`：推送 `v*` tag 时自动构建并发布到 GitHub Releases，版本号取自 tag，产物附 SHA256。
- 版本号由 v1.0.0 升至 v1.0.1（真相源：各构建脚本的 `APPVER`）。

### 测试修复
- 修复 GitHub Actions（Windows runner）环境下 7 项测试失败：`normalizeDriveLetter` 带路径输入、空间删除测试补 MFT 会话白名单、风险分级测试去除本机文件依赖、工具 golden 基线同步。

## v1.0.0（2026-11-12）

二次开发基线版本。基于原项目 v1.1.0（GPLv3）整理，不包含原作者服务端绑定。

### 改名与身份
- 中文名统一为「ah系统优化工具箱」，短名 / 可执行文件 / 进程名为 `ahtool`。
- module、Wails 配置、manifest、窗口标题、页面标题、自启动值名、本机日志与临时目录、
  User-Agent、golden 基线全部改为新身份。
- 移除原作者 Logo、赞赏码、社群二维码与官网截图；新增中性 AH 标识。

### 版本
- 版本号统一为 **v1.0.0**，唯一真源为 `build.cmd` / `build.ps1` / `build.sh` 里的 `APPVER`，
  通过 `-X main.appVersion` 注入；前端版本号改为从 `GetAboutInfo()` 读取。
- 移除发布构建号与 `tools/relgen` 发布链；归档旧更新记录与发布流程。

### 解绑原作者服务
- **默认关闭在线更新与官方指纹校验**，启动时不再发起任何更新网络请求。
- 移除官网、技术文库、QQ 群、微信、赞赏、UU远程、远程服务等全部外链与素材。
- 「服务大厅」改为「帮助与诊断」：只保留本地自检清单与本机诊断报告。
- 诊断报告只在本机生成，不再带服务方联系方式。
- 测速功能不再通过明文 HTTP 查询并回显公网 IP / 归属地。

### 安全加固
- `deepjunk` 全盘清理增加运行时保护清单与重解析点（junction / 符号链接）拦截。
- 应用残留清理与强制清理在删除目录前重新执行目录安全裁决。
- 特权删除使用 `FILE_FLAG_OPEN_REPARSE_POINT`，避免误删链接目标。
- 注册表整树删除区分「确实不存在」与「拒绝访问」，不再把失败报成成功。
- UWP / 快捷方式等调用 PowerShell 的路径改为解析原生 System32 绝对路径。
- 需要真机 / 管理员权限的诊断型测试统一标记 `//go:build manual`，默认 `go test` 不扫真实磁盘。

### 第二轮审计修复（2026-11-12）
- `SpaceDelete` 只允许删除本次 MFT 扫描索引中解析为文件的路径；拒绝重解析点/运行时保护路径；删除只走单文件，移除 `os.RemoveAll` 兜底。
- 强制清理：注册表整树删前 `reg export`；系统保护区/共用容器键跳过。
- 驱动 `/force` 删除前用 `pnputil /enum-devices /drivers` 检查设备占用。
- `winold` 移除 `cmd /c takeown&icacls&rmdir` 字符串拼接，改参数化调用 + Go 删除链；`SpaceOpen` 移除 `cmd /c start` 兜底。
- `netNicRestart` 兼容英文 `Enabled`，接口名取第 4 列起。
- 修正优化还原安全值：`safe-3` 不再在还原时关闭 UAC；`ForegroundLockTimeout` 还原为系统默认 200000。
- 新增 `space_guard_test.go`：MFT 路径白名单与卸载命令解析的安全回归。
- 新增 `docs/audit/08-完成度真实性与可行性复核.md`。

### 启动修复
- 补回 `frontend/dist/index.html` 占位文件：Wails v2 的 assetserver 启动时强制要求
  embed 资产里存在 index.html，缺失会导致 `wails.Run` 失败，而 GUI 子系统没有控制台，
  表现为“双击无反应”。
- `main.go` 增加启动日志（`%LOCALAPPDATA%\ahtool\startup.log`）、panic 捕获、
  `wails.Run` 错误 MessageBox 和 `-diag` 环境报告。
- 新增 `TestEmbeddedAssetsHaveIndexHTML` 回归测试。

### 磁盘保护与健康（Phase 1）
- 新增磁盘画像：逻辑盘 + 物理盘型号/SSD-HDD/总线/健康状态/转速/SMR 提示（`diskprofile.go`）。
- 新增磁盘 I/O 模式：静音/均衡/极速，持久化到 `%LOCALAPPDATA%\ahtool\diskio.txt`；扫描、item 并发、删除并发按模式调整（`diskio.go`）。
- 机械盘深度清扫强制最多 2 路 worker，减少随机寻道声/卡顿（`junk_scan.go`）。
- 磁盘优化整理增加 SSD/HDD 感知提示（`tools_disk.go`）。
- 磁盘空间页新增 I/O 模式切换与物理磁盘健康卡片（`design/disk.js`、`design/disk.css`）。
- 新增 `docs/audit/10-磁盘保护性能与健康管理.md`。

### 磁盘快速检查（Phase 2）
- 新增 `RunDiskCheck`：物理盘健康、剩余空间、NTFS dirty 标志、disk/Ntfs/volmgr 事件日志（只读）。
- 磁盘页健康卡片增加「快速检查」入口，结果以弹窗展示并落盘到 `%LOCALAPPDATA%\ahtool\reports`。
- 新增 `diskcheck_test.go`（盘符归一化 / 介质标签）。

### 磁盘检查与管理（Phase 3 增量）
- 磁盘快速检查历史：本地 `diskhealth/history.json`，弹窗显示最近 5 次。
- 检查报告增加卷信息：文件系统、卷标、分区号、BitLocker 状态。
- `SpaceDelete` 生成删除清单：路径/原大小/原修改时间/成功失败，落到 `reports/delete_*.json`。
- SSD 选中时磁盘页增加「TRIM」按钮：二次确认后执行 `defrag /O`；非 SSD 拒绝。
- 新增 `docs/audit/10` 的 Phase 2/3 记录。

### 磁盘卷信息与碎片分析
- 新增 `GetVolumeDetails`：卷/分区只读信息（文件系统、分区号、BitLocker、健康）。
- 磁盘页新增「卷信息」弹窗；机械盘新增「分析碎片」（`defrag /A`，只读）。
- 新增 `AnalyzeDefrag` 绑定；不提供格式化/分区/改盘符/缩容。

### 修复物理磁盘健康显示 Unknown
- WMI 数值属性（MediaType/BusType/HealthStatus 等）原先用 `VARIANT.ToString()` 读取，go-ole 只支持 BSTR，导致全部为空、界面显示 Unknown。
- 改为 `variantText()`：优先 ToString，空则用 `Value()` 的数值兜底；新增回归测试 `wmi_variant_test.go`。
- 前端在字段仍缺失时显示「介质未知 · 总线未知」，不再出现两个 Unknown。

### 网络详情与 DNS 工具
- 本机配置 → 网络与音频新增「详细网络与安全」：IP/MAC/网关/DNS/DHCP/代理/hosts/防火墙/监听端口/安全提示。
- 工具库 → 上不了网新增「清除 DNS 解析缓存」「DNS 解析监控器」；工具总数 34→36，net 分类 7→9。
- 修复 WMI 数组属性读取（IP/DNS/网关）与数值属性（媒体/总线/健康）读取。
- 磁盘 I/O 模式文案与 HDD 极速确认提示。
- 新增 `docs/audit/11-最终审计与优化.md`。

### 启动崩溃修复
- WMI 数组属性先按元素类型分派（BSTR 才用 ToStringArray），避免数值/VARIANT 数组按 BSTR 读取触发访问违例。
- `GetPCFull` / `GetNetworkInfo` 增加 recover；启动预读失败不再导致窗口消失，前端显示“网络信息读取失败”。
- 修复 `NetEnabled` 布尔值大小写导致的网卡状态误判。

### 蓝屏/异常关机 + 网络洞察
- 蓝屏分析新增异常关机/蓝屏/WHEA 时间线、转储配置检查与归因建议（只读）。
- 新增独立「网络洞察」页：网卡/路由/代理/连接进程/端口暴露/ARP/共享发现/分段连通性。
- 应用卸载：卸载器退出 20 秒后列表项仍在时返回 stale，前端转残留清理，不再等 8 分钟。
- 网络洞察补充 WiFi SSID/信号、按进程聚合、SVG 已知拓扑、两次快照对比。
- 修复网络洞察页因 `join` 未定义导致的空白页。
- 新增 `docs/audit/12-蓝屏异常关机与网络洞察.md`。

### 工程整理
- 新增 `build.ps1` / `build.sh`，`build.cmd` 不再写死离线工具链路径，也不再发布。
- 新增 CI（Windows 构建 + 安全测试集）。
- 归档失效前端构建脚本、发布工具、旧日志与历史截图。
- 新增 `docs/audit/` 审计报告与二次开发说明。
- Go 源码统一 LF 并执行 `gofmt`；新增 `.gitattributes` 防止行尾回归。
