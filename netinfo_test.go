package main

import "testing"

func TestParseDNSDisplay(t *testing.T) {
	txt := `
Windows IP Configuration

    Record Name . . . . . : example.com
    A (Host) Record . . . : 93.184.216.34
    Record Name . . . . . : www.example.com
    CNAME Record . . . . . : example.com

    记录名称 . . . . . . . : test.local
    A (主机) 记录 . . . . : 10.0.0.5
`
	got := parseDNSDisplay(txt)
	if len(got) != 3 {
		t.Fatalf("解析到 %d 条，want 3：%+v", len(got), got)
	}
	want := map[string]bool{
		"example.com|A|93.184.216.34":       true,
		"www.example.com|CNAME|example.com": true,
		"test.local|A|10.0.0.5":             true,
	}
	for _, e := range got {
		if !want[e.key] {
			t.Errorf("多余/错误条目：%+v", e)
		}
	}
}

func TestSplitList(t *testing.T) {
	got := splitList("8.8.8.8, 1.1.1.1 ,, 2001:db8::1")
	if len(got) != 3 || got[0] != "8.8.8.8" || got[2] != "2001:db8::1" {
		t.Fatalf("splitList = %+v", got)
	}
}
