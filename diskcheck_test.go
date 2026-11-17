package main

import "testing"

func TestNormalizeDriveLetter(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"C:", "C:", true},
		{"c:", "C:", true},
		{"D", "D:", true},
		{`E:\`, "E:", true},
		{"", "", false},
		{"CC", "", false},
		{"1:", "", false},
		{"C:\\x", "C:", true}, // 只取盘符，后面的路径忽略
	}
	for _, c := range cases {
		got, ok := normalizeDriveLetter(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("normalizeDriveLetter(%q) = (%q,%v), want (%q,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestDiskMediaName(t *testing.T) {
	if diskMediaName(true) != "SSD" || diskMediaName(false) != "HDD" {
		t.Fatal("disk media label wrong")
	}
}
