package main

// 深度网络信息（本机配置 → 网络与音频）。
//
// 只读：网卡/IP/网关/DNS/DHCP、代理（IE/WinHTTP/环境变量）、防火墙配置、
// hosts 自定义条目、监听端口、SMB1/RDP 等安全相关开关。
// 读不到写空/Unknown，不推断；所有内容只在本机展示，不上传。

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/windows/registry"
)

type NetInterfaceInfo struct {
	Name       string   `json:"name"`
	Desc       string   `json:"desc"`
	Type       string   `json:"type"`
	Status     string   `json:"status"`
	MAC        string   `json:"mac"`
	Speed      string   `json:"speed"`
	DHCP       bool     `json:"dhcp"`
	IPv4       []string `json:"ipv4"`
	IPv6       []string `json:"ipv6"`
	Subnets    []string `json:"subnets"`
	Gateways   []string `json:"gateways"`
	DNS        []string `json:"dns"`
	DNSDomain  string   `json:"dnsDomain"`
	DHCPv4     string   `json:"dhcpServer"`
	WINS       []string `json:"wins"`
	LeaseStart string   `json:"leaseStart"`
}

type ProxyInfo struct {
	Enabled       bool     `json:"enabled"`
	Server        string   `json:"server"`
	Override      string   `json:"override"`
	AutoConfigURL string   `json:"autoConfigUrl"`
	WinHTTP       string   `json:"winhttp"`
	Env           []string `json:"env"`
}

type FirewallProfile struct {
	Name    string `json:"name"`
	Enabled string `json:"enabled"`
}

type NetFinding struct {
	Level  string `json:"level"` // ok / info / warn / bad
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

type NetworkInfo struct {
	Hostname     string             `json:"hostname"`
	Domain       string             `json:"domain"`
	Interfaces   []NetInterfaceInfo `json:"interfaces"`
	Proxy        ProxyInfo          `json:"proxy"`
	Firewall     []FirewallProfile  `json:"firewall"`
	HostsCount   int                `json:"hostsCount"`
	HostsSample  []string           `json:"hostsSample"`
	LocalServers []string           `json:"localServers"`
	Listening    []string           `json:"listening"`
	Findings     []NetFinding       `json:"findings"`
	ReadAt       string             `json:"readAt"`
}

func splitList(s string) []string {
	var out []string
	for _, x := range strings.Split(s, ",") {
		x = strings.TrimSpace(x)
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}

func isIPv6(s string) bool { return strings.Contains(s, ":") }

// readProxyInfo 读 IE/系统代理、WinHTTP 代理和环境变量代理。
func readProxyInfo() ProxyInfo {
	var p ProxyInfo
	if k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE); err == nil {
		defer k.Close()
		if v, _, e := k.GetIntegerValue("ProxyEnable"); e == nil {
			p.Enabled = v != 0
		}
		if v, _, e := k.GetStringValue("ProxyServer"); e == nil {
			p.Server = strings.TrimSpace(v)
		}
		if v, _, e := k.GetStringValue("ProxyOverride"); e == nil {
			p.Override = strings.TrimSpace(v)
		}
		if v, _, e := k.GetStringValue("AutoConfigURL"); e == nil {
			p.AutoConfigURL = strings.TrimSpace(v)
		}
	}
	if out, err := runHiddenOut(10*time.Second, resolveExe("netsh.exe"), "winhttp", "show", "proxy"); err == nil {
		p.WinHTTP = strings.TrimSpace(decodeConsole([]byte(out)))
	}
	for _, k := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			p.Env = append(p.Env, k+"="+v)
		}
	}
	return p
}

func readHostsInfo() (int, []string) {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	b, err := os.ReadFile(filepath.Join(root, `System32\drivers\etc\hosts`))
	if err != nil {
		return 0, nil
	}
	count := 0
	var sample []string
	for i, line := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(strings.TrimRight(line, "\r"))
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		count++
		if len(sample) < 10 {
			sample = append(sample, fmt.Sprintf("L%d: %s", i+1, t))
		}
	}
	return count, sample
}

func readFirewallProfiles() []FirewallProfile {
	rows, err := wmiQuery(`root\StandardCimv2`,
		"SELECT Name,Enabled FROM MSFT_NetFirewallProfile",
		[]string{"Name", "Enabled"})
	if err != nil {
		return nil
	}
	var out []FirewallProfile
	for _, r := range rows {
		en := strings.TrimSpace(r["Enabled"])
		switch en {
		case "1", "true", "True":
			en = "开启"
		case "0", "false", "False":
			en = "关闭"
		default:
			en = "Unknown"
		}
		out = append(out, FirewallProfile{Name: strings.TrimSpace(r["Name"]), Enabled: en})
	}
	return out
}

