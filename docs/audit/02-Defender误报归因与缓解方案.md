# 02 · Win11 安全中心误报归因与缓解方案

- 对象：原项目 `3z9wOpt` v1.1.0（本分支 `ahtool` v1.0.0 继承其大部分代码与行为）
- 日期：2026-11-12
- 证据来源：`docs/false-positive-names.md`（历史记录，5 个检测名，63/71 引擎干净）+ 源码静态审计
- 未覆盖：本分支未重新提交多引擎扫描；以下归因是“代码行为 → 检测器可能触发的特征”的推断，不是厂商判定原文。

---

## 一、结论先说

Win11 安全中心（Microsoft Defender）把它判成木马，通常是**两条线叠加**：

1. **行为线**：这类系统维护工具集中了“管理员 + 批量删除 + 改注册表/服务/引导 + 读原始磁盘 + 结束进程 + 隐藏窗口调用 PowerShell”的高危动作。Defender 的 ASR / 行为监控 / 云信誉会把这种组合当成恶意软件或“潜在不需要的程序（PUP）”。
2. **信誉线**：单文件、无 Authenticode 签名、无稳定下载信誉、`-s -w` 去掉符号、体积/结构像打包器。Defender 的泛型 ML 模型（检测名带 `!ml`）在“新文件 + 低信誉 + 高危行为”时倾向于判可疑。

原记录里的 5 个检测名也分属这两类：`Trojan:Win32/Bearfoos.B!ml`、`Wk/malware_confidence_60%` 是泛型/ML；腾讯 `Qhost`、Kaspersky `Qhost`、Bkav `W32.Malware...` 更偏向“hosts/浏览器劫持”行为特征。

---

## 二、最可能触发的具体行为（按信号强度）

### 1. `Qhost` 系：改写/重置 hosts 文件（最强关联）

- `tools_net.go:455` `toolNetHosts` 读取 `%windir%\System32\drivers\etc\hosts` 并备份后用记事本打开。
- `tools_net.go:481` `netHostsReset` **直接覆写 hosts 为默认空内容**（`defaultHosts`，`tools_net.go:495`）。
- “管理员程序写 hosts”是 Qhost（hosts 劫持）家族的经典行为。腾讯电脑管家 `Win32.Trojan.Qhost.AdH`、Kaspersky `Trojan.Win32.Qhost.qbpg` 很可能由此触发。
- 说明：原意是“重置被劫持的 hosts”，但检测器只看到“写 hosts”，不会区分善意。
- **已做**：本分支保留“编辑 + 备份”，但建议把“重置为空”改为“仅删除本次工具添加的行”或默认不提供；若保留，必须在 UI 二次确认并展示将写入的内容。

### 2. 原始磁盘访问 + MFT 解析

- `mftscan.go:86` `DeviceIoControl(FSCTL_GET_NTFS_VOLUME_DATA)`；`space.go:87` 读卷属性。
- 打开 `\\.\C:` 并直接读 MFT，是勒索软件/取证工具常见行为；Defender 对“非微软进程直接读卷”较敏感。
- 同类工具（WizTree、Everything）也会被部分引擎误报，但通常有签名与信誉。
- 建议：保留功能，但增加“仅 NTFS、只读、不写卷”的显式说明；考虑把该功能拆到独立可选组件（降低主程序行为面）。

### 3. 删还原点 / 清 Defender 隔离区 / 截断事件日志

- `junk_clean.go:916` `vssadmin delete shadows /all /quiet`。
- `junk_items.go:276-280`、`junk_clean.go:417-448` 清 `Microsoft\Windows Defender\Scans\History` 与 `Quarantine`。
- `junk_clean.go:937` `wevtutil cl Security/System/Application/Setup`。
- 这三类动作在恶意软件里常用来“销毁取证痕迹”，是行为引擎的高权重信号。
- 建议：默认关闭；UI 明确“会删除还原点/隔离区/审计日志”；不要提供“一键全选深度清理”把它们一起勾上。

### 4. 结束进程 + 重启删除 + 驱动强删

