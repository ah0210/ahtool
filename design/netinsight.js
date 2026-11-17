/* ══════════════════════════════════════════════════════════════════════════
   网络洞察 · 独立页面（只读）
   网卡全景 / 路由与出口 / 连接与进程 / 端口暴露面 / 邻居 / 共享与发现 /
   分段连通性。探测只在用户点击「分段检查」时执行；全部本机展示，不上传。
   ══════════════════════════════════════════════════════════════════════════ */

var NI = {loading:false, err:'', data:null, connFilter:'all', connView:'raw', seg:null, segBusy:false, diff:null, snapBusy:false};
window.NI = NI;

function niEsc(s){
  return String(s == null ? '' : s).replace(/[&<>"]/g, function(c){
    return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c];
  });
}
function niEl(id){ return document.getElementById(id); }
function niJoin(a, sep){
  a = (a || []).filter(function(x){ return x != null && x !== ''; });
  return a.join(sep || ' · ');
}

function renderNetInsight(){
  return '<div class="page-head" style="display:flex;align-items:flex-end;justify-content:space-between;gap:14px">'
    + '<div><div class="page-title">网络洞察</div>'
    + '<div class="page-sub">看清本机连到哪、谁在对外通信、哪些端口对局域网开放。只读，不主动扫描、不上传。</div></div>'
    + '<div style="display:flex;gap:8px;flex-wrap:wrap">'
    +   '<button class="btn sm" data-ni-snap>' + ico('disk', 14) + '保存快照</button>'
    +   '<button class="btn sm" data-ni-diff>' + ico('refresh', 14) + '对比上次</button>'
    +   '<button class="btn sm" data-ni-refresh>' + ico('refresh', 14) + '刷新</button>'
    +   '<button class="btn sm primary" data-ni-seg>' + ico('wifi', 14) + '分段检查</button>'
    + '</div></div>'
    + '<div id="niBody">' + niBodyHtml() + '</div>';
}

