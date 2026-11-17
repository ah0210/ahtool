package main

import "testing"

func TestCrashLevelName(t *testing.T) {
	if crashLevelName(1) != "bad" || crashLevelName(3) != "warn" || crashLevelName(4) != "info" {
		t.Fatal("事件级别映射错误")
	}
}

func TestCrashCode(t *testing.T) {
	m := map[string]string{
		"BugcheckCode":       "209",
		"BugcheckParameter1": "0x1",
		"BugcheckParameter2": "0x2",
		"BugcheckParameter3": "0x3",
		"BugcheckParameter4": "0x4",
	}
	code, hex, params := crashCode(m)
	if code != 209 || hex != "0x000000D1" || len(params) != 4 {
		t.Fatalf("crashCode = %d %q %v", code, hex, params)
	}
}
