package main

// ============================================================================
// 本地帮助与诊断板块（二次开发版）
//
// 原版这一层同时承担在线服务大厅、付费远程服务、官网文章与联系方式分发。
// 本分支已解除原作者服务端绑定：这里只保留**纯本地**的自检清单与诊断报告，
// 不包含任何官网 / 社群 / 赞赏 / 远程服务外链，也不采集、不上传任何数据。
//
// 扩展方式：
//   - 新增一条自助排查：往 serviceGuides() 加一条（Steps 至少 2 步）；
//   - 诊断报告改字段：改 BuildDiagReport()，报告是纯文本，由用户自行复制。
// ============================================================================

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// svcAppVer 对外版本号：由构建注入 main.appVersion，前端不硬编码。
var svcAppVer = "v" + strings.TrimPrefix(strings.TrimSpace(appVersion), "v")

// diagVer 诊断报告里的工具版本：版本号 + 构建日期（有构建号时优先构建号）。
func diagVer() string {
	v := "v" + strings.TrimPrefix(strings.TrimSpace(appVersion), "v")
	if b := strings.TrimSpace(buildNum); b != "" {
		return v + "（构建 " + b + "）"
	}
	if buildDate != "" && buildDate != "unknown" {
		return v + "（构建 " + buildDate + "）"
	}
	return v
}

// ---------- 本机自检清单 ----------