function niBodyHtml(){
  if(NI.loading) return '<div class="card"><div class="empty" style="padding:40px">' + ico('refresh',26) + '<div class="empty-t">正在读取网络信息…</div></div></div>';
  if(NI.err) return '<div class="card"><div class="empty" style="padding:40px"><div class="empty-ico" style="color:var(--danger)">' + ico('warn',28) + '</div><div class="empty-t">读取失败</div><div class="empty-d">' + niEsc(NI.err) + '</div></div></div>';
  if(!NI.data) return '<div class="card"><div class="empty" style="padding:40px"><div class="empty-t">尚未读取</div></div></div>';
  var d = NI.data, used = d.stats || {};
  var head = '<div class="ni-stats">'
    +   niStat('网卡', used.adapters || 0, '已启用/有 IP 的网卡')
    +   niStat('连接', (d.connections || []).length, 'TCP/UDP 行数')
    +   niStat('监听端口', used.ports || 0, '其中对外 ' + (used.ports_exposed || 0) + ' 个')
    +   niStat('邻居', used.arp || 0, 'ARP 被动表')
    + '</div>';

  var adapters = (d.adapters || []).map(function(a){
    return '<div class="ni-card">'
      + '<div class="ni-card-t"><b>' + niEsc(a.name || '未命名网卡') + '</b>'
      +   '<span>' + niEsc(niJoin([a.type, a.status, a.speed])) + '</span></div>'
      + '<div class="ni-kv"><span>IP</span><b>' + niEsc(niJoin(a.ipv4 || [])) + (a.ipv6 && a.ipv6.length ? '<br>' + niEsc(niJoin(a.ipv6)) : '') + '</b></div>'
      + '<div class="ni-kv"><span>网关</span><b>' + niEsc(niJoin(a.gateways || [])) + '</b></div>'
      + '<div class="ni-kv"><span>DNS</span><b>' + niEsc(niJoin(a.dns || [])) + '</b></div>'
      + '<div class="ni-kv"><span>MAC</span><b>' + niEsc(a.mac || '') + '</b></div>'
      + '<div class="ni-kv"><span>DHCP</span><b>' + (a.dhcp ? '是 · ' + niEsc(a.dhcpServer || '服务器未读到') : '静态') + '</b></div>'
      + '</div>';
  }).join('');

  var routes = (d.routes || []).length
    ? '<div class="ni-code">' + (d.routes || []).map(function(x){ return niEsc(x); }).join('<br>') + '</div>'
    : '<div class="ni-empty">没有读到默认路由</div>';
  var px = d.proxy || {};
  var proxy = '<div class="ni-kv"><span>系统代理</span><b>' + (px.enabled ? '已启用 · ' + niEsc(px.server || '') : '未启用') + '</b></div>'
    + '<div class="ni-kv"><span>PAC</span><b>' + niEsc(px.autoConfigUrl || '未设置') + '</b></div>'
    + '<div class="ni-kv"><span>WinHTTP</span><b style="white-space:pre-wrap">' + niEsc(px.winhttp || '未读到') + '</b></div>'
    + (px.env && px.env.length ? '<div class="ni-kv"><span>环境变量</span><b>' + niEsc(px.env.join(' / ')) + '</b></div>' : '');

  var wifi = (d.wifi || []).map(function(w){
    return '<div class="ni-card"><div class="ni-card-t"><b>' + niEsc(w.ssid || '未关联') + '</b><span>' + niEsc(w.state || '') + '</span></div>'
      + '<div class="ni-kv"><span>信号</span><b>' + niEsc(w.signal || '—') + '</b></div>'
      + '<div class="ni-kv"><span>BSSID</span><b>' + niEsc(w.bssid || '—') + '</b></div>'
      + '<div class="ni-kv"><span>速率</span><b>' + niEsc(niJoin([w.rxRate, w.txRate], ' / ')) + '</b></div>'
      + '<div class="ni-kv"><span>无线电</span><b>' + niEsc(w.radio || '—') + '</b></div></div>';
  }).join('');
  var topo = niTopoSVG(d);

  var filters = [['all','全部'],['external','对外'],['lan','内网'],['listen','监听']];
  var conns = (d.connections || []).filter(function(c){
    return NI.connFilter === 'all' || c.scope === NI.connFilter;
  }).slice(0, 400);
  var connTable = '<div class="ni-filters">' + filters.map(function(f){
      return '<button class="ni-filter' + (NI.connFilter === f[0] ? ' on' : '') + '" data-nifilter="' + f[0] + '">' + f[1] + '</button>';
    }).join('')
    + '<span style="flex:1"></span>'
    + '<button class="ni-filter' + (NI.connView === 'raw' ? ' on' : '') + '" data-niview="raw">原始连接</button>'
    + '<button class="ni-filter' + (NI.connView === 'proc' ? ' on' : '') + '" data-niview="proc">按进程聚合</button>'
    + '</div>'
    + '<div class="ni-table-wrap"><table class="ni-table"><thead><tr>'
    + (NI.connView === 'proc'
        ? '<th>进程</th><th>PID</th><th>连接数</th><th>对外</th><th>内网</th><th>监听</th><th>路径</th>'
        : '<th>进程</th><th>协议</th><th>本地</th><th>远端</th><th>状态</th><th>PID</th><th>分类</th>')
    + '</tr></thead><tbody>'
    + (NI.connView === 'proc' ? niAggRows(conns) : (conns.length ? conns.map(function(c){
        return '<tr title="' + niEsc(c.path || '') + '"><td>' + niEsc(c.process || '—') + '</td><td>' + niEsc(c.proto) + '</td><td>' + niEsc(c.local) + '</td><td>' + niEsc(c.remote) + '</td><td>' + niEsc(c.state) + '</td><td>' + c.pid + '</td><td><i class="ni-badge ' + niEsc(c.scope) + '">' + niEsc(c.scope) + '</i></td></tr>';
      }).join('') : '<tr><td colspan="7" class="ni-empty">没有匹配的连接</td></tr>'))
    + '</tbody></table></div>';

  var ports = (d.ports || []).map(function(p){
    return '<tr><td>' + niEsc(p.proto) + '</td><td>' + niEsc(p.addr) + '</td><td><b>' + niEsc(p.port) + '</b></td><td>' + niEsc(p.process || '—') + '</td><td>' + p.pid + '</td><td><i class="ni-badge ' + (p.exposed ? 'bad' : 'ok') + '">' + (p.exposed ? '对外' : '仅本机') + '</i></td></tr>';
  }).join('');
  var portTable = '<div class="ni-table-wrap"><table class="ni-table"><thead><tr><th>协议</th><th>监听地址</th><th>端口</th><th>进程</th><th>PID</th><th>暴露</th></tr></thead><tbody>'
    + (ports || '<tr><td colspan="6" class="ni-empty">没有监听端口</td></tr>') + '</tbody></table></div>';

  var arp = (d.arp || []).map(function(a){
    return '<tr><td>' + niEsc(a.ip) + '</td><td>' + niEsc(a.mac) + '</td><td>' + niEsc(a.type || '') + '</td><td>' + niEsc(a.iface || '') + '</td></tr>';
  }).join('');
  var arpTable = '<div class="ni-table-wrap"><table class="ni-table"><thead><tr><th>IP</th><th>MAC</th><th>类型</th><th>接口</th></tr></thead><tbody>'
    + (arp || '<tr><td colspan="4" class="ni-empty">没有 ARP 邻居</td></tr>') + '</tbody></table></div>';

  var shares = (d.shares || []).length ? '<div class="ni-code">' + d.shares.map(niEsc).join('<br>') + '</div>' : '<div class="ni-empty">没有共享 / 未读取到</div>';
  var disc = (d.discovery || []).length ? '<div class="ni-code">' + d.discovery.map(niEsc).join('<br>') + '</div>' : '<div class="ni-empty">未读取到发现服务</div>';

  var seg = '';
  if(NI.segBusy) seg = '<div class="ni-empty">正在检查网卡 → 网关 → DNS → 公网…</div>';
  else if(NI.seg && NI.seg.length){
    seg = '<div class="ni-seg">' + NI.seg.map(function(s){
      return '<div class="ni-seg-row ' + niEsc(s.status) + '"><b>' + niEsc(s.name) + '</b>'
        + '<span>' + niEsc(s.target || '') + '</span><i>' + niEsc(s.detail || '') + '</i></div>';
    }).join('') + '</div>';
  } else seg = '<div class="ni-empty">点上方「分段检查」测试网卡→网关→DNS→公网；不会自动扫描局域网。</div>';

  return head
    + '<div class="ni-grid">'
    +   '<div class="ni-section"><div class="ni-h">' + ico('wifi', 15) + '网卡全景</div>' + (adapters || '<div class="ni-empty">没有网卡</div>') + (wifi ? '<div class="ni-sub2">WiFi</div>' + wifi : '') + '</div>'
    +   '<div class="ni-section"><div class="ni-h">' + ico('globe', 15) + '路由与出口 / 代理</div>' + routes + proxy + '</div>'
    +   '<div class="ni-section wide"><div class="ni-h">' + ico('list', 15) + '连接与进程 <span class="ni-sub">按进程聚合前先看原始连接；悬停可看 exe 路径</span></div>' + connTable + '</div>'
    +   '<div class="ni-section"><div class="ni-h">' + ico('shield', 15) + '端口暴露面 <span class="ni-sub">0.0.0.0 / :: 表示所有接口</span></div>' + portTable + '</div>'
    +   '<div class="ni-section"><div class="ni-h">' + ico('grid', 15) + 'ARP 邻居（被动表）</div>' + arpTable + '</div>'
    +   '<div class="ni-section"><div class="ni-h">' + ico('folder', 15) + '共享 / 网络发现</div>' + shares + '<div class="ni-sub2">发现服务</div>' + disc + '</div>'
    +   '<div class="ni-section wide"><div class="ni-h">' + ico('wifi', 15) + '分段连通性</div>' + seg + '</div>'
    +   '<div class="ni-section"><div class="ni-h">' + ico('grid', 15) + '已知拓扑 <span class="ni-sub">基于默认路由/ARP，不代表扫描结果</span></div>' + topo + '</div>'
    +   '<div class="ni-section"><div class="ni-h">' + ico('refresh', 15) + '快照对比 <span class="ni-sub">新设备 / 新端口 / DNS 变更</span></div>' + niDiffHtml() + '</div>'
    + '</div>';
}

