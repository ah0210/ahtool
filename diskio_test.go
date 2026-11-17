package main

import (
	"path/filepath"
	"testing"
)

func TestIOWorkersByMode(t *testing.T) {
	if got := scanWorkersForMode(DiskIOQuiet); got != 2 {
		t.Fatalf("quiet scan workers = %d, want 2", got)
	}
	if got := scanItemWorkersForMode(DiskIOQuiet); got != 2 {
		t.Fatalf("quiet item workers = %d, want 2", got)
	}
	if got := deleteWorkersForMode(DiskIOQuiet); got != 2 {
		t.Fatalf("quiet delete workers = %d, want 2", got)
	}
	if got := deleteWorkersForMode(DiskIOBalanced); got != 8 {
		t.Fatalf("balanced delete workers = %d, want 8", got)
	}
	if got := deleteWorkersForMode(DiskIOFast); got != 12 {
		t.Fatalf("fast delete workers = %d, want 12", got)
	}
}

func TestSetDiskIOMode(t *testing.T) {
	oldPath := diskIOModePath
	oldMode := diskIOMode
	diskIOModePath = filepath.Join(t.TempDir(), "diskio.txt")
	diskIOMode = ""
	defer func() { diskIOModePath = oldPath; diskIOMode = oldMode }()

	a := &App{}
	if msg := a.SetDiskIOMode("garbage"); msg == "" {
		t.Fatal("非法模式应返回错误")
	}
	if msg := a.SetDiskIOMode(DiskIOQuiet); msg != "" {
		t.Fatalf("合法模式应成功，实际 %q", msg)
	}
	if got := a.GetDiskIOMode(); got != DiskIOQuiet {
		t.Fatalf("读取模式 = %q, want quiet", got)
	}
	if got := scanWorkersForMode(a.GetDiskIOMode()); got != 2 {
		t.Fatalf("quiet 模式扫描并发 = %d, want 2", got)
	}
}
