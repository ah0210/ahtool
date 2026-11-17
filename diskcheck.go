package main

// 磁盘快速检查（Phase 2，只读）。
//
// 只做四件事：
//   1. 物理盘健康（MSFT_PhysicalDisk / Win32_DiskDrive 回退）
//   2. 卷剩余空间
//   3. NTFS dirty 标志（fsutil dirty query）
//   4. System 日志里 disk / Ntfs / volmgr 的错误/警告（wevtutil 只读查询）
//
// 不运行 chkdsk /f、不写注册表、不删除文件。需要深度扫描时引导用户去工具库跑
// chkdsk /scan（只读）。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type DiskCheckItem struct {
	Level  string `json:"level"` // ok / info / warn / bad
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

type DiskCheckReport struct {
	Drive      string          `json:"drive"`
	Model      string          `json:"model"`
	Media      string          `json:"media"`
	Bus        string          `json:"bus"`
	Health     string          `json:"health"`
	Total      int64           `json:"total"`
	Free       int64           `json:"free"`
	Healthy    bool            `json:"healthy"`
	Items      []DiskCheckItem `json:"items"`
	Generated  string          `json:"generated"`
	ReportPath string          `json:"reportPath"`
	Note       string          `json:"note"`
	Volume     VolumeDetail    `json:"volume"`
}

type VolumeDetail struct {
	Drive           string `json:"drive"`
	Label           string `json:"label"`
	FileSystem      string `json:"fileSystem"`
	Health          string `json:"health"`
	Size            int64  `json:"size"`
	Free            int64  `json:"free"`
	DiskNumber      int    `json:"diskNumber"`
	PartitionNumber int    `json:"partitionNumber"`
	BitLocker       string `json:"bitLocker"` // On / Off / Unknown
}

type DiskHealthPoint struct {
	Time      string `json:"time"`
	Drive     string `json:"drive"`
	Model     string `json:"model"`
	Media     string `json:"media"`
	Health    string `json:"health"`
	Healthy   bool   `json:"healthy"`
	Total     int64  `json:"total"`
	Free      int64  `json:"free"`
	ItemCount int    `json:"itemCount"`
	Report    string `json:"report"`
}

