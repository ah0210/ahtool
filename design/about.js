/* ══════════════════════════════════════════════════════════════════════════
   关于 · 独立模块（二次开发版）
   ---------------------------------------------------------------------------
   只展示本机可验证的信息：版本号、构建日期、许可协议、当前 exe 的 SHA256。
   本分支已解除原作者服务端绑定，不展示任何联系方式、社群、赞赏或外部服务入口。
   数据全部来自后端 GetAboutInfo()，页面上不写死版本号。
   ══════════════════════════════════════════════════════════════════════════ */

var AB = {state:'loading', err:'', info:null};
window.AB = AB;
window.ABOUT = null;

function abEsc(s){
  return String(s == null ? '' : s).replace(/[&<>"]/g, function(c){
    return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c];
  });
}
function abOnPage(){ return !!document.querySelector('[data-abpage]'); }

function abAfter(){
  if(abOnPage() && typeof render === 'function') render();
  if(window.setIsOpen && window.setIsOpen() && typeof paintSettings === 'function') paintSettings();
}

function loadAbout(){
  if(!hasGo()){
    if(inWails()){ AB.state = 'failed'; AB.err = '后端连接还没就绪（window.go 为空）'; }
    abAfter();
    return;
  }
  AB.state = 'loading';
  callGo('GetAboutInfo').then(function(r){
    AB.info = r || {};
    AB.state = 'ready';
    window.ABOUT = AB.info;
    abAfter();
  }).catch(function(e){
    AB.state = 'failed'; AB.err = String(e);
    abAfter();
  });
}
window.loadAbout = loadAbout;

function abRow(k, v){
  return '<div class="ab-kv"><span>' + abEsc(k) + '</span><b>' + v + '</b></div>';
}

function renderAboutPane(){
  if(AB.state === 'failed'){
    return '<div class="card"><div class="empty" style="padding:40px 20px">'
      + '<div class="empty-ico" style="color:var(--danger)">' + ico('warn', 28) + '</div>'
      + '<div class="empty-t">没能读到版本信息</div><div class="empty-d">' + abEsc(AB.err) + '</div>'
      + '<div style="display:flex;justify-content:center;margin-top:14px">'
      + '<button class="btn primary" data-abact="retry">' + ico('refresh', 16) + '重新读取</button>'
      + '</div></div></div>';
  }
  if(AB.state !== 'ready'){
    return '<div class="card"><div class="empty" style="padding:48px 20px">'
      + '<div class="empty-ico" style="color:var(--info)">' + ico('refresh', 28) + '</div>'
      + '<div class="empty-t">正在读版本信息…</div></div></div>';
  }

  var a = AB.info || {};
  var ver = a.version || '';
  var bd = (a.buildDate && a.buildDate !== 'unknown') ? a.buildDate : '未知';
  var hash = a.selfHash || '';

  return '<div class="ab-pane">'
    + '<div class="card ab-hero">'
    +   '<div class="ab-name">ah系统优化工具箱</div>'
    +   '<div class="ab-ver">' + abEsc(ver || '版本未知') + '</div>'
    +   '<p class="ab-desc">Windows 清理、优化、启动项、磁盘空间、硬件信息与系统修复工具。全部功能在本机执行；本分支不包含自动更新与联网上报。</p>'
    + '</div>'
    + '<div class="card">'
    +   '<div class="ab-t">' + ico('info', 15) + '软件信息</div>'
    +   abRow('版本', abEsc(ver || '未知'))
    +   abRow('构建日期', abEsc(bd))
    +   abRow('许可协议', abEsc(a.license || 'GPL-3.0'))
    +   abRow('运行架构', abEsc('Windows 10/11 · amd64'))
    + '</div>'
    + '<div class="card">'
    +   '<div class="ab-t">' + ico('shield', 15) + '文件校验</div>'
    +   '<p class="ab-desc" style="margin-top:6px">当前运行文件的 SHA256。可自行与可信渠道公布的指纹比对；本值只在本机计算，不会上传。</p>'
    +   (hash ? '<div class="ab-hash">' + abEsc(hash) + '</div>'
           + '<button class="btn sm" data-abact="copyhash">' + ico('copy', 13) + '复制 SHA256</button>'
           : '<div class="empty-d">读取失败</div>')
    + '</div>'
    + '<div class="card"><div class="empty-d" style="line-height:1.8">'
    +   '本项目基于 GPLv3 发布，保留原作者版权声明；本分支由二次开发维护者独立维护，与原作者及其服务无关。'
    + '</div></div>'
    + '</div>';
}
window.renderAboutPane = renderAboutPane;

function abClick(e){
  var b = e && e.target && e.target.closest ? e.target.closest('[data-abact]') : null;
  if(!b) return;
  var act = b.dataset.abact;
  if(act === 'retry'){ loadAbout(); return; }
  if(act === 'copyhash'){
    var h = (AB.info || {}).selfHash || '';
    if(!h){ toast('没有可复制的 SHA256', 'warn'); return; }
    if(!hasGo()){ toast('后端未就绪，无法写入剪贴板', 'warn'); return; }
    callGo('CopyText', h).then(function(msg){ toast(msg ? ('复制失败：' + msg) : 'SHA256 已复制', msg ? 'danger' : 'ok'); });
  }
}
window.abClick = abClick;
