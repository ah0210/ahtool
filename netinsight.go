package main

// 网络洞察（只读）：网卡/路由/连接/端口/邻居/共享/分段连通性。
//
// MVP 数据源：
//   - 网卡/IP/网关/DNS/代理：复用 GetNetworkInfo
//   - 连接/端口：netstat -ano + tasklist（PID→进程名/路径）
//   - 路由：route print -4
//   - 邻居：arp -a（被动表，不主动扫描）
//   - 共享/发现：net share + sc query
// 所有内容只在本机展示，不上传；探测仅在用户点击时执行。

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

type NetConn struct {
	Proto   string `json:"proto"`
	Local   string `json:"local"`
	Remote  string `json:"remote"`
	State   string `json:"state"`
	PID     int    `json:"pid"`
	Process string `json:"process"`
	Path    string `json:"path,omitempty"`
	Scope   string `json:"scope"` // external / lan / local / listen
}

type NetPort struct {
	Proto   string `json:"proto"`
	Addr    string `json:"addr"`
	Port    string `json:"port"`
	PID     int    `json:"pid"`
	Process string `json:"process"`
	Path    string `json:"path,omitempty"`
	Scope   string `json:"scope"` // loopback / lan / any
	Exposed bool   `json:"exposed"`
}

type NetArp struct {
	IP    string `json:"ip"`
	MAC   string `json:"mac"`
	Type  string `json:"type"`
	Iface string `json:"iface"`
}

type NetSegment struct {
	Name   string `json:"name"`
	Target string `json:"target"`
	Status string `json:"status"` // ok / bad / unknown
	Detail string `json:"detail"`
}

type WiFiInfo struct {
	SSID   string `json:"ssid"`
	BSSID  string `json:"bssid"`
	State  string `json:"state"`
	Signal string `json:"signal"`
	Radio  string `json:"radio"`
	RxRate string `json:"rxRate"`
	TxRate string `json:"txRate"`
}

type NetInsight struct {
	ReadAt      string             `json:"readAt"`
	Hostname    string             `json:"hostname"`
	Adapters    []NetInterfaceInfo `json:"adapters"`
	Routes      []string           `json:"routes"`
	Proxy       ProxyInfo          `json:"proxy"`
	WiFi        []WiFiInfo         `json:"wifi"`
	Connections []NetConn          `json:"connections"`
	Ports       []NetPort          `json:"ports"`
	Arp         []NetArp           `json:"arp"`
	Shares      []string           `json:"shares"`
	Discovery   []string           `json:"discovery"`
	Stats       map[string]int     `json:"stats"`
}

func splitHostPort(addr string) (string, string) {
	addr = strings.TrimSpace(addr)
	if i := strings.LastIndex(addr, ":"); i > 0 {
		return strings.Trim(addr[:i], "[]"), addr[i+1:]
	}
	return addr, ""
}

func isLoopbackHost(h string) bool {
	if h == "127.0.0.1" || h == "::1" || h == "localhost" || h == "" {
		return true
	}
	return strings.HasPrefix(h, "127.")
}

func isPrivateIP(s string) bool {
	ip := net.ParseIP(strings.Trim(s, "[]"))
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() {
		return true
	}
	return false
}

func connScope(local, remote, state string) string {
	lh, _ := splitHostPort(local)
	if strings.EqualFold(state, "LISTENING") || strings.EqualFold(state, "LISTEN") || strings.EqualFold(state, "BOUND") {
		if isLoopbackHost(lh) {
			return "local"
		}
		return "listen"
	}
	rh, _ := splitHostPort(remote)
	if rh == "" || rh == "*" || rh == "0.0.0.0" || rh == "::" {
		return "local"
	}
	if isPrivateIP(rh) {
		return "lan"
	}
	return "external"
}

func portExposure(addr string) (string, bool) {
	h, _ := splitHostPort(addr)
	switch {
	case isLoopbackHost(h):
		return "loopback", false
	case h == "0.0.0.0" || h == "::" || h == "*":
		return "any", true
	default:
		return "lan", true
	}
}

