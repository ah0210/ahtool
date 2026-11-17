/* ══════════════════════════════════════════════════════════════════════════
   帮助与诊断 · 独立模块（二次开发版）
   ---------------------------------------------------------------------------
   原「服务大厅」是原作者的付费远程服务转化页。本分支已解除该绑定：
   - 不展示任何官网 / 社群 / 微信 / 赞赏 / 远程服务入口；
   - 只保留本地自检清单与本机诊断报告；
   - 诊断报告只在本机生成，由用户自行复制，不上传。
   ══════════════════════════════════════════════════════════════════════════ */

var SV = {state:'loading', err:'', guides:[], report:'', busy:false};
window.SV = SV;
window.SVC_LINKS = {};

function svEsc(s){
  return String(s == null ? '' : s).replace(/[&<>"]/g, function(c){
    return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c];
  });
}
function svOnPage(){ return !!document.querySelector('[data-svcpage]'); }
function svAfter(){ if(svOnPage() && typeof render === 'function') render(); }

function loadSvc(){
  if(!hasGo()){
    if(inWails()){ SV.state='failed'; SV.err='后端连接还没就绪（window.go 为空）'; }
    svAfter();
    return;
  }
  SV.state = 'loading';
  callGo('GetGuides').then(function(list){
    SV.guides = list || [];
    SV.state = 'ready';
    svAfter();
  }).catch(function(e){
    SV.state = 'failed'; SV.err = String(e);
    svAfter();
  });
}
window.loadSvc = loadSvc;

function renderSvc(){
  var head = '<div class="page-head"><div><h1 class="page-title">帮助与诊断</h1>'
    + '<div class="page-sub">按症状自查，或生成一份纯本机诊断报告。所有操作都在本机完成，不上传。</div></div>'
    + '<div class="page-actions"><button class="btn primary" id="svcDiagBtn">' + ico('doc',16) + '生成诊断报告</button></div></div>';

  if(SV.state === 'failed'){
    return head + '<div class="card"><div class="empty" style="padding:40px 20px">'
      + '<div class="empty-ico" style="color:var(--danger)">' + ico('warn',28) + '</div>'
      + '<div class="empty-t">自检清单读取失败</div><div class="empty-d">' + svEsc(SV.err) + '</div>'
      + '<div style="display:flex;justify-content:center;margin-top:14px"><button class="btn primary" id="svcRetry">' + ico('refresh',16) + '重新读取</button></div></div></div>';
  }
  if(SV.state !== 'ready'){
    return head + '<div class="card"><div class="empty" style="padding:48px 20px">'
      + '<div class="empty-ico" style="color:var(--info)">' + ico('refresh',28) + '</div>'
      + '<div class="empty-t">正在读取自检清单…</div></div></div>';
  }

  var html = head + '<div class="guide-grid">';
  SV.guides.forEach(function(g, i){
    html += '<div class="card guide-card">'
      + '<div class="guide-t">' + svEsc(g.title || '未命名') + '</div>'
      + '<ol class="guide-steps">';
    (g.steps || []).forEach(function(s){ html += '<li>' + svEsc(s) + '</li>'; });
    html += '</ol>';
    if(g.toolId || g.pageId){
      html += '<div class="guide-act"><button class="btn sm" data-svcact="go" data-tool="' + svEsc(g.toolId || '') + '" data-page="' + svEsc(g.pageId || '') + '">'
        + svEsc(g.actLabel || '执行') + '</button></div>';
    }
    html += '</div>';
  });
  html += '</div>';
  return html;
}
window.renderSvc = renderSvc;

function svOpenDiag(){
  var modal = document.getElementById('modal');
  var box = document.getElementById('modalBox');
  if(!modal || !box) return;
  box.innerHTML = '<div class="modal-t">' + ico('doc',18) + ' 本机诊断报告</div>'
    + '<div class="modal-d">只汇总本机信息（系统 / 硬件 / 磁盘 / 蓝屏线索），不会自动上传。生成后可自行复制。</div>'
    + '<div class="svc-out" id="svcOut" style="max-height:46vh;overflow:auto;white-space:pre-wrap;font-size:12px;line-height:1.6;margin-top:10px;padding:10px;border:1px solid var(--stroke);border-radius:10px">正在采集…</div>'
    + '<div class="modal-f"><button class="btn" id="svcCopy">' + ico('copy',14) + '复制报告</button>'
    + '<button class="btn primary" onclick="closeModal()">关闭</button></div>';
  modal.classList.add('open');
  svcGenReport();
}

function svcGenReport(){
  var out = document.getElementById('svcOut');
  if(!out) return;
  if(!hasGo()){ out.textContent = '后端未就绪，无法生成报告。'; return; }
  SV.busy = true;
  callGo('BuildDiagReport', '').then(function(rep){
    SV.busy = false;
    SV.report = rep || '';
    out.textContent = SV.report || '报告为空';
  }).catch(function(e){
    SV.busy = false;
    out.textContent = '采集失败：' + e;
  });
}

function svcDiagRun(){ if(!document.getElementById('modal').classList.contains('open')) svOpenDiag(); }
window.svcDiagRun = svcDiagRun;

function bindSvc(){
  var b = document.getElementById('svcDiagBtn');
  if(b) b.onclick = svOpenDiag;
  var r = document.getElementById('svcRetry');
  if(r) r.onclick = loadSvc;
  var cp = document.getElementById('svcCopy');
  if(cp) cp.onclick = function(){
    if(!SV.report){ toast('报告还没生成完', 'warn'); return; }
    callGo('CopyText', SV.report).then(function(msg){ toast(msg ? ('复制失败：'+msg) : '诊断报告已复制', msg ? 'danger' : 'ok'); });
  };
  var grid = document.querySelector('.guide-grid');
  if(grid) grid.onclick = function(e){
    var btn = e.target.closest('[data-svcact="go"]');
    if(!btn) return;
    var tool = btn.dataset.tool, page = btn.dataset.page;
    if(tool && typeof go === 'function'){
      go('tools');
      setTimeout(function(){ if(typeof tkRun === 'function') tkRun(tool, ''); }, 300);
      return;
    }
    if(page && typeof go === 'function'){ go(page); }
  };
}
window.bindSvc = bindSvc;