function niAggRows(conns){
  var m = {}, order = [];
  for(var i = 0; i < conns.length; i++){
    var c = conns[i], k = (c.process || '—') + '|' + c.pid;
    if(!m[k]){ m[k] = {proc:c.process || '—', pid:c.pid, n:0, ext:0, lan:0, listen:0, path:c.path || ''}; order.push(k); }
    m[k].n++;
    if(c.scope === 'external') m[k].ext++;
    else if(c.scope === 'lan') m[k].lan++;
    else if(c.scope === 'listen' || c.scope === 'local') m[k].listen++;
  }
  if(!order.length) return '<tr><td colspan="7" class="ni-empty">没有匹配的连接</td></tr>';
  order.sort(function(a,b){ return m[b].n - m[a].n; });
  return order.map(function(k){
    var x = m[k];
    return '<tr title="' + niEsc(x.path) + '"><td>' + niEsc(x.proc) + '</td><td>' + x.pid + '</td><td><b>' + x.n + '</b></td><td>' + x.ext + '</td><td>' + x.lan + '</td><td>' + x.listen + '</td><td class="ni-path">' + niEsc(x.path || '—') + '</td></tr>';
  }).join('');
}

function niTopoSVG(d){
  var a = (d.adapters || [])[0] || {};
  var gw = (a.gateways || [])[0] || '';
  var dns = (a.dns || [])[0] || '';
  var arp = (d.arp || []).slice(0, 6);
  if(!gw && !dns && !arp.length) return '<div class="ni-empty">没有足够的路由/ARP 数据画拓扑</div>';
  var w = 720, h = 250;
  var svg = '<svg class="ni-topo" viewBox="0 0 ' + w + ' ' + h + '" xmlns="http://www.w3.org/2000/svg">';
  var hostX = 360, hostY = 150;
  var gwX = 360, gwY = 82;
  var inetX = 610, inetY = 40;
  var dnsX = 610, dnsY = 120;
  var line = function(x1,y1,x2,y2){ return '<line x1="'+x1+'" y1="'+y1+'" x2="'+x2+'" y2="'+y2+'" class="ni-topo-l"/>'; };
  var node = function(x,y,t,sub,cls){ return '<g class="ni-topo-n '+(cls||'')+'"><circle cx="'+x+'" cy="'+y+'" r="24"/><text x="'+x+'" y="'+(y+4)+'" text-anchor="middle" class="ni-topo-t">'+niEsc(t)+'</text>'
    + (sub ? '<text x="'+x+'" y="'+(y+40)+'" text-anchor="middle" class="ni-topo-s">'+niEsc(sub)+'</text>' : '') + '</g>'; };
  svg += line(hostX, hostY-24, gwX, gwY+24);
  if(gw) svg += node(gwX, gwY, '网关', gw, 'gw');
  svg += line(gwX, gwY-24, inetX, inetY+24);
  svg += node(inetX, inetY, 'Internet', '默认路由', 'inet');
  if(dns){ svg += line(hostX+24, hostY, dnsX-24, dnsY); svg += node(dnsX, dnsY, 'DNS', dns, 'dns'); }
  var startX = 100, gap = 110;
  arp.forEach(function(x, i){
    var nx = startX + i*gap, ny = 225;
    svg += line(hostX, hostY+24, nx, ny-24);
    svg += node(nx, ny, x.ip, x.mac, 'neighbor');
  });
  svg += node(hostX, hostY, '本机', a.name || '', 'host');
  svg += '</svg>';
  return '<div class="ni-topo-wrap">' + svg + '</div><div class="ni-empty">拓扑只画默认路由/ARP 已知关系；邻居多时只显示前 6 个。</div>';
}

