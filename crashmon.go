package main

// 异常关机 / 蓝屏事件时间线（只读）。
//
// 数据源：System 事件日志（wevtutil /f:xml），不解析本地化文本。
//   - Kernel-Power 41：未正常关机（断电/死机/强制关机/蓝屏后重启）
//   - BugCheck 1001：蓝屏，带 BugCheck 代码与参数
//   - EventLog 6008：上一次关机是意外的
//   - WHEA-Logger 17/18/19/20/46/47：硬件错误
//
// 结论只给“证据 + 可能方向”，不把 41 直接判成电源损坏。

import (
	"encoding/xml"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/windows/registry"
)

type CrashEvent struct {
	Time     string   `json:"time"`
	Kind     string   `json:"kind"` // 蓝屏 / 异常关机 / 硬件错误
	EventID  int      `json:"eventId"`
	Provider string   `json:"provider"`
	Level    string   `json:"level"` // bad / warn / info
	Code     string   `json:"code,omitempty"`
	Title    string   `json:"title"`
	Detail   string   `json:"detail,omitempty"`
	Params   []string `json:"params,omitempty"`
	Advice   string   `json:"advice,omitempty"`
}

type CrashConfig struct {
	Enabled        string `json:"enabled"`
	DumpType       string `json:"dumpType"`
	DumpFile       string `json:"dumpFile"`
	MinidumpDir    string `json:"minidumpDir"`
	AutoReboot     string `json:"autoReboot"`
	Overwrite      string `json:"overwrite"`
	LogEvent       string `json:"logEvent"`
	AlwaysKeepDump string `json:"alwaysKeepDump"`
}

type CrashTimeline struct {
	Events      []CrashEvent `json:"events"`
	Config      CrashConfig  `json:"config"`
	Summary     string       `json:"summary"`
	Suggestions []string     `json:"suggestions"`
	Note        string       `json:"note"`
	ReadAt      string       `json:"readAt"`
}

type crashXMLEvent struct {
	System struct {
		EventID  int    `xml:"EventID"`
		Level    int    `xml:"Level"`
		Computer string `xml:"Computer"`
		Provider struct {
			Name string `xml:"Name,attr"`
		} `xml:"Provider"`
		TimeCreated struct {
			SystemTime string `xml:"SystemTime,attr"`
		} `xml:"TimeCreated"`
	} `xml:"System"`
	EventData struct {
		Data []xmlData `xml:"Data"`
	} `xml:"EventData"`
	RenderingInfo struct {
		Message string `xml:"Message"`
	} `xml:"RenderingInfo"`
}

func crashLevelName(v int) string {
	switch v {
	case 1:
		return "bad"
	case 2:
		return "bad"
	case 3:
		return "warn"
	default:
		return "info"
	}
}

func dataMap(d []xmlData) map[string]string {
	m := map[string]string{}
	for _, x := range d {
		m[x.Name] = strings.TrimSpace(x.Value)
	}
	return m
}

func crashCode(m map[string]string) (uint32, string, []string) {
	raw := strings.TrimSpace(m["BugcheckCode"])
	if raw == "" {
		return 0, "", nil
	}
	n, err := strconv.ParseUint(raw, 0, 32)
	if err != nil {
		return 0, raw, nil
	}
	code := uint32(n)
	var params []string
	for i := 1; i <= 4; i++ {
		if v := strings.TrimSpace(m[fmt.Sprintf("BugcheckParameter%d", i)]); v != "" {
			params = append(params, v)
		}
	}
	return code, fmt.Sprintf("0x%08X", code), params
}