- `junk_locker.go:407`、`appclean.go:1611/1621` `TerminateProcess`。
- `junk_rebootdel.go:84` `MoveFileEx(..., MOVEFILE_DELAY_UNTIL_REBOOT)`。
- `drivers.go:727` `pnputil /delete-driver ... /uninstall /force`。
- 这些是“清理被占用文件/顽固软件”的合理手段，但组合起来像“AV 对抗/持久化清除”。
- 建议：结束进程前列出进程名与路径并确认；只结束用户态、非安全软件进程（代码已有保护清单 `junk_locker.go:512`）；不要默认勾选“结束占用进程”。

### 5. 隐藏窗口 + PowerShell `-ExecutionPolicy Bypass`

- `tools_net.go:253/263/332`：`powershell.exe -NoProfile -ExecutionPolicy Bypass -Command ...`，`HideWindow`。
- `junk_clean.go:1070`、`junk_reg.go:432` 也有隐藏 PowerShell 调用。
- “隐藏窗口 + Bypass 策略”是 PowerShell 恶意脚本的高频特征。
- 建议：能用原生 API / WMI / `netsh` 的不要用 PowerShell；必须用时去掉 `-ExecutionPolicy Bypass`（改用 `-NoProfile -NonInteractive`），并避免 `HideWindow`（或至少不用 `CREATE_NO_WINDOW` 组合）。本分支已把裸 `powershell` 改为 System32 绝对路径，但这不改变行为特征。

### 6. 自更新：复制自身、覆盖自身、重启

- `update.go:661-690` 把当前 exe 复制到 `%TEMP%\ahtool_update\ahtool_updater.exe`，以 `-apply-update` 启动副本；`update.go:723-782` 等主进程退出后覆盖目标 exe、失败回滚、再启动。
- “进程复制自身 + 覆盖 exe + 重启”与某些窃密/勒索 loader 的自我替换链高度相似。
- 本分支已默认关闭在线更新；若恢复，建议使用独立签名的更新器、清单签名、Authenticode 校验，并避免“复制自身”这一形态。

### 7. 无签名、单文件、`-s -w`

- 无 Authenticode 签名（原 README 自述；本分支构建也没有证书）。
- `build.cmd` / `build.sh` 使用 `-s -w`（去符号与 DWARF），单文件 14MB。
- 这些会降低 Defender 的“可信发布者/可信构建”评分，使泛型 ML 更容易给出 `!ml` 或 `malware_confidence_60%`。

### 8. 启动即提权 + 高权限 API

- `app.manifest:9` `requireAdministrator`。
- `junk_priv.go:83-122` 启用 `SeBackupPrivilege` / `SeRestorePrivilege`，可绕过 ACL 删除。
- 这些是合法备份工具也会用的能力，但会提高“高权限未知程序”的风险分。

---

## 三、为什么多数引擎判干净

- 没有 C2 域名、没有混淆/加壳、没有持久化（自启动是用户可选）、没有窃取浏览器密码/键盘记录、没有加密文件。
- 源码可读、行为有明确 UI 与日志、删除目标有清单与保护规则。
- 行为虽高危，但属于系统维护工具的“合理组合”；有签名的同类工具通常会被放行。
- 63/71 引擎干净说明“不是明确的恶意样本”，更像是**低信誉 + 高危行为**触发的泛型判定。

---

## 四、缓解方案（按优先级）

### P0 · 立刻可做（成本低、收益高）

1. **代码签名 + 时间戳**：购买 OV/EV 证书，使用 `signtool sign /fd SHA256 /tr <timestamp> /td SHA256`。这是降低 SmartScreen/Defender 误报最有效的一步。EV 证书对 SmartScreen 信誉积累更快。
2. **完整版本资源**：本分支已用 `go-winres` 重建 `rsrc.syso`，包含 `ProductName`、`FileDescription`、`FileVersion=1.0.0.0`、`LegalCopyright`、图标。原版 `rsrc.syso` 内含旧品牌与旧版本，已替换。
3. **不要用 `-s -w` 隐藏构建信息**：至少保留符号表或单独发布 PDB；`-s -w` 本身不是恶意的，但会降低可分析性。
4. **收敛 hosts 行为**：删除“重置为空”或改为“只删除本工具写入的行”；保留备份。预计对 Qhost 系检测最有效。
5. **默认关闭高风险清理项**：还原点、Defender 隔离区、事件日志、驱动强删、结束进程、WinSxS `/ResetBase`。UI 必须逐项确认。
6. **去掉隐藏 PowerShell Bypass**：能原生 API 就原生；必须用时改为非隐藏 + 不加 Bypass。

