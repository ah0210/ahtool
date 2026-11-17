package main

import "testing"

func TestSplitHostPort(t *testing.T) {
	cases := []struct{ in, h, p string }{
		{"127.0.0.1:135", "127.0.0.1", "135"},
		{"[::]:445", "::", "445"},
		{"0.0.0.0:80", "0.0.0.0", "80"},
		{"*:*", "*", "*"},
	}
	for _, c := range cases {
		h, p := splitHostPort(c.in)
		if h != c.h || p != c.p {
			t.Errorf("splitHostPort(%q) = (%q,%q), want (%q,%q)", c.in, h, p, c.h, c.p)
		}
	}
}

func TestPortExposure(t *testing.T) {
	if s, e := portExposure("127.0.0.1:135"); s != "loopback" || e {
		t.Errorf("loopback = %q/%v", s, e)
	}
	if s, e := portExposure("0.0.0.0:445"); s != "any" || !e {
		t.Errorf("any = %q/%v", s, e)
	}
	if s, e := portExposure("[::]:80"); s != "any" || !e {
		t.Errorf("ipv6 any = %q/%v", s, e)
	}
	if s, e := portExposure("192.168.1.10:80"); s != "lan" || !e {
		t.Errorf("lan = %q/%v", s, e)
	}
}

func TestConnScopeAndPrivate(t *testing.T) {
	if !isPrivateIP("192.168.1.1") || !isPrivateIP("10.0.0.1") || !isPrivateIP("fe80::1") {
		t.Error("私有地址判断错误")
	}
	if isPrivateIP("8.8.8.8") {
		t.Error("8.8.8.8 不应判为私有")
	}
	if got := connScope("0.0.0.0:80", "0.0.0.0:0", "LISTENING"); got != "listen" {
		t.Errorf("监听 scope = %q", got)
	}
	if got := connScope("192.168.1.2:5555", "8.8.8.8:443", "ESTABLISHED"); got != "external" {
		t.Errorf("对外 scope = %q", got)
	}
}