function niDiffHtml(){
  if(NI.snapBusy) return '<div class="ni-empty">正在保存/对比快照…</div>';
  var d = NI.diff;
  if(!d) return '<div class="ni-empty">先点「保存快照」，过一会儿再保存一份，然后点「对比上次」。</div>';
  if(!d.curTime && d.note) return '<div class="ni-empty">' + niEsc(d.note) + '</div>';
  var list = function(arr, cls){
    if(!arr || !arr.length) return '<div class="ni-empty">无</div>';
    return '<div class="ni-code">' + arr.map(function(x){ return '<span class="' + (cls||'') + '">' + niEsc(x) + '</span>'; }).join('<br>') + '</div>';
  };
  return '<div class="ni-sub2">' + niEsc(d.prevTime || '') + ' → ' + niEsc(d.curTime || '') + '</div>'
    + '<div class="ni-sub2">新 ARP 邻居</div>' + list(d.newArp)
    + '<div class="ni-sub2">新监听端口</div>' + list(d.newPorts)
    + '<div class="ni-sub2">DNS 变更</div>' + list(d.dnsAdded, 'ni-new') + (d.dnsRemoved && d.dnsRemoved.length ? '<div class="ni-sub2">DNS 移除</div>' + list(d.dnsRemoved) : '')
    + '<div class="ni-sub2">新对外连接</div>' + list(d.newExternal)
    + '<div class="ni-empty">' + niEsc(d.note || '') + '</div>';
}

