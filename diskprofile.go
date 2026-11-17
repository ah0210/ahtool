package main

// 磁盘画像 + 健康摘要（Phase 1）。
//
// 数据来源优先级：
//   1. root\Microsoft\Windows\Storage 的 MSFT_PhysicalDisk（物理盘型号/介质/总线/健康）
//   2. 回退 root\cimv2 的 Win32_DiskDrive（型号/接口/状态）
//   3. 逻辑盘信息来自 GetDiskList（盘符/卷标/容量/SSD 标记）
//
// 读不到就返回 unknown，不编造健康值。

import (
	"strconv"
	"strings"
)

type PhysicalDiskInfo struct {
	Index      int    `json:"index"`
	Name       string `json:"name"`
	Media      string `json:"media"`  // SSD / HDD / SCM / Unknown
	Bus        string `json:"bus"`    // NVMe / SATA / USB / RAID / Unknown
	Health     string `json:"health"` // Healthy / Warning / Unhealthy / Unknown
	Status     string `json:"status"` // OperationalStatus 原文
	Size       int64  `json:"size"`
	SpindleRPM string `json:"spindleRpm"` // 机械盘转速（可能为空）
	SMR        string `json:"smr"`        // yes/no/unknown（型号启发式，仅提示）
}

type DiskProfiles struct {
	Volumes  []DiskInfo         `json:"volumes"`
	Physical []PhysicalDiskInfo `json:"physical"`
	IOMode   string             `json:"ioMode"`
	Warning  string             `json:"warning,omitempty"`
}

// GetDiskProfiles 返回逻辑盘 + 物理盘健康摘要 + 当前 I/O 模式。
func (a *App) GetDiskProfiles() DiskProfiles {
	out := DiskProfiles{Volumes: a.GetDiskList(), IOMode: currentDiskIOMode()}

	rows, err := wmiQuery(`root\Microsoft\Windows\Storage`,
		"SELECT DeviceId,FriendlyName,MediaType,BusType,HealthStatus,OperationalStatus,Size,SpindleSpeed FROM MSFT_PhysicalDisk",
		[]string{"DeviceId", "FriendlyName", "MediaType", "BusType", "HealthStatus", "OperationalStatus", "Size", "SpindleSpeed"})
	if err == nil && len(rows) > 0 {
		for _, r := range rows {
			idx, _ := strconv.Atoi(strings.TrimSpace(r["DeviceId"]))
			size, _ := strconv.ParseInt(strings.TrimSpace(r["Size"]), 10, 64)
			media := mediaTypeName(r["MediaType"])
			rpm := strings.TrimSpace(r["SpindleSpeed"])
			if media != "HDD" || rpm == "0" || rpm == "4294967295" {
				rpm = ""
			}
			out.Physical = append(out.Physical, PhysicalDiskInfo{
				Index:      idx,
				Name:       strings.TrimSpace(r["FriendlyName"]),
				Media:      media,
				Bus:        busTypeName(r["BusType"]),
				Health:     healthStatusName(r["HealthStatus"]),
				Status:     strings.TrimSpace(r["OperationalStatus"]),
				Size:       size,
				SpindleRPM: rpm,
				SMR:        smrHeuristic(r["FriendlyName"], media),
			})
		}
		return out
	}

	// 回退：Win32_DiskDrive（信息更少，但兼容性更好）
	out.Warning = "Storage 命名空间不可用，已回退到 Win32_DiskDrive（健康信息可能不完整）"
	if rows, err := wmiQuery(`root\cimv2`,
		"SELECT Index,Model,InterfaceType,MediaType,Status,Size FROM Win32_DiskDrive",
		[]string{"Index", "Model", "InterfaceType", "MediaType", "Status", "Size"}); err == nil {
		for _, r := range rows {
			idx, _ := strconv.Atoi(strings.TrimSpace(r["Index"]))
			size, _ := strconv.ParseInt(strings.TrimSpace(r["Size"]), 10, 64)
			mt := strings.TrimSpace(r["MediaType"])
			media := "Unknown"
			if strings.Contains(strings.ToLower(mt), "fixed") || strings.Contains(strings.ToLower(mt), "hard") {
				media = "HDD/SSD"
			}
			out.Physical = append(out.Physical, PhysicalDiskInfo{
				Index:  idx,
				Name:   strings.TrimSpace(r["Model"]),
				Media:  media,
				Bus:    strings.TrimSpace(r["InterfaceType"]),
				Health: "Unknown",
				Status: strings.TrimSpace(r["Status"]),
				Size:   size,
				SMR:    "unknown",
			})
		}
	}
	return out
}

func mediaTypeName(v string) string {
	switch strings.TrimSpace(v) {
	case "3":
		return "HDD"
	case "4":
		return "SSD"
	case "5":
		return "SCM"
	default:
		return "Unknown"
	}
}

func busTypeName(v string) string {
	n, _ := strconv.Atoi(strings.TrimSpace(v))
	switch n {
	case 3:
		return "ATA"
	case 7:
		return "USB"
	case 8:
		return "RAID"
	case 10:
		return "SAS"
	case 11:
		return "SATA"
	case 14:
		return "Virtual"
	case 17:
		return "NVMe"
	case 18:
		return "SCM"
	default:
		return "Unknown"
	}
}

func healthStatusName(v string) string {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return "Unknown"
	}
	switch n {
	case 0:
		return "Healthy"
	case 1:
		return "Warning"
	case 2:
		return "Unhealthy"
	default:
		return "Unknown"
	}
}

// smrHeuristic 只是提示，不是结论：SMR 无法通过通用 WMI 字段可靠识别，
// 这里只覆盖几个常见的消费级 SMR 型号关键词，其余返回 unknown。
func smrHeuristic(name, media string) string {
	if media != "HDD" {
		return "no"
	}
	l := strings.ToLower(name)
	for _, k := range []string{"barracuda 2.5", "st2000dm008", "st4000dm004", "st8000dm004", "wd20spzx", "wd40ezaz"} {
		if strings.Contains(l, k) {
			return "likely"
		}
	}
	return "unknown"
}