### P1 · 需要一些工程（1–2 周）

7. **可复现构建**：固定 Go 版本、`GOPROXY`、`-trimpath`、记录 SHA256；发布页同时给源码 tag 与构建命令。可复现构建能显著提高“这是正常开源软件”的可信度。
8. **多引擎基线**：每次发布前用 VirusTotal / 多引擎扫描，记录 63/71 这类基线；发现新检测名后按厂商提交误报申诉（附 SHA256、源码地址、行为说明）。
9. **清单签名 + 更新器独立签名**：若恢复在线更新，清单用 Ed25519/RSA 签名，更新器 exe 单独签名并在替换前校验 Authenticode。
10. **行为收敛**：
    - `TerminateProcess` 只对用户明确勾选的进程；
    - `pnputil /force` 删除前枚举使用该驱动的设备；
    - `vssadmin delete shadows` 改为“删除指定还原点”而非 `/all`；
    - 不再直接清空 Defender `Quarantine` 目录（改为调用 Defender 官方 API/命令，或只清历史记录）。
11. **自更新形态改造**：不要“复制自身覆盖自身”；改为独立签名的 updater（或使用 MSIX/安装器）。

### P2 · 可选（会牺牲部分功能/便利）

12. 把“全盘 MFT 扫描”“深度清理”“驱动强删”拆成独立可选组件或高级模式，主程序默认只做只读扫描。
13. 首次启动显示“本工具会请求管理员权限并执行删除/注册表修改”，并提供“只读模式”。
14. 发布页提供 SHA256、签名证书指纹、构建来源（CI artifact attestation）。

### 明确会降低误报但牺牲功能的改动

| 改动 | 降低误报 | 牺牲 |
|---|---|---|
| 不再重置 hosts | 高（Qhost 系） | 失去“主页劫持一键修复” |
| 不再清 Defender 隔离区/还原点/事件日志 | 高 | 深度清理能力下降 |
| 不再结束进程 / 强删驱动 | 中高 | 顽固软件/驱动清理能力下降 |
| 不再自更新 | 中 | 用户需手动下载新版本 |
| 只读模式 | 中 | 所有清理/修复功能不可用 |

---

## 五、复现与验证建议（未在本环境执行）

1. 在 Windows 11 VM 中关闭实时保护前先记录 Defender 版本；用 `Get-MpComputerStatus`。
2. 对 exe 计算 SHA256：`Get-FileHash .\ahtool_v1.0.0.exe -Algorithm SHA256`。
3. 上传 VirusTotal（不要上传含个人信息的构建）；记录每个引擎的检测名与版本。
4. 在 VM 中开启 Defender 审计/ASR 规则，运行 `-cli -nofull`（只读自检）观察是否触发；再逐项测试高风险功能（hosts 重置、还原点删除、驱动删除）。
5. 用 `MpCmdRun.exe -Scan -ScanType 3 -File <path>` 做本地扫描。
6. 误报申诉：Microsoft Security Intelligence 提交“软件误报”，附 SHA256、下载地址、源码地址、功能说明、签名证书。

---

## 六、本分支已落地的相关改动

- 在线更新默认关闭（`update.go:49-55`）。
- 移除原作者域名/社群/远程服务（`service.go`）。
- 重建版本资源（`rsrc.syso`，见 07）。
- 运行时保护与重解析点拦截（`junk_clean.go:1042`、`junk_priv.go:157/203`）。
- 卸载命令不再走 `cmd /c`（`appclean.go:1296-1320`）。
- 明文公网 IP 查询停用（`speedtest.go:527-534`）。
- 这些改动会**降低一部分**行为信号，但**没有代码签名**，因此不能保证 Defender 不再误报。

> 交叉引用：[01 安全性与真实性](01-安全性与真实性评估报告.md) · [03 工具库](03-工具库机制与可用性核验.md) · [05 清理清单](05-清理清单与二次开发基线.md)。