func processPath(pid int) (string, string) {
	if pid <= 0 {
		return "", ""
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return "", ""
	}
	defer windows.CloseHandle(h)
	var buf [32768]uint16
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return "", ""
	}
	p := windows.UTF16ToString(buf[:size])
	return strings.ToLower(filepath.Base(p)), p
}

func readProcessMap() map[int]string {
	out := map[int]string{}
	raw, err := runHiddenOut(20*time.Second, resolveExe("tasklist.exe"), "/FO", "CSV", "/NH")
	if err != nil {
		return out
	}
	for _, line := range strings.Split(decodeConsole([]byte(raw)), "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" {
			continue
		}
		parts := strings.Split(line, `","`)
		if len(parts) < 2 {
			continue
		}
		name := strings.Trim(parts[0], `"`)
		pid, e := strconv.Atoi(strings.Trim(parts[1], `"`))
		if e == nil && pid > 0 {
			out[pid] = name
		}
	}
	return out
}

func readNetConnectionsAndPorts() ([]NetConn, []NetPort) {
	raw, err := runHiddenOut(30*time.Second, resolveExe("netstat.exe"), "-ano")
	if err != nil {
		return nil, nil
	}
	pm := readProcessMap()
	var conns []NetConn
	var ports []NetPort
	type pinfo struct{ name, path string }
	pp := map[int]pinfo{}
	getP := func(pid int) (string, string) {
		if v, ok := pp[pid]; ok {
			return v.name, v.path
		}
		name := pm[pid]
		if name == "" {
			name = "—"
		}
		_, path := processPath(pid)
		pp[pid] = pinfo{name, path}
		return name, path
	}
	for _, line := range strings.Split(decodeConsole([]byte(raw)), "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 4 {
			continue
		}
		proto := strings.ToUpper(f[0])
		if proto != "TCP" && proto != "UDP" {
			continue
		}
		var local, remote, state, pidStr string
		if proto == "TCP" && len(f) >= 5 {
			local, remote, state, pidStr = f[1], f[2], strings.ToUpper(f[3]), f[4]
		} else {
			local, remote, state, pidStr = f[1], f[2], "BOUND", f[3]
		}
		pid, _ := strconv.Atoi(pidStr)
		scope := connScope(local, remote, state)
		name, path := getP(pid)
		conns = append(conns, NetConn{Proto: proto, Local: local, Remote: remote, State: state, PID: pid, Process: name, Path: path, Scope: scope})
		if strings.EqualFold(state, "LISTENING") || (proto == "UDP" && state == "BOUND") {
			_, port := splitHostPort(local)
			pscope, exposed := portExposure(local)
			ports = append(ports, NetPort{Proto: proto, Addr: local, Port: port, PID: pid, Process: name, Path: path, Scope: pscope, Exposed: exposed})
		}
		if len(conns) >= 800 {
			break
		}
	}
	sort.SliceStable(conns, func(i, j int) bool {
		if conns[i].Scope != conns[j].Scope {
			return conns[i].Scope > conns[j].Scope
		}
		return conns[i].Process < conns[j].Process
	})
	sort.SliceStable(ports, func(i, j int) bool {
		if ports[i].Exposed != ports[j].Exposed {
			return ports[i].Exposed
		}
		return ports[i].Port < ports[j].Port
	})
	return conns, ports
}

func readRoutes() []string {
	raw, err := runHiddenOut(20*time.Second, resolveExe("route.exe"), "print", "-4")
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(decodeConsole([]byte(raw)), "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if strings.HasPrefix(line, "0.0.0.0") {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		// 没有解析到默认路由：把 Active Routes 段前几行原样带出，便于判断
		for _, line := range strings.Split(decodeConsole([]byte(raw)), "\n") {
			line = strings.TrimSpace(strings.TrimRight(line, "\r"))
			if strings.Contains(line, "0.0.0.0") {
				out = append(out, line)
			}
			if len(out) >= 10 {
				break
			}
		}
	}
	return out
}

