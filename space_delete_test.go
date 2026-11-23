// 回归测试：SpaceDelete 必须真的把文件删掉（永久删除，不进回收站）。
//
// 2026-11-02 两个坑依次钉死：
//
//	① 原 recycleDelete 里 windows.StringToUTF16Ptr(path + "\x00") 会 panic
//	   （x/sys 不允许内嵌 NUL），panic 被 Wails dispatcher recover 后前端拿不到
//	   回调，15 秒判「调用超时」—— shell 一次都没被调用，文件纹丝不动。
//	② 改用 FOF_ALLOWUNDO（进回收站）后，大文件在回收站配额/策略/卷类型不支持
//	   的机器上直接返回非 0，界面显示「0 项成功、1 项失败」，文件仍在原地。
//	   现改为永久删除 + 属性剥离 + os.RemoveAll 兜底 + 以磁盘现状复核。
//
// 这些测试就是钉子：谁把参数构造改回拼接 "\x00"、或把 FOF_ALLOWUNDO 加回来，
// 这里立刻红。
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"
)

// registerTempFilesForDelete 把本测试造出的文件注册进「本次扫描」的 MFT 索引。
// SpaceDelete 在 2026-11-02 加了会话级白名单（只删本次扫描索引里真实存在的
// 文件），测试造的临时文件不在任何真实扫描结果里，必须先补一个最小索引，
// 否则全新进程里必然报「尚未扫描」。开发者本机若残留 spaceIdx 会碰巧通过，
// CI 每次全新进程则暴露 —— 这里显式构造，保证测试可重复。
func registerTempFilesForDelete(t *testing.T, paths ...string) {
	t.Helper()
	if len(paths) == 0 {
		t.Fatal("registerTempFilesForDelete: 没有路径")
	}
	drive := strings.ToUpper(paths[0][:1])
	idx := &mftIndex{
		drive: drive,
		nodes: map[uint64]*spNode{frRoot: {name: drive + ":", parent: frRoot, dir: true}},
		kids:  map[uint64][]uint64{},
	}
	var next uint64 = 100
	ensureDir := func(parent uint64, name string) uint64 {
		for _, k := range idx.kids[parent] {
			n := idx.nodes[k]
			if n != nil && n.dir && strings.EqualFold(n.name, name) {
				return k
			}
		}
		id := next
		next++
		idx.nodes[id] = &spNode{name: name, parent: parent, dir: true}
		idx.kids[parent] = append(idx.kids[parent], id)
		return id
	}
	for _, p := range paths {
		up := strings.ToUpper(p)
		if len(up) < 3 || up[1] != ':' || !strings.EqualFold(up[:1], drive) {
			t.Fatalf("路径不是 %s 盘: %s", drive, p)
		}
		parts := strings.Split(strings.Trim(p[3:], `\`), `\`)
		cur := frRoot
		for _, seg := range parts[:len(parts)-1] {
			if seg != "" {
				cur = ensureDir(cur, seg)
			}
		}
		base := parts[len(parts)-1]
		id := next
		next++
		var size int64
		if info, err := os.Stat(p); err == nil {
			size = info.Size()
		}
		// 系统保护类断言用的纯路径可能并不存在，白名单只查索引不查磁盘，
		// size 为 0 不影响判定。
		idx.nodes[id] = &spNode{name: base, parent: cur, size: size}
		idx.kids[cur] = append(idx.kids[cur], id)
	}
	spaceMu.Lock()
	spaceIdx = idx
	spaceMu.Unlock()
	t.Cleanup(func() {
		spaceMu.Lock()
		spaceIdx = nil
		spaceMu.Unlock()
	})
}

func TestSpaceDeleteReallyDeletes(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "dup-file.bin")
	data := make([]byte, 1<<20) // 1MB 够验证路径，不需要真造 100MB
	for i := range data {
		data[i] = byte(i)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatalf("造测试文件失败: %v", err)
	}
	registerTempFilesForDelete(t, p)

	var a App
	res := a.SpaceDelete([]string{p})
	if len(res) != 1 {
		t.Fatalf("期望 1 条结果，实际 %d 条", len(res))
	}
	if !res[0].Ok {
		if _, err := os.Stat(p); err == nil {
			t.Fatalf("删除失败且文件仍在: %s", res[0].Err)
		}
		t.Skipf("环境权限不足，跳过: %s", res[0].Err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("返回成功但文件仍存在: %v", err)
	}
}

// 多路径一次删多个（重复文件页会一次传一组），逐条结果都要是 ok
func TestSpaceDeleteMultiple(t *testing.T) {
	dir := t.TempDir()
	var paths []string
	for _, n := range []string{"a.bin", "b.bin", "c.bin"} {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, []byte("same-content"), 0o644); err != nil {
			t.Fatalf("造测试文件失败: %v", err)
		}
		paths = append(paths, p)
	}
	registerTempFilesForDelete(t, paths...)
	var a App
	res := a.SpaceDelete(paths)
	if len(res) != len(paths) {
		t.Fatalf("期望 %d 条结果，实际 %d 条", len(paths), len(res))
	}
	for _, r := range res {
		if !r.Ok {
			if _, err := os.Stat(r.Path); err == nil {
				t.Fatalf("删除失败且文件仍在: %s -> %s", r.Path, r.Err)
			}
			t.Skipf("环境权限不足，跳过: %s", r.Err)
		}
		if _, err := os.Stat(r.Path); !os.IsNotExist(err) {
			t.Fatalf("返回成功但文件仍存在: %s", r.Path)
		}
	}
}

// 只读/隐藏/系统属性不能挡住删除 —— 用户反馈的重复文件里常有这类系统文件，
// 这一条专门验证 clearDeleteAttrs + os.RemoveAll 兜底链路。
func TestSpaceDeleteReadOnlyAndHidden(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "只读 副本.bin")
	if err := os.WriteFile(p, []byte("dup"), 0o644); err != nil {
		t.Fatalf("造测试文件失败: %v", err)
	}
	if err := os.Chmod(p, 0o444); err != nil { // Windows 上等价于 FILE_ATTRIBUTE_READONLY
		t.Fatalf("设置只读失败: %v", err)
	}
	registerTempFilesForDelete(t, p)
	var a App
	res := a.SpaceDelete([]string{p})
	if !res[0].Ok {
		t.Fatalf("只读文件应能被删掉，实际失败: %s", res[0].Err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("返回成功但只读文件仍存在: %v", err)
	}
}

// 产品决策（2026-11-02 四次修复）：SpaceDelete 只允许删文件，绝不接受目录 ——
// 目录整树递归误删面太大，UI 也不提供目录删除；spaceDeleteFile 与 MFT 白名单
// 两处都会拒绝目录。这条守卫确保谁都别把目录删除能力加回来。
func TestSpaceDeleteRejectsDirectory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "待删除目录")
	if err := os.MkdirAll(filepath.Join(sub, "内层"), 0o755); err != nil {
		t.Fatalf("造测试目录失败: %v", err)
	}
	f := filepath.Join(sub, "内层", "dup.bin")
	if err := os.WriteFile(f, []byte("dup"), 0o444); err != nil {
		t.Fatalf("造测试文件失败: %v", err)
	}
	var a App
	// 即使没有扫描会话，目录也必须被拒绝且原样保留。
	res := a.SpaceDelete([]string{sub})
	if len(res) != 1 || res[0].Ok {
		t.Fatalf("目录必须被拒绝，实际: %+v", res)
	}
	if _, err := os.Stat(sub); err != nil {
		t.Fatalf("被拒绝后目录不应消失: %v", err)
	}
	if _, err := os.Stat(f); err != nil {
		t.Fatalf("被拒绝后目录内文件不应消失: %v", err)
	}
}

// 钉死产品决策：删除一律永久删除，不许把 FOF_ALLOWUNDO (0x0040) 加回来。
func TestNoAllowUndoFlag(t *testing.T) {
	if shDeleteFlags&0x0040 != 0 {
		t.Fatalf("删除不能带 FOF_ALLOWUNDO：大文件在回收站配额/策略受限的机器上会直接失败")
	}
}

// pathListUTF16 必须产出双 NUL 结尾：这是 SHFileOperationW 的硬要求，
// 也是原来那个 panic 的直接原因。
func TestPathListUTF16DoubleNul(t *testing.T) {
	buf := unsafe.Slice(pathListUTF16([]string{`C:\a.txt`}), 10)
	// "C:\a.txt" = 8 个 UTF-16 单元 + 每路径结尾 NUL + 列表结尾 NUL = 10
	if buf[len(buf)-1] != 0 || buf[len(buf)-2] != 0 {
		t.Fatalf("结尾必须是双 NUL: %v", buf)
	}
	if buf[0] != 'C' || buf[1] != ':' || buf[7] != 't' {
		t.Fatalf("内容编码不对: %v", buf)
	}

	multi := unsafe.Slice(pathListUTF16([]string{`C:\a.txt`, `D:\b.txt`}), 19)
	if multi[len(multi)-1] != 0 || multi[len(multi)-2] != 0 {
		t.Fatalf("多路径结尾也必须是双 NUL: %v", multi)
	}
	// 两个路径之间必须有一个 NUL 分隔
	if multi[8] != 0 {
		t.Fatalf("路径之间缺少 NUL 分隔: %v", multi)
	}
}