function niStat(k, v, s){ return '<div class="ni-stat"><span>' + niEsc(k) + '</span><b>' + v + '</b><small>' + niEsc(s) + '</small></div>'; }

function loadNetInsight(){
  if(NI.loading) return;
  NI.loading = true; NI.err = '';
  var b = niEl('niBody'); if(b) b.innerHTML = niBodyHtml();
  callGo('GetNetInsight').then(function(d){
    NI.data = d || {}; NI.loading = false;
    var b2 = niEl('niBody'); if(b2) b2.innerHTML = niBodyHtml();
  }).catch(function(e){
    NI.loading = false; NI.err = String(e);
    var b2 = niEl('niBody'); if(b2) b2.innerHTML = niBodyHtml();
  });
}
window.loadNetInsight = loadNetInsight;

function bindNetInsight(){
  var root = niEl('content');
  if(!root) return;
  if(root.dataset.niBound !== '1'){
    root.dataset.niBound = '1';
    root.addEventListener('click', function(e){
      var t = e.target; if(!t || !t.closest) return;
      if(t.closest('[data-ni-refresh]')){ NI.data = null; loadNetInsight(); return; }
      if(t.closest('[data-ni-snap]')){
        if(NI.snapBusy) return;
        NI.snapBusy = true;
        callGo('TakeNetSnapshot').then(function(msg){
          NI.snapBusy = false;
          toast(msg && msg.indexOf('失败') >= 0 ? msg : ('快照已保存：' + msg), msg && msg.indexOf('失败') >= 0 ? 'danger' : 'ok');
        }).catch(function(e){ NI.snapBusy = false; toast('保存快照失败：' + e, 'danger'); });
        return;
      }
      if(t.closest('[data-ni-diff]')){
        if(NI.snapBusy) return;
        NI.snapBusy = true;
        callGo('CompareLastNetSnapshot').then(function(d){
          NI.snapBusy = false; NI.diff = d || null;
          var b = niEl('niBody'); if(b) b.innerHTML = niBodyHtml();
        }).catch(function(e){ NI.snapBusy = false; toast('对比失败：' + e, 'danger'); });
        return;
      }
      var vw = t.closest('[data-niview]');
      if(vw){ NI.connView = vw.getAttribute('data-niview'); var b0 = niEl('niBody'); if(b0) b0.innerHTML = niBodyHtml(); return; }
      if(t.closest('[data-ni-seg]')){
        if(NI.segBusy) return;
        NI.segBusy = true; NI.seg = null;
        var b = niEl('niBody'); if(b) b.innerHTML = niBodyHtml();
        callGo('RunNetSegmentCheck').then(function(r){
          NI.seg = r || []; NI.segBusy = false;
          var b2 = niEl('niBody'); if(b2) b2.innerHTML = niBodyHtml();
        }).catch(function(err){
          NI.seg = [{name:'检查失败', status:'bad', detail:String(err)}]; NI.segBusy = false;
          var b2 = niEl('niBody'); if(b2) b2.innerHTML = niBodyHtml();
        });
        return;
      }
      var f = t.closest('[data-nifilter]');
      if(f){ NI.connFilter = f.getAttribute('data-nifilter'); var b3 = niEl('niBody'); if(b3) b3.innerHTML = niBodyHtml(); return; }
    });
  }
  if(!NI.data && !NI.loading && !NI.err) loadNetInsight();
}
window.bindNetInsight = bindNetInsight;
window.renderNetInsight = renderNetInsight;