func readArp() []NetArp {
	raw, err := runHiddenOut(20*time.Second, resolveExe("arp.exe"), "-a")
	if err != nil {
		return nil
	}
	var out []NetArp
	iface := ""
	for _, line := range strings.Split(decodeConsole([]byte(raw)), "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "interface:") || strings.HasPrefix(line, "接口:") {
			if i := strings.LastIndex(line, ":"); i >= 0 {
				iface = strings.TrimSpace(line[i+1:])
			}
			continue
		}
		f := strings.Fields(line)
		if len(f) >= 2 && strings.Count(f[1], "-") == 5 {
			typ := ""
			if len(f) >= 3 {
				typ = strings.Join(f[2:], " ")
			}
			out = append(out, NetArp{IP: f[0], MAC: f[1], Type: typ, Iface: iface})
		}
		if len(out) >= 100 {
			break
		}
	}
	return out
}

func readShares() []string {
	raw, err := runHiddenOut(20*time.Second, resolveExe("net.exe"), "share")
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(decodeConsole([]byte(raw)), "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		low := strings.ToLower(line)
		if line == "" || strings.HasPrefix(line, "共享名") || strings.HasPrefix(low, "share name") || strings.HasPrefix(line, "---") || strings.Contains(low, "command completed") || strings.Contains(line, "命令成功完成") {
			continue
		}
		out = append(out, line)
		if len(out) >= 30 {
			break
		}
	}
	return out
}

func readDiscovery() []string {
	var out []string
	for _, svc := range []string{"FDResPub", "SSDPSRV", "upnphost", "LanmanServer", "LanmanWorkstation"} {
		raw, err := runHiddenOut(10*time.Second, resolveExe("sc.exe"), "query", svc)
		if err != nil {
			out = append(out, svc+": 未读取到")
			continue
		}
		txt := decodeConsole([]byte(raw))
		state := "Unknown"
		for _, line := range strings.Split(txt, "\n") {
			if strings.Contains(line, "STATE") || strings.Contains(line, "状态") {
				state = strings.TrimSpace(line)
				break
			}
		}
		out = append(out, svc+": "+state)
	}
	return out
}

func readWiFi() []WiFiInfo {
	raw, err := runHiddenOut(10*time.Second, resolveExe("netsh.exe"), "wlan", "show", "interfaces")
	if err != nil {
		return nil
	}
	txt := decodeConsole([]byte(raw))
	var list []WiFiInfo
	var cur WiFiInfo
	flush := func() {
		if cur.SSID != "" || cur.State != "" || cur.BSSID != "" {
			list = append(list, cur)
		}
		cur = WiFiInfo{}
	}
	for _, line := range strings.Split(txt, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" {
			continue
		}
		low := strings.ToLower(line)
		val := ""
		if i := strings.Index(line, ":"); i >= 0 {
			val = strings.TrimSpace(line[i+1:])
		}
		switch {
		case strings.HasPrefix(low, "name") || strings.HasPrefix(line, "名称"):
			flush()
		case strings.HasPrefix(low, "ssid") && !strings.HasPrefix(low, "bssid") || strings.HasPrefix(line, "SSID"):
			cur.SSID = val
		case strings.HasPrefix(low, "bssid") || strings.HasPrefix(line, "BSSID"):
			cur.BSSID = val
		case strings.Contains(low, "signal") || strings.Contains(line, "信号"):
			cur.Signal = val
		case strings.Contains(low, "radio type") || strings.Contains(line, "无线电类型"):
			cur.Radio = val
		case strings.Contains(low, "receive rate") || strings.Contains(line, "接收速率"):
			cur.RxRate = val
		case strings.Contains(low, "transmit rate") || strings.Contains(line, "传输速率"):
			cur.TxRate = val
		case strings.HasPrefix(low, "state") || strings.HasPrefix(line, "状态"):
			cur.State = val
		}
	}
	flush()
	return list
}

