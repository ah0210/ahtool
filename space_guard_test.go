package main

import (
	"os"
	"path/filepath"
	"testing"
)

// nodeByPath 是 SpaceDelete 的会话级白名单：只认本次 MFT 索引里真实存在的文件。
func TestMftNodeByPath(t *testing.T) {
	idx := &mftIndex{
		drive: "C",
		nodes: map[uint64]*spNode{
			frRoot: {name: "C:", parent: frRoot, dir: true},
			16:     {name: "Users", parent: frRoot, dir: true},
			17:     {name: "alice", parent: 16, dir: true},
			18:     {name: "a.txt", parent: 17, size: 1},
		},
		kids: map[uint64][]uint64{
			frRoot: {16},
			16:     {17},
			17:     {18},
		},
	}

	if n, ok := idx.nodeByPath(`C:\Users\alice\a.txt`); !ok || n == nil || n.size != 1 {
		t.Fatalf("应解析到文件节点，ok=%v", ok)
	}
	if _, ok := idx.nodeByPath(`c:\users\ALICE\A.TXT`); !ok {
		t.Error("路径解析应不区分大小写")
	}
	if _, ok := idx.nodeByPath(`C:\Users\alice`); ok {
		t.Error("目录不能被当作可删除文件")
	}
	if _, ok := idx.nodeByPath(`D:\Users\alice\a.txt`); ok {
		t.Error("不同盘符的路径必须拒绝")
	}
	if _, ok := idx.nodeByPath(`C:\Users\..\Windows\a.txt`); ok {
		t.Error("含 .. 的路径必须拒绝")
	}
	if _, ok := idx.nodeByPath(`C:\Users\alice\missing.txt`); ok {
		t.Error("索引里不存在的路径必须拒绝")
	}
}

// 卸载命令解析：不能再回落到 cmd.exe /c <注册表原串>。
func TestBuildUninstallCmdRejectsRawShell(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "uninst.exe")
	if err := os.WriteFile(exe, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, _, ok := buildUninstallCmd(`"` + exe + `" /S`); !ok {
		t.Error("存在的绝对路径应被接受")
	}
	if _, _, _, ok := buildUninstallCmd(`definitely-not-a-real-uninstaller.exe /S`); ok {
		t.Error("裸命令名不应走 PATH 查找")
	}
	if _, _, _, ok := buildUninstallCmd(`whatever & calc.exe`); ok {
		t.Error("无法解析的字符串不能交给 cmd /c 执行")
	}
	if _, _, _, ok := buildUninstallCmd(`MsiExec.exe /X{11111111-2222-3333-4444-555555555555}`); !ok {
		t.Error("带 ProductCode 的 MSI 卸载命令应被接受")
	}
	if _, _, _, ok := buildUninstallCmd(`MsiExec.exe /X not-a-guid`); ok {
		t.Error("没有 ProductCode 的 MSI 命令应拒绝")
	}
}