func readListeningPorts() []string {
	out, err := runHiddenOut(15*time.Second, resolveExe("netstat.exe"), "-ano", "-p", "TCP")
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var list []string
	for _, line := range strings.Split(decodeConsole([]byte(out)), "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" || !strings.HasPrefix(strings.ToUpper(line), "TCP") {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 5 {
			continue
		}
		state := strings.ToUpper(f[3])
		if state != "LISTENING" && !strings.Contains(line, "侦听") {
			continue
		}
		item := f[1] + "  pid=" + f[4]
		if !seen[item] {
			seen[item] = true
			list = append(list, item)
		}
		if len(list) >= 40 {
			break
		}
	}
	return list
}

func readSMB1RDP() (smb1 int, rdpAllowed bool, rdpKnown bool) {
	smb1 = -1
	if k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SYSTEM\CurrentControlSet\Services\LanmanServer\Parameters`, registry.QUERY_VALUE); err == nil {
		if v, _, e := k.GetIntegerValue("SMB1"); e == nil {
			smb1 = int(v)
		}
		k.Close()
	}
	if k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SYSTEM\CurrentControlSet\Control\Terminal Server`, registry.QUERY_VALUE); err == nil {
		if v, _, e := k.GetIntegerValue("fDenyTSConnections"); e == nil {
			rdpKnown = true
			rdpAllowed = v == 0
		}
		k.Close()
	}
	return
}

