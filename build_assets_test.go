package main

import "testing"

// Wails v2 的 assetserver 要求 embed 资产里必须有 index.html；缺了 wails.Run 会直接
// 返回错误，而 GUI 子系统没有控制台，表现为“双击无反应”。这条测试防止再次误删。
func TestEmbeddedAssetsHaveIndexHTML(t *testing.T) {
	f, err := assets.Open("frontend/dist/index.html")
	if err != nil {
		t.Fatalf("frontend/dist/index.html 缺失：%v（Wails 启动会失败）", err)
	}
	_ = f.Close()
}