// GetNetInsight 返回网络洞察 MVP 数据（只读）。
func (a *App) GetNetInsight() (n NetInsight) {
	defer func() {
		if r := recover(); r != nil {
			startupLog("GetNetInsight panic: %v", r)
			n = NetInsight{ReadAt: time.Now().Format("2006-01-02 15:04:05"), Stats: map[string]int{}}
		}
	}()
	n = NetInsight{ReadAt: time.Now().Format("2006-01-02 15:04:05"), Stats: map[string]int{}}
	ni := a.GetNetworkInfo()
	n.Hostname, n.Adapters, n.Proxy = ni.Hostname, ni.Interfaces, ni.Proxy
	n.Routes = readRoutes()
	n.Connections, n.Ports = readNetConnectionsAndPorts()
	n.Arp = readArp()
	n.WiFi = readWiFi()
	n.Shares = readShares()
	n.Discovery = readDiscovery()
	for _, c := range n.Connections {
		n.Stats["conn_"+c.Scope]++
	}
	for _, p := range n.Ports {
		n.Stats["ports"]++
		if p.Exposed {
			n.Stats["ports_exposed"]++
		}
	}
	n.Stats["arp"] = len(n.Arp)
	n.Stats["adapters"] = len(n.Adapters)
	return n
}

func pingTarget(target string) (bool, string) {
	out, err := runHiddenOut(8*time.Second, resolveExe("ping.exe"), "-n", "2", "-w", "1000", target)
	txt := strings.TrimSpace(decodeConsole([]byte(out)))
	if err != nil {
		if len(txt) > 200 {
			txt = txt[:200]
		}
		return false, "不可达" + (func() string {
			if txt == "" {
				return ""
			}
			return "：" + txt
		})()
	}
	low := strings.ToLower(txt)
	if strings.Contains(low, "ttl=") || strings.Contains(txt, "时间<") || strings.Contains(txt, "时间=") || strings.Contains(low, "time<") || strings.Contains(low, "time=") {
		return true, "可达"
	}
	return false, "无响应"
}

// RunNetSegmentCheck 用户点击后执行：网卡→网关→DNS→公网。
func (a *App) RunNetSegmentCheck() (out []NetSegment) {
	defer func() {
		if r := recover(); r != nil {
			startupLog("RunNetSegmentCheck panic: %v", r)
			out = []NetSegment{{Name: "检查失败", Status: "bad", Detail: "探测异常，详情见 startup.log"}}
		}
	}()
	ni := a.GetNetworkInfo()
	seen := map[string]bool{}
	add := func(name, target string, fn func() (bool, string)) {
		if target == "" || seen[target] {
			return
		}
		seen[target] = true
		ok, detail := fn()
		st := "bad"
		if ok {
			st = "ok"
		}
		out = append(out, NetSegment{Name: name, Target: target, Status: st, Detail: detail})
	}
	for _, it := range ni.Interfaces {
		for _, g := range it.Gateways {
			add("网关", g, func() (bool, string) { return pingTarget(g) })
		}
		for _, d := range it.DNS {
			add("DNS", d, func() (bool, string) { return pingTarget(d) })
		}
		if len(it.IPv4)+len(it.IPv6) > 0 {
			out = append(out, NetSegment{Name: "网卡", Target: it.Name, Status: "ok", Detail: "已配置 " + strings.Join(append(it.IPv4, it.IPv6...), " / ")})
		}
	}
	add("公网", "223.5.5.5", func() (bool, string) { return pingTarget("223.5.5.5") })
	// DNS 解析单独测：ping 只证明 ICMP，不证明解析
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	if ips, err := net.DefaultResolver.LookupHost(ctx, "www.msftconnecttest.com"); err != nil {
		out = append(out, NetSegment{Name: "DNS 解析", Target: "www.msftconnecttest.com", Status: "bad", Detail: shortErr(err)})
	} else {
		out = append(out, NetSegment{Name: "DNS 解析", Target: "www.msftconnecttest.com", Status: "ok", Detail: strings.Join(ips, " / ")})
	}
	return out
}

// ---------- 两次快照对比 ----------

type NetSnapshot struct {
	Time     string   `json:"time"`
	Arp      []string `json:"arp"`
	Ports    []string `json:"ports"`
	DNS      []string `json:"dns"`
	External []string `json:"external"`
}