func normalizeDriveLetter(d string) (string, bool) {
	d = strings.ToUpper(strings.TrimSpace(d))
	d = strings.TrimSuffix(d, `\`)
	if len(d) == 1 && d[0] >= 'A' && d[0] <= 'Z' {
		d += ":"
	}
	if len(d) != 2 || d[1] != ':' || d[0] < 'A' || d[0] > 'Z' {
		return "", false
	}
	return d, true
}

// physicalForDrive 把盘符映射到 MSFT_Partition.DiskNumber，再找物理盘画像。
func physicalForDrive(drive string) (PhysicalDiskInfo, bool) {
	letter := int(drive[0])
	rows, err := wmiQuery(`root\Microsoft\Windows\Storage`,
		"SELECT DiskNumber,DriveLetter FROM MSFT_Partition",
		[]string{"DiskNumber", "DriveLetter"})
	if err != nil {
		return PhysicalDiskInfo{}, false
	}
	diskNum := -1
	for _, r := range rows {
		raw := strings.TrimSpace(r["DriveLetter"])
		if raw == "" {
			continue
		}
		n, err := strconv.Atoi(raw)
		if err == nil {
			if n == letter {
				diskNum, _ = strconv.Atoi(strings.TrimSpace(r["DiskNumber"]))
				break
			}
			continue
		}
		if strings.EqualFold(raw, string(rune(letter))) {
			diskNum, _ = strconv.Atoi(strings.TrimSpace(r["DiskNumber"]))
			break
		}
	}
	if diskNum < 0 {
		return PhysicalDiskInfo{}, false
	}
	p := (&App{}).GetDiskProfiles()
	for _, d := range p.Physical {
		if d.Index == diskNum {
			return d, true
		}
	}
	return PhysicalDiskInfo{}, false
}

func diskCheckEventLog(drive string) (string, int, error) {
	q := `*[System[(Level=1 or Level=2 or Level=3) and (Provider[@Name='disk'] or Provider[@Name='Ntfs'] or Provider[@Name='volmgr'] or Provider[@Name='volsnap'])]]`
	out, err := runHiddenOut(30*time.Second, resolveExe("wevtutil.exe"),
		"qe", "System", "/q:"+q, "/f:text", "/c:10", "/rd:true")
	txt := strings.TrimSpace(decodeConsole([]byte(out)))
	if err != nil {
		return txt, 0, err
	}
	if txt == "" || strings.Contains(strings.ToLower(txt), "no events") {
		return "", 0, nil
	}
	// wevtutil /f:text 每条事件以空行分隔；粗略按“事件[”或空行计数。
	n := strings.Count(txt, "事件[")
	if n == 0 {
		n = strings.Count(txt, "Event[")
	}
	if n == 0 {
		n = 1
	}
	return txt, n, nil
}

func diskHealthHistoryPath() string {
	if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
		return filepath.Join(local, "ahtool", "diskhealth", "history.json")
	}
	return filepath.Join(os.TempDir(), "ahtool_diskhealth_history.json")
}

func appendDiskHealthHistory(rep DiskCheckReport) {
	p := diskHealthHistoryPath()
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	var list []DiskHealthPoint
	if b, err := os.ReadFile(p); err == nil {
		_ = json.Unmarshal(b, &list)
	}
	list = append(list, DiskHealthPoint{
		Time: rep.Generated, Drive: rep.Drive, Model: rep.Model, Media: rep.Media,
		Health: rep.Health, Healthy: rep.Healthy, Total: rep.Total, Free: rep.Free,
		ItemCount: len(rep.Items), Report: rep.ReportPath,
	})
	if len(list) > 200 {
		list = list[len(list)-200:]
	}
	if b, err := json.Marshal(list); err == nil {
		_ = os.WriteFile(p, b, 0o644)
	}
}

// GetDiskHealthHistory 返回某块盘的最近检查历史（最多 20 条）。
func (a *App) GetDiskHealthHistory(drive string) []DiskHealthPoint {
	d, ok := normalizeDriveLetter(drive)
	if !ok {
		return []DiskHealthPoint{}
	}
	b, err := os.ReadFile(diskHealthHistoryPath())
	if err != nil {
		return []DiskHealthPoint{}
	}
	var all []DiskHealthPoint
	if json.Unmarshal(b, &all) != nil {
		return []DiskHealthPoint{}
	}
	out := make([]DiskHealthPoint, 0, 20)
	for i := len(all) - 1; i >= 0 && len(out) < 20; i-- {
		if strings.EqualFold(all[i].Drive, d) {
			out = append(out, all[i])
		}
	}
	return out
}

func volumeDetailsFor(drive string) VolumeDetail {
	out := VolumeDetail{Drive: drive, Health: "Unknown", BitLocker: "Unknown"}
	letter := int(drive[0])
	if rows, err := wmiQuery(`root\Microsoft\Windows\Storage`,
		"SELECT DriveLetter,FileSystem,FileSystemLabel,HealthStatus,Size,SizeRemaining FROM MSFT_Volume",
		[]string{"DriveLetter", "FileSystem", "FileSystemLabel", "HealthStatus", "Size", "SizeRemaining"}); err == nil {
		for _, r := range rows {
			raw := strings.TrimSpace(r["DriveLetter"])
			match := false
			if n, e := strconv.Atoi(raw); e == nil {
				match = n == letter
			} else {
				match = strings.EqualFold(raw, string(rune(letter)))
			}
			if !match {
				continue
			}
			out.Label = strings.TrimSpace(r["FileSystemLabel"])
			out.FileSystem = strings.TrimSpace(r["FileSystem"])
			out.Health = healthStatusName(r["HealthStatus"])
			out.Size, _ = strconv.ParseInt(strings.TrimSpace(r["Size"]), 10, 64)
			out.Free, _ = strconv.ParseInt(strings.TrimSpace(r["SizeRemaining"]), 10, 64)
			break
		}
	}
	if rows, err := wmiQuery(`root\Microsoft\Windows\Storage`,
		"SELECT DiskNumber,PartitionNumber,DriveLetter FROM MSFT_Partition",
		[]string{"DiskNumber", "PartitionNumber", "DriveLetter"}); err == nil {
		for _, r := range rows {
			raw := strings.TrimSpace(r["DriveLetter"])
			match := false
			if n, e := strconv.Atoi(raw); e == nil {
				match = n == letter
			} else {
				match = strings.EqualFold(raw, string(rune(letter)))
			}
			if !match {
				continue
			}
			out.DiskNumber, _ = strconv.Atoi(strings.TrimSpace(r["DiskNumber"]))
			out.PartitionNumber, _ = strconv.Atoi(strings.TrimSpace(r["PartitionNumber"]))
			break
		}
	}
	// BitLocker 状态：读不到就 Unknown（不推断）
	if rows, err := wmiQuery(`root\cimv2\Security\MicrosoftVolumeEncryption`,
		"SELECT DriveLetter,ProtectionStatus FROM Win32_EncryptableVolume",
		[]string{"DriveLetter", "ProtectionStatus"}); err == nil {
		for _, r := range rows {
			if !strings.EqualFold(strings.TrimSpace(r["DriveLetter"]), drive) {
				continue
			}
			switch strings.TrimSpace(r["ProtectionStatus"]) {
			case "1":
				out.BitLocker = "On"
			case "0":
				out.BitLocker = "Off"
			default:
				out.BitLocker = "Unknown"
			}
			break
		}
	}
	return out
}

// RunDiskCheck 对指定盘做一次快速只读检查。
func (a *App) RunDiskCheck(drive string) DiskCheckReport {
	d, ok := normalizeDriveLetter(drive)
	rep := DiskCheckReport{Drive: d, Generated: time.Now().Format("2006-01-02 15:04:05"), Healthy: true, Note: "本检查只读：不修复、不删除。深度扫描请到工具库运行「磁盘错误扫描」（chkdsk /scan）。"}
	if !ok {
		rep.Healthy = false
		rep.Items = append(rep.Items, DiskCheckItem{Level: "bad", Title: "盘符无效", Detail: "请传 C: / D: 这样的盘符"})
		return rep
	}

	vols := a.GetDiskList()
	for _, v := range vols {
		if strings.EqualFold(v.Drive, d) {
			rep.Total, rep.Free = v.Total, v.Free
			rep.Model = v.Label
			rep.Media = diskMediaName(v.SSD)
			break
		}
	}
	if rep.Total == 0 {
		rep.Healthy = false
		rep.Items = append(rep.Items, DiskCheckItem{Level: "bad", Title: "找不到卷", Detail: d + " 不是固定磁盘或当前不可读"})
		return rep
	}

	rep.Volume = volumeDetailsFor(d)
	if rep.Volume.FileSystem != "" {
		rep.Items = append(rep.Items, DiskCheckItem{Level: "info", Title: "卷信息",
			Detail: fmt.Sprintf("%s · %s · 分区 %d · BitLocker %s", rep.Volume.FileSystem, rep.Volume.Label, rep.Volume.PartitionNumber, rep.Volume.BitLocker)})
	}

	// 物理盘健康
	if p, ok := physicalForDrive(d); ok {
		rep.Model, rep.Media, rep.Bus, rep.Health = p.Name, p.Media, p.Bus, p.Health
		switch p.Health {
		case "Healthy":
			rep.Items = append(rep.Items, DiskCheckItem{Level: "ok", Title: "物理盘健康", Detail: p.Name + " · " + p.Media + " · " + p.Bus})
		case "Warning", "Unhealthy":
			rep.Healthy = false
			rep.Items = append(rep.Items, DiskCheckItem{Level: "bad", Title: "物理盘健康异常", Detail: p.Name + " 报告 " + p.Health + "；请立即备份并检查 SMART"})
		default:
			rep.Items = append(rep.Items, DiskCheckItem{Level: "info", Title: "物理盘健康未知", Detail: p.Name + " · 未读到健康状态"})
		}
		if p.SMR == "likely" {
			rep.Items = append(rep.Items, DiskCheckItem{Level: "info", Title: "疑似 SMR 盘", Detail: "大量随机写入可能变慢/有异响；避免频繁深度清理和碎片整理"})
		}
	} else {
		rep.Items = append(rep.Items, DiskCheckItem{Level: "info", Title: "物理盘信息未映射", Detail: "读到了卷，但未能把盘符映射到物理盘健康信息"})
	}

	// 剩余空间
	if rep.Total > 0 {
		pct := float64(rep.Free) / float64(rep.Total) * 100
		switch {
		case pct < 5:
			rep.Healthy = false
			rep.Items = append(rep.Items, DiskCheckItem{Level: "bad", Title: "剩余空间极低", Detail: fmt.Sprintf("仅剩 %.1f%%（%s），系统更新/临时文件可能失败", pct, fmtBytes(rep.Free))})
		case pct < 10:
			rep.Items = append(rep.Items, DiskCheckItem{Level: "warn", Title: "剩余空间偏低", Detail: fmt.Sprintf("剩余 %.1f%%（%s），建议清理到 15%% 以上", pct, fmtBytes(rep.Free))})
		default:
			rep.Items = append(rep.Items, DiskCheckItem{Level: "ok", Title: "剩余空间", Detail: fmt.Sprintf("剩余 %.1f%%（%s）", pct, fmtBytes(rep.Free))})
		}
	}

	// NTFS dirty 标志（只读）
	if out, err := runHiddenOut(20*time.Second, resolveExe("fsutil.exe"), "dirty", "query", d); err == nil {
		txt := strings.ToLower(decodeConsole([]byte(out)))
		if strings.Contains(txt, "not dirty") || strings.Contains(txt, "不脏") || strings.Contains(txt, "未标记为脏") {
			rep.Items = append(rep.Items, DiskCheckItem{Level: "ok", Title: "NTFS 脏标志", Detail: "卷未被标记为脏"})
		} else if strings.Contains(txt, "dirty") || strings.Contains(txt, "脏") {
			rep.Healthy = false
			rep.Items = append(rep.Items, DiskCheckItem{Level: "warn", Title: "NTFS 脏标志已置位", Detail: "上次关机/写入异常；建议重启后运行 chkdsk /scan（只读）确认"})
		} else {
			rep.Items = append(rep.Items, DiskCheckItem{Level: "info", Title: "NTFS 脏标志", Detail: "输出无法识别：" + strings.TrimSpace(decodeConsole([]byte(out)))})
		}
	} else {
		rep.Items = append(rep.Items, DiskCheckItem{Level: "info", Title: "NTFS 脏标志", Detail: "fsutil 不可用或查询失败（可能不是 NTFS / 权限不足）"})
	}

	// 事件日志：disk / Ntfs / volmgr / volsnap 的 Error/Warning
	if txt, n, err := diskCheckEventLog(d); err != nil {
		rep.Items = append(rep.Items, DiskCheckItem{Level: "info", Title: "事件日志", Detail: "查询失败：" + shortErr(err)})
	} else if n == 0 {
		rep.Items = append(rep.Items, DiskCheckItem{Level: "ok", Title: "事件日志", Detail: "最近没有 disk/Ntfs/volmgr 的错误或警告"})
	} else {
		rep.Healthy = false
		detail := txt
		if len(detail) > 1200 {
			detail = detail[:1200] + "…"
		}
		rep.Items = append(rep.Items, DiskCheckItem{Level: "warn", Title: fmt.Sprintf("事件日志有 %d 条磁盘错误/警告", n), Detail: detail})
	}

	// 报告落盘（本地，不上传）
	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "ahtool", "reports")
	if err := os.MkdirAll(dir, 0o755); err == nil {
		p := filepath.Join(dir, "diskcheck_"+strings.TrimSuffix(d, ":")+"_"+time.Now().Format("20060102_150405")+".txt")
		var b strings.Builder
		fmt.Fprintf(&b, "ah系统优化工具箱 磁盘快速检查\n盘符: %s\n时间: %s\n健康: %v\n\n", d, rep.Generated, rep.Healthy)
		for _, it := range rep.Items {
			fmt.Fprintf(&b, "[%s] %s\n%s\n\n", strings.ToUpper(it.Level), it.Title, it.Detail)
		}
		if err := os.WriteFile(p, []byte(b.String()), 0o644); err == nil {
			rep.ReportPath = p
		}
	}
	appendDiskHealthHistory(rep)
	return rep
}

func diskMediaName(ssd bool) string {
	if ssd {
		return "SSD"
	}
	return "HDD"
}

// TrimDrive 对 SSD 执行一次 TRIM/retrim（defrag /O）。只允许 SSD，避免用户误在
// 机械盘上做“优化整理”。成功返回空串，失败返回中文原因。
func (a *App) TrimDrive(drive string) string {
	d, ok := normalizeDriveLetter(drive)
	if !ok {
		return "盘符无效"
	}
	if !diskIsSSDByDrive(d) {
		return "该盘不是 SSD（或读不到介质类型），已拒绝执行 TRIM"
	}
	out, err := runHiddenOut(10*time.Minute, resolveExe("defrag.exe"), d, "/O")
	if err != nil {
		txt := strings.TrimSpace(decodeConsole([]byte(out)))
		if len(txt) > 300 {
			txt = txt[:300] + "…"
		}
		if txt != "" {
			return "TRIM 失败：" + shortErr(err) + "；输出：" + txt
		}
		return "TRIM 失败：" + shortErr(err)
	}
	return ""
}

// GetVolumeDetails 返回所有固定卷的只读信息（文件系统/分区号/BitLocker/健康）。
func (a *App) GetVolumeDetails() []VolumeDetail {
	vols := a.GetDiskList()
	out := make([]VolumeDetail, 0, len(vols))
	for _, v := range vols {
		d := volumeDetailsFor(v.Drive)
		if d.Label == "" {
			d.Label = v.Label
		}
		if d.Size == 0 {
			d.Size = v.Total
		}
		if d.Free == 0 {
			d.Free = v.Free
		}
		out = append(out, d)
	}
	return out
}

// AnalyzeDefrag 只读分析碎片（defrag /A），返回原始输出；不做整理。
func (a *App) AnalyzeDefrag(drive string) string {
	d, ok := normalizeDriveLetter(drive)
	if !ok {
		return "盘符无效"
	}
	out, err := runHiddenOut(30*time.Minute, resolveExe("defrag.exe"), d, "/A")
	txt := strings.TrimSpace(decodeConsole([]byte(out)))
	if err != nil {
		if txt != "" {
			return "分析失败：" + shortErr(err) + "\n\n" + txt
		}
		return "分析失败：" + shortErr(err)
	}
	if txt == "" {
		return "分析完成，但没有输出（该盘可能不支持或不需要碎片分析）"
	}
	return txt
}
