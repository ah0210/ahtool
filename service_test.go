package main

import (
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

// toolExists 该 ID 是否真实存在于工具库注册表。
func toolExists(id string) bool {
	if id == "" {
		return false
	}
	if _, ok := toolImpls[id]; ok {
		return true
	}
	buildCatalog()
	for _, it := range toolCatalog {
		if it.ID == id {
			return true
		}
	}
	return false
}

var knownPages = map[string]bool{
	"home": true, "clean": true, "optimize": true, "space": true, "sec": true,
	"driver": true, "bsod": true, "tools": true, "appx": true, "svc": true,
	"set": true, "doc": true,
}

// 自检清单：结构完整、引用的工具/页面必须真实存在。
func TestServiceGuidesIntegrity(t *testing.T) {
	gs := serviceGuides()
	if len(gs) < 8 {
		t.Fatalf("自检清单至少 8 条，实际 %d", len(gs))
	}
	seen := map[string]bool{}
	for _, g := range gs {
		if g.ID == "" || seen[g.ID] {
			t.Errorf("清单 ID 为空或重复：%q", g.ID)
		}
		seen[g.ID] = true
		if g.Title == "" || len(g.Steps) < 2 {
			t.Errorf("%s 标题为空或步骤少于 2 步", g.ID)
		}
		if g.ToolID != "" && !toolExists(g.ToolID) {
			t.Errorf("%s 引用的工具 %q 不存在", g.ID, g.ToolID)
		}
		if g.PageID != "" && !knownPages[g.PageID] {
			t.Errorf("%s 引用的页面 %q 不在已知页面集合里", g.ID, g.PageID)
		}
		if (g.ToolID != "" || g.PageID != "") && g.ActLabel == "" {
			t.Errorf("%s 有动作但没写按钮文案", g.ID)
		}
	}
}

// 诊断报告：能生成、含全部小节、带用户自述、不出现外部地址。
func TestBuildDiagReport(t *testing.T) {
	note := "一开机就蓝屏，代码 0x000000D1"
	rep := (&App{}).BuildDiagReport(note)

	if len(rep) < 300 {
		t.Fatalf("报告太短（%d 字节），可能采集全失败：\n%s", len(rep), rep)
	}
	for _, sec := range []string{"【ah系统优化工具箱 · 本机诊断报告】", "── 系统 ──", "── 硬件 ──", "── 磁盘 ──", "── 故障线索 ──", "── 用户自述 ──"} {
		if !strings.Contains(rep, sec) {
			t.Errorf("报告缺少小节：%q", sec)
		}
	}
	if !strings.Contains(rep, note) {
		t.Error("报告没有带上用户自述内容")
	}
	if strings.ContainsAny(rep, "\x00") || strings.Contains(rep, "�") {
		t.Error("报告里出现了空字节或替换字符（编码问题）")
	}
	if strings.Contains(rep, "%!") {
		t.Error("报告里有格式化失败残留（%!…），说明 fmt 参数对不上")
	}
	if strings.Contains(rep, "http://") || strings.Contains(rep, "https://") {
		t.Error("本分支诊断报告不应包含任何外部网址")
	}
	rep2 := (&App{}).BuildDiagReport("   ")
	if strings.Contains(rep2, "── 用户自述 ──") {
		t.Error("没有自述时不应输出「用户自述」小节")
	}
	t.Log("REPORT_BEGIN|" + rep + "|REPORT_END")
}

// ---------- 剪贴板 ----------

var procGetClipboardData = svcU32.NewProc("GetClipboardData")

func clipboardRead(t *testing.T) string {
	t.Helper()
	ok := false
	for i := 0; i < 8; i++ {
		if r, _, _ := procOpenClipboard.Call(0); r != 0 {
			ok = true
			break
		}
	}
	if !ok {
		t.Skip("打不开剪贴板（无交互桌面 / 被占用），跳过")
	}
	defer procCloseClipboard.Call()
	h, _, _ := procGetClipboardData.Call(cfUnicodeText)
	if h == 0 {
		return ""
	}
	ptr, _, _ := procGlobalLock.Call(h)
	if ptr == 0 {
		return ""
	}
	defer procGlobalUnlock.Call(h)
	p := *(*unsafe.Pointer)(unsafe.Pointer(&ptr))
	return syscall.UTF16ToString(unsafe.Slice((*uint16)(p), 4096))
}

func TestCopyTextRoundTrip(t *testing.T) {
	want := "ahtool 剪贴板自检 —— 中文与符号 0x000000D1 OK"
	if err := (&App{}).CopyText(want); err != "" {
		t.Skipf("写入剪贴板失败（无交互桌面 / 被占用）：%s", err)
	}
	if got := clipboardRead(t); got == "" {
		t.Skip("剪贴板读回为空（无交互桌面），跳过")
	} else if got != want {
		t.Errorf("剪贴板读回不一致：\n want=%q\n got =%q", want, got)
	}
	if err := (&App{}).CopyText(""); err == "" {
		t.Error("空内容应该返回错误提示")
	}
}

// 关于页：版本来自构建注入，许可固定，不包含任何联系方式。
func TestAboutInfo(t *testing.T) {
	ab := (&App{}).GetAboutInfo()
	if !strings.HasPrefix(ab.Version, "v") {
		t.Errorf("版本号 %q 应以 v 开头", ab.Version)
	}
	if ab.License != "GPL-3.0" {
		t.Errorf("许可协议应为 GPL-3.0，实际 %q", ab.License)
	}
	if strings.Contains(ab.SelfHash, " ") {
		t.Errorf("SHA256 不应包含空格：%q", ab.SelfHash)
	}
}

// 解除原作者服务端绑定：链接表必须为空，不含任何外部地址。
func TestNoExternalServiceBindings(t *testing.T) {
	if l := (&App{}).SvcLinks(); len(l) != 0 {
		t.Errorf("SvcLinks 应返回空表，实际 %+v", l)
	}
	for _, g := range serviceGuides() {
		for _, s := range g.Steps {
			if strings.Contains(s, "http://") || strings.Contains(s, "https://") {
				t.Errorf("自检清单不应包含外部链接：%q", s)
			}
		}
	}
}

// 默认配置下，在线更新与官方指纹校验必须处于停用状态。
func TestOnlineUpdateDisabledByDefault(t *testing.T) {
	if updatesEnabled() {
		t.Fatal("默认配置不应启用在线更新")
	}
	a := NewApp()
	if r := a.CheckUpdate(); r.State != "disabled" {
		t.Errorf("CheckUpdate 在停用时应返回 disabled，实际 %q", r.State)
	}
	if r := a.VerifySelf(); r.State != "disabled" {
		t.Errorf("VerifySelf 在停用时应返回 disabled，实际 %q", r.State)
	}
	if r := a.DownloadUpdate("https://example.invalid/x.exe", strings.Repeat("a", 64)); r.State == "ok" {
		t.Error("停用后不应允许下载更新包")
	}
}