// GetNetworkInfo 返回本机深度网络信息与安全提示。
func (a *App) GetNetworkInfo() (n NetworkInfo) {
	defer func() {
		if r := recover(); r != nil {
			startupLog("GetNetworkInfo panic: %v", r)
			// 返回空结构而不是让进程退出；前端会显示“未读到”，不影响首页启动。
			n = NetworkInfo{ReadAt: time.Now().Format("2006-01-02 15:04:05")}
			n.Findings = []NetFinding{{Level: "info", Title: "网络信息读取失败", Detail: "WMI/网卡查询异常，详情见 startup.log"}}
		}
	}()
	n = NetworkInfo{ReadAt: time.Now().Format("2006-01-02 15:04:05")}
	n.Hostname, _ = os.Hostname()
	n.Proxy = readProxyInfo()
	n.Firewall = readFirewallProfiles()
	n.HostsCount, n.HostsSample = readHostsInfo()
	n.Listening = readListeningPorts()

	// 网卡：Win32_NetworkAdapterConfiguration 带 IP/网关/DNS/DHCP；再按 MAC 合并 readNetCards 的速率/类型。
	cards := map[string]hwNetCard{}
	for _, c := range readNetCards() {
		cards[strings.ToUpper(strings.TrimSpace(c.MAC))] = c
	}
	rows, err := wmiQuery(`root\cimv2`,
		"SELECT Description,MACAddress,IPAddress,IPSubnet,DefaultIPGateway,DNSServerSearchOrder,DNSDomain,DHCPEnabled,DHCPServer,WINSPrimaryServer,WINSSecondaryServer,DHCPLeaseObtained FROM Win32_NetworkAdapterConfiguration WHERE IPEnabled = TRUE",
		[]string{"Description", "MACAddress", "IPAddress", "IPSubnet", "DefaultIPGateway", "DNSServerSearchOrder", "DNSDomain", "DHCPEnabled", "DHCPServer", "WINSPrimaryServer", "WINSSecondaryServer", "DHCPLeaseObtained"})
	if err == nil {
		for _, r := range rows {
			mac := strings.ToUpper(strings.TrimSpace(r["MACAddress"]))
			it := NetInterfaceInfo{
				Desc:      strings.TrimSpace(r["Description"]),
				MAC:       mac,
				DHCP:      strings.EqualFold(strings.TrimSpace(r["DHCPEnabled"]), "true") || strings.TrimSpace(r["DHCPEnabled"]) == "1",
				DNSDomain: strings.TrimSpace(r["DNSDomain"]),
				DHCPv4:    strings.TrimSpace(r["DHCPServer"]),
				Subnets:   splitList(r["IPSubnet"]),
				Gateways:  splitList(r["DefaultIPGateway"]),
				DNS:       splitList(r["DNSServerSearchOrder"]),
			}
			for _, ip := range splitList(r["IPAddress"]) {
				if isIPv6(ip) {
					it.IPv6 = append(it.IPv6, ip)
				} else {
					it.IPv4 = append(it.IPv4, ip)
				}
			}
			if c, ok := cards[mac]; ok {
				it.Name, it.Type, it.Speed, it.Status = c.Name, c.Type, c.Speed, c.Link
			}
			if it.Name == "" {
				it.Name = it.Desc
			}
			if it.Type == "" {
				if strings.Contains(strings.ToLower(it.Desc), "wireless") || strings.Contains(it.Desc, "无线") {
					it.Type = "无线"
				} else {
					it.Type = "以太网"
				}
			}
			if it.Status == "" {
				if len(it.IPv4)+len(it.IPv6) > 0 {
					it.Status = "已连接"
				} else {
					it.Status = "未连接"
				}
			}
			if it.DHCPv4 != "" {
				it.LeaseStart = strings.TrimSpace(r["DHCPLeaseObtained"])
			}
			if it.DNSDomain == "" {
				if v, _, e := registryGetString(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\Tcpip\Parameters`, "Domain"); e == nil {
					n.Domain = v
				}
			}
			if len(it.IPv4)+len(it.IPv6) > 0 {
				n.Interfaces = append(n.Interfaces, it)
			}
		}
	}
	sort.Slice(n.Interfaces, func(i, j int) bool { return n.Interfaces[i].Name < n.Interfaces[j].Name })

	// 本地服务器候选：本机、网关、DHCP、DNS、SMB。
	serverSeen := map[string]bool{}
	addServer := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" && !serverSeen[s] {
			serverSeen[s] = true
			n.LocalServers = append(n.LocalServers, s)
		}
	}
	addServer("127.0.0.1 (localhost)")
	if n.Hostname != "" {
		addServer(n.Hostname + " (本机名)")
		addServer("smb://" + n.Hostname)
	}
	for _, it := range n.Interfaces {
		for _, g := range it.Gateways {
			addServer("网关 " + g)
		}
		if it.DHCPv4 != "" {
			addServer("DHCP " + it.DHCPv4)
		}
		for _, d := range it.DNS {
			addServer("DNS " + d)
		}
	}

	// 安全提示：只做“值得确认”的提示，不把正常配置说成病毒。
	add := func(level, title, detail string) {
		n.Findings = append(n.Findings, NetFinding{Level: level, Title: title, Detail: detail})
	}
	if n.Proxy.Enabled {
		add("warn", "系统代理已启用", "代理服务器："+emptyDash(n.Proxy.Server)+"；如果不是你自己设置的，请检查是否被劫持")
	}
	if n.Proxy.AutoConfigURL != "" {
		add("warn", "PAC 自动配置已设置", n.Proxy.AutoConfigURL)
	}
	if n.Proxy.WinHTTP != "" && !strings.Contains(strings.ToLower(n.Proxy.WinHTTP), "direct") && !strings.Contains(n.Proxy.WinHTTP, "直接") && !strings.Contains(n.Proxy.WinHTTP, "无代理") {
		add("info", "WinHTTP 代理", n.Proxy.WinHTTP)
	}
	for _, e := range n.Proxy.Env {
		add("info", "环境变量代理", e)
	}
	if n.HostsCount > 0 {
		add("warn", fmt.Sprintf("hosts 有 %d 条自定义记录", n.HostsCount), "请确认这些条目是你自己添加的（广告屏蔽/本地调试常见）；不是的话可能被劫持")
	}
	if len(n.Firewall) > 0 {
		var off []string
		for _, f := range n.Firewall {
			if f.Enabled == "关闭" {
				off = append(off, f.Name)
			}
		}
		if len(off) > 0 {
			add("bad", "防火墙有配置文件未开启", strings.Join(off, " / "))
		} else {
			add("ok", "防火墙", "域/专用/公用配置文件均已开启")
		}
	} else {
		add("info", "防火墙状态", "未能读取（可能被安全软件接管或 WMI 不可用）")
	}
	smb1, rdpAllowed, rdpKnown := readSMB1RDP()
	if smb1 == 1 {
		add("warn", "SMB1 已启用", "SMB1 是老旧协议，存在已知风险；仅连接老 NAS/XP 时才需要")
	} else if smb1 == -1 {
		add("info", "SMB1 状态", "未读到（可能由系统默认策略管理）")
	}
	if rdpKnown && rdpAllowed {
		add("info", "远程桌面已允许", "如不需要远程桌面，建议在系统设置中关闭")
	}
	for _, it := range n.Interfaces {
		if len(it.DNS) == 0 {
			add("warn", "网卡没有 DNS", it.Name+" 未配置 DNS，域名解析可能失败")
		}
	}
	if len(n.Findings) == 0 {
		add("ok", "未发现明显的网络配置风险", "代理、hosts、防火墙、DNS 均未发现值得警惕的项")
	}
	return n
}

func registryGetString(root registry.Key, path, name string) (string, uint32, error) {
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return "", 0, err
	}
	defer k.Close()
	return k.GetStringValue(name)
}