func crashEventFromXML(e crashXMLEvent) CrashEvent {
	m := dataMap(e.EventData.Data)
	ce := CrashEvent{
		EventID:  e.System.EventID,
		Provider: strings.TrimSpace(e.System.Provider.Name),
		Level:    crashLevelName(e.System.Level),
		Detail:   strings.TrimSpace(e.RenderingInfo.Message),
	}
	if t, err := time.Parse(time.RFC3339Nano, e.System.TimeCreated.SystemTime); err == nil {
		ce.Time = t.Local().Format("2006-01-02 15:04:05")
	} else {
		ce.Time = e.System.TimeCreated.SystemTime
	}
	if len(ce.Detail) > 600 {
		ce.Detail = ce.Detail[:600] + "…"
	}
	switch e.System.EventID {
	case 1001:
		ce.Kind = "蓝屏"
		code, hex, params := crashCode(m)
		ce.Code, ce.Params = hex, params
		ce.Title = "BugCheck " + hex
		if code != 0 {
			bi := bsodLookup(code)
			ce.Title = bi.title + "（" + hex + "）"
			ce.Advice = strings.Join(bi.advice, "；") + "；" + bi.cause
		}
	case 41:
		ce.Kind = "异常关机"
		ce.Title = "系统未正常关机"
		ce.Level = "warn"
		code, hex, params := crashCode(m)
		ce.Code, ce.Params = hex, params
		if code != 0 {
			bi := bsodLookup(code)
			ce.Advice = "事件里带 BugCheck 码 " + hex + "：" + strings.Join(bi.advice, "；")
		} else {
			ce.Advice = "可能原因：断电/电源不稳、强制关机、死机、过热或蓝屏后自动重启。先结合同一时间的 BugCheck/WHEA 事件判断，不能只看 41 就认定电源坏了。"
		}
	case 6008:
		ce.Kind = "异常关机"
		ce.Title = "上一次关机是意外的"
		ce.Level = "warn"
		ce.Advice = "检查同一时间是否有 Kernel-Power 41 / BugCheck 1001 / WHEA；若没有，优先排查供电、过热、强制关机。"
	default:
		ce.Kind = "硬件错误"
		ce.Title = "WHEA 硬件错误"
		ce.Level = "bad"
		var parts []string
		for _, k := range []string{"ErrorSource", "ErrorType", "ErrorStatus", "ValidBits", "Length"} {
			if v := strings.TrimSpace(m[k]); v != "" {
				parts = append(parts, k+"="+v)
			}
		}
		if len(parts) > 0 {
			ce.Detail = strings.Join(parts, "；")
		}
		ce.Advice = "WHEA 表示硬件/固件报告了错误：优先检查内存（MemTest）、CPU 温度/超频、电源、PCIe 设备；更新 BIOS/芯片组驱动后再观察。"
	}
	return ce
}