// SvcGuide 一条按症状组织的本地自查清单。
type SvcGuide struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`    // 症状
	Steps    []string `json:"steps"`    // 自查步骤（从简到繁）
	ToolID   string   `json:"toolId"`   // 可选的工具库 ID
	PageID   string   `json:"pageId"`   // 可选的软件内页面
	ActLabel string   `json:"actLabel"` // 按钮文案
}

func serviceGuides() []SvcGuide {
	return []SvcGuide{
		{
			ID: "g-slow", Title: "开机慢、越用越卡",
			Steps: []string{
				"看启动项：把不认识的、用不上的开机自启关掉（这一条最见效）",
				"清一遍系统垃圾与浏览器缓存",
				"看 C 盘剩余空间：低于 10% 会明显拖慢系统",
				"做完还是卡，多半是硬盘老化或后台服务异常，建议检查磁盘健康与事件日志",
			},
			PageID: "sec", ActLabel: "打开启动项管理",
		},
		{
			ID: "g-cdisk", Title: "C 盘爆满、总是红色警告",
			Steps: []string{
				"先用「磁盘空间」逐层钻取，看清到底哪个目录最占地方",
				"清理系统垃圾、休眠文件、旧系统还原点",
				"微信 / QQ 聊天记录、桌面大文件搬到其他盘",
				"要无损扩容 C 盘（动分区）有一定风险，建议先备份再做",
			},
			PageID: "space", ActLabel: "打开磁盘空间分析",
		},
		{
			ID: "g-bsod", Title: "开机蓝屏、进不去系统",
			Steps: []string{
				"先在本机读一遍蓝屏转储：能进系统就直接看错误码和嫌疑驱动",
				"能进系统：优先更新或回滚显卡、网卡驱动，卸载最近装的补丁",
				"进不去系统：用 WinRE 或安全模式排查，别急着重装",
				"蓝屏反复出现通常不是单一原因，逐项记录错误码再判断",
			},
			PageID: "bsod", ActLabel: "打开蓝屏分析",
		},
		{
			ID: "g-bsodcode", Title: "蓝屏代码看不懂是什么原因",
			Steps: []string{
				"有转储文件：用蓝屏分析读本机记录，直接给中文结论和处理建议",
				"没有转储：记录错误码，结合最近安装的驱动 / 更新一起排查",
				"仍不确定是哪个驱动：在蓝屏分析页查看嫌疑驱动列表",
			},
			PageID: "bsod", ActLabel: "打开蓝屏分析",
		},
		{
			ID: "g-nosound", Title: "电脑没声音了",
			Steps: []string{
				"先排除硬件：音箱 / 耳机插对孔了吗，是不是静音或选错输出设备",
				"看设备管理器：声卡有没有黄色感叹号（驱动问题）",
				"重启音频服务，或重装声卡驱动",
			},
			PageID: "driver", ActLabel: "打开驱动管理",
		},
		{
			ID: "g-black", Title: "黑屏只剩鼠标箭头",
			Steps: []string{
				"按 Ctrl+Alt+Del：能调出任务管理器就说明系统还活着",
				"任务管理器 → 运行新任务 → 输入 explorer.exe，桌面通常能回来",
				"反复出现说明系统文件或用户配置文件损坏，可用 SFC 修复",
			},
			ToolID: "fix-sfc", ActLabel: "执行 SFC 系统文件修复",
		},
		{
			ID: "g-nonet", Title: "上不了网、网卡掉线",
			Steps: []string{
				"先用「网络修复」一键重置 Winsock 与 TCP/IP（需重启生效）",
				"看网卡驱动状态，无线掉线经常是驱动或省电设置的问题",
				"家里有别的设备能上网吗？能上网说明是本机问题，不是宽带",
			},
			ToolID: "net-smart", ActLabel: "执行 网络修复",
		},
		{
			ID: "g-ads", Title: "弹窗广告关不掉、主页被劫持",
			Steps: []string{
				"先找出是谁弹的：任务管理器看可疑进程，或看最近装了什么软件",
				"用「应用清理」把来源不明的全家桶卸干净，再清残留",
				"清理浏览器快捷方式与主页设置，随后加拦截规则",
			},
			PageID: "appx", ActLabel: "打开应用清理",
		},
		{
			ID: "g-printer", Title: "打印机 / 局域网共享连不上",
			Steps: []string{
				"先重启打印服务（Print Spooler）",
				"再跑一次「局域网共享修复」，它会处理共享组件与 RPC 加固引起的连不上",
				"确认两台机器在同一网段，共享账号有权限",
			},
			ToolID: "net-lanfix", ActLabel: "执行 局域网共享修复",
		},
		{
			ID: "g-game", Title: "游戏掉帧、卡顿、延迟高",
			Steps: []string{
				"先跑一遍「系统优化」：关掉后台冗余、游戏相关的几个开关",
				"更新显卡驱动，并按显卡控制台的推荐设置调整",
				"看内存和磁盘余量：磁盘几乎满了会明显卡顿",
			},
			PageID: "optimize", ActLabel: "打开系统优化",
		},
	}
}

// GetGuides 返回本地自检清单。
func (a *App) GetGuides() []SvcGuide { return serviceGuides() }

// ---------- 关于页数据（前端不硬编码） ----------

type AboutInfo struct {
	Version   string `json:"version"`   // 版本号（侧栏与关于页共用）
	BuildDate string `json:"buildDate"` // 构建日期（构建时注入；未注入为 unknown）
	BuildNum  string `json:"buildNum"`  // 构建号（可选）
	License   string `json:"license"`   // 许可协议
	SelfHash  string `json:"selfHash"`  // 当前 exe 的 SHA256（本地自校验，不上传）
}

// GetAboutInfo 关于页一次性数据源。不包含任何联系方式或外部地址。
func (a *App) GetAboutInfo() AboutInfo {
	return AboutInfo{
		Version:   svcAppVer,
		BuildDate: buildDate,
		BuildNum:  buildNum,
		License:   "GPL-3.0",
		SelfHash:  a.GetSelfHash(),
	}
}

// SvcLinks 保留给前端兼容的链接表；本分支不再提供任何外部服务地址。
func (a *App) SvcLinks() map[string]string { return map[string]string{} }

// ---------- 本机诊断报告 ----------

// BuildDiagReport 生成一份纯本机诊断报告（纯文本，用户自行复制；不上传）。
func (a *App) BuildDiagReport(note string) string {
	var b strings.Builder
	line := func(f string, args ...interface{}) { fmt.Fprintf(&b, f+"\n", args...) }
	kv := func(k, v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			v = "—"
		}
		line("%-10s%s", k, v)
	}

	line("【ah系统优化工具箱 · 本机诊断报告】")
	kv("工具版本", diagVer())
	line("生成时间    %s", time.Now().Format("2006-01-02 15:04:05"))
	line("说明        本报告只在本机生成，不会自动上传")
	line("")

	line("── 系统 ──")
	os0 := wmiFirst(`root\cimv2`,
		"SELECT Caption,Version,BuildNumber,OSArchitecture,InstallDate,CSName FROM Win32_OperatingSystem",
		[]string{"Caption", "Version", "BuildNumber", "OSArchitecture", "InstallDate", "CSName"})
	kv("计算机名", os0["CSName"])
	kv("操作系统", os0["Caption"])
	ver, bn, arch := strings.TrimSpace(os0["Version"]), strings.TrimSpace(os0["BuildNumber"]), strings.TrimSpace(os0["OSArchitecture"])
	if bn != "" && !strings.Contains(ver, bn) {
		ver += " build " + bn
	}
	if arch != "" {
		ver += "（" + arch + "）"
	}
	kv("版本", ver)
	if t, ok := parseWmiDate(os0["InstallDate"]); ok {
		kv("安装日期", t.Format("2006-01-02"))
	}
	if up := systemUptimeMs(); up > 0 {
		d := time.Duration(up) * time.Millisecond
		kv("已开机", fmt.Sprintf("%d 天 %d 小时 %d 分", int(d.Hours())/24, int(d.Hours())%24, int(d.Minutes())%60))
	}
	line("")

	line("── 硬件 ──")
	if rows, err := wmiQuery(`root\cimv2`,
		"SELECT Name,NumberOfCores,NumberOfLogicalProcessors FROM Win32_Processor",
		[]string{"Name", "NumberOfCores", "NumberOfLogicalProcessors"}); err == nil && len(rows) > 0 {
		r := rows[0]
		kv("处理器", strings.TrimSpace(r["Name"]))
		cores, threads := strings.TrimSpace(r["NumberOfCores"]), strings.TrimSpace(r["NumberOfLogicalProcessors"])
		switch {
		case cores != "" && threads != "":
			kv("核心线程", cores+" 核 / "+threads+" 线程")
		case threads != "":
			kv("核心线程", threads+" 线程")
		case cores != "":
			kv("核心线程", cores+" 核")
		}
	} else {
		kv("处理器", "查询失败（"+shortErr(err)+"）")
	}

	mem := wmiFirst(`root\cimv2`, "SELECT TotalPhysicalMemory FROM Win32_ComputerSystem", []string{"TotalPhysicalMemory"})
	if tot, err := strconv.ParseInt(strings.TrimSpace(mem["TotalPhysicalMemory"]), 10, 64); err == nil && tot > 0 {
		if st, err2 := strconv.ParseInt(strings.TrimSpace(wmiFirst(`root\cimv2`,
			"SELECT FreePhysicalMemory FROM Win32_OperatingSystem", []string{"FreePhysicalMemory"})["FreePhysicalMemory"]), 10, 64); err2 == nil {
			kv("内存", fmt.Sprintf("%s（可用 %s）", fmtBytes(tot), fmtBytes(st*1024)))
		} else {
			kv("内存", fmtBytes(tot))
		}
	}

	if rows, err := wmiQuery(`root\cimv2`,
		"SELECT Name,DriverVersion FROM Win32_VideoController", []string{"Name", "DriverVersion"}); err == nil {
		for _, r := range rows {
			kv("显卡", strings.TrimSpace(r["Name"]+"（驱动 "+emptyDash(r["DriverVersion"])+"）"))
		}
	}

	mb := wmiFirst(`root\cimv2`, "SELECT Manufacturer,Product FROM Win32_BaseBoard", []string{"Manufacturer", "Product"})
	kv("主板", strings.TrimSpace(emptyDash(mb["Manufacturer"])+" "+emptyDash(mb["Product"])))
	bios := wmiFirst(`root\cimv2`, "SELECT SMBIOSBIOSVersion,ReleaseDate FROM Win32_BIOS", []string{"SMBIOSBIOSVersion", "ReleaseDate"})
	if t, ok := parseWmiDate(bios["ReleaseDate"]); ok {
		kv("BIOS", fmt.Sprintf("%s（%s）", emptyDash(bios["SMBIOSBIOSVersion"]), t.Format("2006-01-02")))
	}
	line("")

	line("── 磁盘 ──")
	if ds := (&App{}).GetDiskList(); len(ds) > 0 {
		for _, d := range ds {
			pct := 0.0
			if d.Total > 0 {
				pct = float64(d.Total-d.Free) / float64(d.Total) * 100
			}
			line("%-10s%s 总计 %s，可用 %s（已用 %.0f%%）", "磁盘", d.Drive, fmtBytes(d.Total), fmtBytes(d.Free), pct)
		}
	} else {
		kv("磁盘", "查询失败")
	}
	line("")

	line("── 故障线索 ──")
	dumps := (&App{}).BsodList()
	if len(dumps) == 0 {
		kv("蓝屏记录", "无转储文件")
	} else {
		kv("蓝屏记录", fmt.Sprintf("共 %d 次（列出最近 5 次）", len(dumps)))
		for i, d := range dumps {
			if i >= 5 {
				break
			}
			if !d.Valid {
				line("           %s  %s（解析失败：%s）", d.Time, d.Name, shortErrStr(d.Err))
				continue
			}
			drv := "未定位"
			if len(d.Drivers) > 0 {
				names := make([]string, 0, len(d.Drivers))
				for _, x := range d.Drivers {
					names = append(names, x.Name)
				}
				drv = strings.Join(names, " / ")
			}
			line("           %s  %s %s  嫌疑驱动：%s", d.Time, d.CodeHex, emptyDash(d.Zh), drv)
		}
	}
	if rb := (&App{}).GetPendingReboot(); rb.N > 0 {
		kv("待重启项", fmt.Sprintf("%d 项：%s", rb.N, strings.Join(rb.Items, "、")))
	} else {
		kv("待重启项", "无")
	}
	if s := strings.TrimSpace(note); s != "" {
		line("")
		line("── 用户自述 ──")
		line("%s", s)
	}
	line("")
	line("──────────────────────────────")
	line("由「ah系统优化工具箱」在本机生成，仅供自行排查参考")
	return b.String()
}

// shortErrStr 给字符串型错误做一次清洗（空的显示「未知」）
func shortErrStr(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "未知"
	}
	return s
}

// ---------- 剪贴板（写文本，供「复制诊断报告」用） ----------

var (
	svcU32               = syscall.NewLazyDLL("user32.dll")
	svcK32               = syscall.NewLazyDLL("kernel32.dll")
	procOpenClipboard    = svcU32.NewProc("OpenClipboard")
	procEmptyClipboard   = svcU32.NewProc("EmptyClipboard")
	procSetClipboardData = svcU32.NewProc("SetClipboardData")
	procCloseClipboard   = svcU32.NewProc("CloseClipboard")
	procGlobalAlloc      = svcK32.NewProc("GlobalAlloc")
	procGlobalLock       = svcK32.NewProc("GlobalLock")
	procGlobalUnlock     = svcK32.NewProc("GlobalUnlock")
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

// CopyText 把文本写进系统剪贴板；成功返回空串，失败返回原因。
// 剪贴板可能被别的程序短暂占用，所以重试几次。
func (a *App) CopyText(s string) string {
	if s == "" {
		return "内容为空"
	}
	utf, err := syscall.UTF16FromString(s)
	if err != nil {
		return "编码失败：" + err.Error()
	}
	n := uintptr(len(utf) * 2)

	var opened bool
	for i := 0; i < 8; i++ {
		if r, _, _ := procOpenClipboard.Call(0); r != 0 {
			opened = true
			break
		}
		time.Sleep(60 * time.Millisecond)
	}
	if !opened {
		return "剪贴板被其他程序占用，请稍后重试"
	}
	defer procCloseClipboard.Call()

	procEmptyClipboard.Call()
	h, _, _ := procGlobalAlloc.Call(gmemMoveable, n)
	if h == 0 {
		return "内存分配失败"
	}
	ptr, _, _ := procGlobalLock.Call(h)
	if ptr == 0 {
		return "内存锁定失败"
	}
	/* GlobalLock 把内存指针当返回值交出来（uintptr）。直接 uintptr→unsafe.Pointer 会被 go vet
	   判为 unsafe.Pointer 误用；这里用一次类型重解释把指针取回来 —— 二进制语义与转换完全等价。 */
	p := *(*unsafe.Pointer)(unsafe.Pointer(&ptr))
	copy(unsafe.Slice((*byte)(p), n), unsafe.Slice((*byte)(unsafe.Pointer(&utf[0])), n))
	procGlobalUnlock.Call(h)
	if r, _, e := procSetClipboardData.Call(cfUnicodeText, h); r == 0 {
		return "写入剪贴板失败：" + shortErr(e)
	}
	return ""
}
