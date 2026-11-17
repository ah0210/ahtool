package main

// 磁盘 I/O 模式与并发策略（Phase 1）。
//
// 目标：把“保护”和“性能”从二选一变成按场景切换：
//   quiet    静音/保护：机械盘优先，低并发、低突发
//   balanced 均衡（默认）：SSD 能提速，HDD 不过载
//   fast     极速：SSD/NVMe 压满队列，接受更高噪音/功耗
//
// 注意：并发只影响“扫描/删除”的线程数，不改变任何清理范围或安全裁决。

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const (
	DiskIOQuiet    = "quiet"
	DiskIOBalanced = "balanced"
	DiskIOFast     = "fast"
)

var (
	diskIOMu       sync.RWMutex
	diskIOMode     string
	diskIOModePath string // 单测用；空则用 %LOCALAPPDATA%\ahtool\diskio.txt
)

func diskIOModeFilePath() string {
	if diskIOModePath != "" {
		return diskIOModePath
	}
	if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
		return filepath.Join(local, "ahtool", "diskio.txt")
	}
	return filepath.Join(os.TempDir(), "ahtool_diskio.txt")
}

func loadDiskIOMode() string {
	b, err := os.ReadFile(diskIOModeFilePath())
	if err != nil {
		return DiskIOBalanced
	}
	switch string(strings.TrimSpace(string(b))) {
	case DiskIOQuiet:
		return DiskIOQuiet
	case DiskIOFast:
		return DiskIOFast
	default:
		return DiskIOBalanced
	}
}

// currentDiskIOMode 返回当前模式；首次访问时从本地设置读取。
func currentDiskIOMode() string {
	diskIOMu.RLock()
	m := diskIOMode
	diskIOMu.RUnlock()
	if m != "" {
		return m
	}
	diskIOMu.Lock()
	defer diskIOMu.Unlock()
	if diskIOMode == "" {
		diskIOMode = loadDiskIOMode()
	}
	return diskIOMode
}

// GetDiskIOMode 供前端读取当前模式。
func (a *App) GetDiskIOMode() string { return currentDiskIOMode() }

// SetDiskIOMode 切换模式；成功返回空串，失败返回中文原因。
func (a *App) SetDiskIOMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case DiskIOQuiet, DiskIOBalanced, DiskIOFast:
	default:
		return "未知的磁盘模式：" + mode
	}
	diskIOMu.Lock()
	diskIOMode = mode
	diskIOMu.Unlock()
	// 落盘失败不阻断本次切换，但要告诉前端（下次启动会回到均衡）。
	if p := diskIOModeFilePath(); p != "" {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err == nil {
			if err := os.WriteFile(p, []byte(mode), 0o644); err != nil {
				return "模式已生效，但保存失败（下次启动会恢复均衡）：" + err.Error()
			}
		}
	}
	return ""
}

func scanWorkersForMode(mode string) int {
	n := runtime.NumCPU()
	switch mode {
	case DiskIOQuiet:
		return 2
	case DiskIOFast:
		if n < 4 {
			n = 4
		}
		if n > 16 {
			n = 16
		}
		return n * 2
	default:
		if n < 4 {
			n = 4
		}
		if n > 8 {
			n = 8
		}
		return n
	}
}

func scanItemWorkersForMode(mode string) int {
	n := runtime.NumCPU()
	switch mode {
	case DiskIOQuiet:
		return 2
	case DiskIOFast:
		if n < 4 {
			n = 4
		}
		if n > 12 {
			n = 12
		}
		return n
	default:
		if n > 6 {
			n = 6
		}
		if n < 2 {
			n = 2
		}
		return n
	}
}

func deleteWorkersForMode(mode string) int {
	switch mode {
	case DiskIOQuiet:
		return 2
	case DiskIOFast:
		return 12
	default:
		return 8
	}
}

// 给现有扫描/清理链使用的快捷函数。
func scanWorkers() int     { return scanWorkersForMode(currentDiskIOMode()) }
func scanItemWorkers() int { return scanItemWorkersForMode(currentDiskIOMode()) }
func deleteWorkers() int   { return deleteWorkersForMode(currentDiskIOMode()) }
