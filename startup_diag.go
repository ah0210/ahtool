package main

// 启动诊断：GUI 子系统没有控制台，wails.Run 的错误、panic、WebView2 缺失
// 都会表现为“双击没反应”。这里统一落盘到 %LOCALAPPDATA%\ahtool\startup.log，
// 并在致命错误时弹一个 MessageBox；另提供 `-diag` 参数输出环境报告。

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/wailsapp/go-webview2/webviewloader"
	"golang.org/x/sys/windows"
)

const appDisplayName = "ah系统优化工具箱"

var (
	user32DLL       = syscall.NewLazyDLL("user32.dll")
	procMessageBoxW = user32DLL.NewProc("MessageBoxW")
)

func startupLogPath() string {
	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "ahtool")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return filepath.Join(os.TempDir(), "ahtool_startup.log")
	}
	return filepath.Join(dir, "startup.log")
}

// startupLog 追加一行启动日志；失败时退到 %TEMP%，再失败就静默。
func startupLog(format string, args ...interface{}) {
	line := time.Now().Format("2006-01-02 15:04:05") + " " + fmt.Sprintf(format, args...)
	p := startupLogPath()
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		f, err = os.OpenFile(filepath.Join(os.TempDir(), "ahtool_startup.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return
		}
	}
	defer f.Close()
	_, _ = f.WriteString(line + "\n")
}

func showMessage(title, text string, icon uintptr) {
	t, err1 := windows.UTF16PtrFromString(title)
	m, err2 := windows.UTF16PtrFromString(text)
	if err1 != nil || err2 != nil {
		return
	}
	const mbOK = 0x00000000
	const mbTopmost = 0x00040000
	_, _, _ = procMessageBoxW.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), mbOK|mbTopmost|icon)
}

func showFatal(msg string) {
	const mbIconError = 0x00000010
	showMessage(appDisplayName+" 启动失败", msg, mbIconError)
}

// installCrashHandler 捕获启动/运行期的 panic，避免“无声退出”。
func installCrashHandler() {
	if r := recover(); r != nil {
		stack := string(debug.Stack())
		startupLog("PANIC: %v\n%s", r, stack)
		showFatal(fmt.Sprintf("程序发生未处理错误：\n%v\n\n日志：%s", r, startupLogPath()))
	}
}

// checkWebView2 返回 WebView2 Runtime 版本；空串表示未安装/读不到。
func checkWebView2() (string, error) {
	v, err := webviewloader.GetAvailableCoreWebView2BrowserVersionString("")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(v), nil
}

// runStartupDiag 输出启动环境报告，写文件 + 弹窗，便于用户反馈。
func runStartupDiag() {
	startupLog("=== runStartupDiag ===")
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()
	wv, wvErr := checkWebView2()
	admin := "否"
	if isElevated() {
		admin = "是"
	}
	lines := []string{
		"版本: " + appVersion + "  构建: " + buildDate,
		"exe: " + exe,
		"工作目录: " + cwd,
		"系统: " + os.Getenv("OS") + "  " + runtime.GOOS + "/" + runtime.GOARCH,
		"管理员: " + admin,
		"WebView2: " + emptyDash(wv),
	}
	if wvErr != nil {
		lines = append(lines, "WebView2 读取错误: "+wvErr.Error())
	}
	if wv == "" {
		lines = append(lines, "提示: 未检测到 WebView2 Runtime。Win11 通常自带；LTSC/精简系统可能需要安装。")
	}
	lines = append(lines,
		"LOCALAPPDATA: "+os.Getenv("LOCALAPPDATA"),
		"TEMP: "+os.Getenv("TEMP"),
		"日志: "+startupLogPath(),
	)
	report := strings.Join(lines, "\n")
	startupLog("diag:\n%s", report)
	_ = os.WriteFile(filepath.Join(os.TempDir(), "ahtool_diag.txt"), []byte(report+"\n"), 0o644)

	const mbIconInfo = 0x00000040
	showMessage(appDisplayName+" 启动诊断", report+"\n\n报告已写入 %TEMP%\\ahtool_diag.txt", mbIconInfo)
}