func readCrashConfig() CrashConfig {
	c := CrashConfig{Enabled: "Unknown", DumpType: "Unknown", AutoReboot: "Unknown", Overwrite: "Unknown", LogEvent: "Unknown", AlwaysKeepDump: "Unknown"}
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\CrashControl`, registry.QUERY_VALUE)
	if err != nil {
		return c
	}
	defer k.Close()
	getD := func(name string) (uint64, bool) {
		v, _, e := k.GetIntegerValue(name)
		return v, e == nil
	}
	if v, ok := getD("CrashDumpEnabled"); ok {
		c.Enabled = "是"
		switch v {
		case 0:
			c.Enabled, c.DumpType = "否", "未启用"
		case 1:
			c.DumpType = "完整内存转储"
		case 2:
			c.DumpType = "内核内存转储"
		case 3:
			c.DumpType = "小内存转储"
		case 7:
			c.DumpType = "自动内存转储"
		default:
			c.DumpType = fmt.Sprintf("未知类型 %d", v)
		}
	}
	yesNo := func(name string) string {
		if v, ok := getD(name); ok {
			if v != 0 {
				return "是"
			}
			return "否"
		}
		return "Unknown"
	}
	c.AutoReboot, c.Overwrite, c.LogEvent, c.AlwaysKeepDump = yesNo("AutoReboot"), yesNo("Overwrite"), yesNo("LogEvent"), yesNo("AlwaysKeepMemoryDump")
	if v, _, e := k.GetStringValue("DumpFile"); e == nil {
		c.DumpFile = v
	}
	if v, _, e := k.GetStringValue("MinidumpDir"); e == nil {
		c.MinidumpDir = v
	}
	return c
}

// GetCrashTimeline 返回最近 days 天的蓝屏/异常关机/硬件错误时间线（只读）。
func (a *App) GetCrashTimeline(days int) (out CrashTimeline) {
	defer func() {
		if r := recover(); r != nil {
			startupLog("GetCrashTimeline panic: %v", r)
			out = CrashTimeline{ReadAt: time.Now().Format("2006-01-02 15:04:05"), Summary: "事件日志读取异常，详情见 startup.log"}
		}
	}()
	if days <= 0 || days > 365 {
		days = 30
	}
	out = CrashTimeline{ReadAt: time.Now().Format("2006-01-02 15:04:05"), Config: readCrashConfig()}
	out.Note = "事件日志只保留有限历史；日志被清空/轮转时，这里看不到更早的记录。结论是排查方向，不是故障认定。"
	q := `*[System[(EventID=41 or EventID=1001 or EventID=6008) or (Provider[@Name='Microsoft-Windows-WHEA-Logger'] and (EventID=17 or EventID=18 or EventID=19 or EventID=20 or EventID=46 or EventID=47))]]`
	raw, err := runHiddenOut(30*time.Second, resolveExe("wevtutil.exe"), "qe", "System", "/q:"+q, "/f:xml", "/c:100", "/rd:true")
	if err != nil {
		out.Summary = "事件日志查询失败：" + shortErr(err)
		out.Suggestions = []string{"确认以管理员身份运行；检查 System 日志是否可读。"}
		return out
	}
	txt := bootSanitize(reXmlns.ReplaceAllString(decodeConsole([]byte(raw)), ""))
	if strings.TrimSpace(txt) == "" {
		out.Summary = "最近没有异常关机/蓝屏/硬件错误事件"
		return out
	}
	var wrap struct {
		Events []crashXMLEvent `xml:"Event"`
	}
	if err := xml.Unmarshal([]byte("<Events>"+txt+"</Events>"), &wrap); err != nil {
		out.Summary = "事件 XML 解析失败：" + shortErr(err)
		return out
	}
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	for _, e := range wrap.Events {
		ce := crashEventFromXML(e)
		if ce.Time != "" {
			if t, err := time.ParseInLocation("2006-01-02 15:04:05", ce.Time, time.Local); err == nil && t.Before(cutoff) {
				continue
			}
		}
		out.Events = append(out.Events, ce)
	}
	sort.SliceStable(out.Events, func(i, j int) bool { return out.Events[i].Time > out.Events[j].Time })
	if len(out.Events) > 100 {
		out.Events = out.Events[:100]
	}
	bsod, shutdown, whea := 0, 0, 0
	for _, e := range out.Events {
		switch e.Kind {
		case "蓝屏":
			bsod++
		case "异常关机":
			shutdown++
		default:
			whea++
		}
	}
	out.Summary = fmt.Sprintf("最近 %d 天：蓝屏 %d 次 / 异常关机 %d 次 / 硬件错误 %d 次。", days, bsod, shutdown, whea)
	out.Suggestions = []string{
		"先看时间线里同一次蓝屏是否同时有 BugCheck 1001 和 Kernel-Power 41：有 1001 时以 BugCheck 码和转储为主。",
		"有 WHEA 事件时优先做内存诊断、检查 CPU/内存超频与散热，再更新 BIOS/芯片组驱动。",
		"没有转储时，确认「系统属性 → 高级 → 启动和故障恢复 → 写入调试信息」已开启小内存转储。",
	}
	if out.Config.Enabled == "否" || out.Config.Enabled == "Unknown" {
		out.Suggestions = append(out.Suggestions, "当前转储配置未开启或读不到；蓝屏后可能没有 .dmp，建议开启小内存转储。")
	}
	return out
}