type NetDiff struct {
	PrevTime    string   `json:"prevTime"`
	CurTime     string   `json:"curTime"`
	NewArp      []string `json:"newArp"`
	NewPorts    []string `json:"newPorts"`
	DNSAdded    []string `json:"dnsAdded"`
	DNSRemoved  []string `json:"dnsRemoved"`
	NewExternal []string `json:"newExternal"`
	Note        string   `json:"note"`
}

func netSnapshotDir() string {
	if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
		return filepath.Join(local, "ahtool", "netinsight")
	}
	return filepath.Join(os.TempDir(), "ahtool_netinsight")
}

func snapshotFromInsight(d NetInsight) NetSnapshot {
	s := NetSnapshot{Time: d.ReadAt}
	for _, a := range d.Arp {
		s.Arp = append(s.Arp, a.IP+"|"+a.MAC)
	}
	for _, p := range d.Ports {
		s.Ports = append(s.Ports, p.Proto+"|"+p.Addr+"|"+p.Port+"|"+p.Process)
	}
	seenDNS := map[string]bool{}
	for _, a := range d.Adapters {
		for _, x := range a.DNS {
			if !seenDNS[x] {
				seenDNS[x] = true
				s.DNS = append(s.DNS, x)
			}
		}
	}
	seenExt := map[string]bool{}
	for _, c := range d.Connections {
		if c.Scope == "external" {
			k := c.Process + "|" + c.Remote
			if !seenExt[k] {
				seenExt[k] = true
				s.External = append(s.External, k)
			}
		}
	}
	sort.Strings(s.Arp)
	sort.Strings(s.Ports)
	sort.Strings(s.DNS)
	sort.Strings(s.External)
	return s
}

// TakeNetSnapshot 保存一份本机网络快照，返回文件路径。
func (a *App) TakeNetSnapshot() string {
	d := a.GetNetInsight()
	s := snapshotFromInsight(d)
	dir := netSnapshotDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "保存失败：" + err.Error()
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "保存失败：" + err.Error()
	}
	p := filepath.Join(dir, "snap_"+time.Now().Format("20060102_150405")+".json")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		return "保存失败：" + err.Error()
	}
	return p
}

func loadSnapshots() []struct {
	Path string
	Snap NetSnapshot
} {
	dir := netSnapshotDir()
	files, _ := filepath.Glob(filepath.Join(dir, "snap_*.json"))
	sort.Strings(files)
	var out []struct {
		Path string
		Snap NetSnapshot
	}
	for _, p := range files {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var s NetSnapshot
		if json.Unmarshal(b, &s) == nil {
			out = append(out, struct {
				Path string
				Snap NetSnapshot
			}{p, s})
		}
	}
	return out
}

func diffSet(old, cur []string) (added, removed []string) {
	o, c := map[string]bool{}, map[string]bool{}
	for _, x := range old {
		o[x] = true
	}
	for _, x := range cur {
		c[x] = true
	}
	for _, x := range cur {
		if !o[x] {
			added = append(added, x)
		}
	}
	for _, x := range old {
		if !c[x] {
			removed = append(removed, x)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return
}

// CompareLastNetSnapshot 对比最近两份快照；不足两份时返回提示。
func (a *App) CompareLastNetSnapshot() NetDiff {
	ss := loadSnapshots()
	if len(ss) < 2 {
		return NetDiff{Note: "至少需要两份快照才能对比；先点「保存快照」，过一会儿再保存一份。"}
	}
	prev, cur := ss[len(ss)-2].Snap, ss[len(ss)-1].Snap
	d := NetDiff{PrevTime: prev.Time, CurTime: cur.Time}
	d.NewArp, _ = diffSet(prev.Arp, cur.Arp)
	d.NewPorts, _ = diffSet(prev.Ports, cur.Ports)
	d.DNSAdded, d.DNSRemoved = diffSet(prev.DNS, cur.DNS)
	d.NewExternal, _ = diffSet(prev.External, cur.External)
	d.Note = "对比的是最近两份快照；ARP/连接是动态数据，变化不一定代表风险，只用于排障。"
	return d
}
