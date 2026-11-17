/* ════════════════════════════════════════════════════════════════════════════
   设置面板 · 独立模块（design/setpanel.js + design/setpanel.css）
   ---------------------------------------------------------------------------
   2026-10-13 从 ui-prototype.html 里分出来，跟 pcinfo / opt / uninst / disk /
   startup / driver / tools / bsod / svc / about 一个路子：
     · 主文件只留入口，**同步 XHR + 直接 eval** 载入本文件（外链 <script src>
       是异步的，会晚于主文件内联脚本执行 → 「点齿轮 openSettings 还没定义」）；
     · 本文件里的 `let` / `const` 只在 eval 作用域内，主文件够不着，也污染不了；
     · 需要被主文件访问的，一律显式挂 window（见文件末尾的 window.SetPanel）。

   结构：左侧一列卡片式 tab（通用 / 更新与校验 / 改动记录 / 反馈 / 关于），
        右侧面板**高度固定**，内容超出只在面板内滚 —— 切 tab 时弹窗不跳高。

   🔴 依赖（来自主文件，都是顶层 function 声明，eval 后沿作用域链可达）：
     ico / escHome / callGo / toast / paintIcons / closeModal / switchTheme /
     setMotion / motionOff / go / SET / OPLOG / oplogState / SETLOG_OPEN /
     loadOpLog / opSuIdByName / opUndoStartup / askThenDo
   🔴 主文件反向调用只走 window.SetPanel 这个门面，别直接摸这里的函数名。
   ════════════════════════════════════════════════════════════════════════════ */

/* ════════ 设置弹窗 · 左导航分面（2026-10-13 三改：LightC 那种切着看）════════
   入口还是顶栏齿轮 / 命令面板，点开一个窗，关掉回原来那页。
   类目 = 软件里实际有的五块：通用 / 更新与校验 / 改动记录 / 反馈 / 关于。
   ⚠️ 面板一律单列竖排 —— 之前「左设置 + 右信息」两栏在 1080px 以下会塌成一列，
      塌完「本软件信息」被挤到最底下，1020×660 的机器上一屏看不到，非滚不可。
      拆成五面后每一面都矮了一半以上，也不用再去调那个 grid-template-columns。 */
const SET_TABS = [
  {id:'general',  n:'通用',       ico:'sliders'},
  {id:'update',   n:'更新与校验', ico:'bolt'},
  {id:'log',      n:'改动记录',   ico:'clock'},
  {id:'feedback', n:'反馈',       ico:'message'},
  {id:'about',    n:'关于',       ico:'info'}
];
let SET_TAB = 'general';

/* 面板共用的零件。五个面板必须用同一份 ——
   分开抄的话，switch 的 data-set 就会有两种写法，正是以前被骂「假开关」的起因。 */
function setSw(key, state, reading, label){
  if(reading) return '<div class="switch dis" aria-busy="true" title="正在读取"></div>';
  return '<div class="switch'+(state?' on':'')+'" data-set="'+key+'" role="switch" tabindex="0"'
    + ' aria-checked="'+(state?'true':'false')+'" aria-label="'+label+'"></div>';
}
function setRow(t, d, a){
  return '<div class="set-row"><div class="set-l"><div class="set-l-t">'+t+'</div>'
    + (d ? '<div class="set-l-d">'+d+'</div>' : '') + '</div><div class="set-a">'+a+'</div></div>';
}
function setKv(k, v){
  return '<div class="set-kv"><span class="set-kv-k">'+k+'</span><span class="set-kv-v">'+v+'</span></div>';
}
function setGrp(t){ return '<div class="set-grp">'+t+'</div>'; }

function settingsBody(){
  if(SET_TAB === 'update')   return setPaneUpdate();
  if(SET_TAB === 'log')      return setPaneLog();
  if(SET_TAB === 'feedback') return setPaneFeedback();
  if(SET_TAB === 'about')    return setPaneAbout();
  return setPaneGeneral();
}

/* ── 面板 1：通用（外观 + 启动）── */
function setPaneGeneral(){
  const on = SET.autoStart;
  const dark = document.documentElement.dataset.theme === 'dark';
  return '<div class="card setcard">'
    +   setGrp('外观')
    +   setRow('深色模式', '夜里看着不刺眼。和顶栏那个小太阳是同一个开关', setSw('dark', dark, false, '深色模式'))
    +   setRow('减少动效', '关掉入场与过渡动画，界面更干脆', setSw('motion', motionOff(), false, '减少动效'))
    +   setGrp('启动')
    +   setRow('开机自动启动本软件',
              on === null ? '正在读取…' : (on ? '已开启：开机后自动运行' : '未开启'),
              setSw('auto', on === true, on === null, '开机自动启动'))
    + '</div>';
}

/* ── 面板 2：版本与校验（本分支不联网更新）── */
function setPaneUpdate(){
  const elevTxt = SET.elev === null ? '未获取到' : (SET.elev ? '管理员' : '普通用户');
  const verTxt = SET.ver ? verLabel(SET.ver) : '版本未知';
  const hashShort = SET.hash
    ? (SET.hash.length > 34 ? (SET.hash.slice(0, 24) + '…' + SET.hash.slice(-8)) : SET.hash)
    : '';
  return '<div class="card setcard">'
    +   setGrp('软件更新')
    +   '<div class="set-note">本分支已停用在线更新与官方指纹校验，不会连接任何更新服务器。需要新版本时，请从你信任的分发渠道获取并自行核对 SHA256。</div>'
    + '</div>'
    + '<div class="card setcard" style="margin-top:12px">'
    +   setGrp('本软件')
    +   setKv('版本', verTxt + (SET.build && SET.build !== 'unknown' ? '<small>'+escHome(SET.build)+'</small>' : ''))
    +   setKv('运行权限', elevTxt)
    +   (SET.elev === false
        ? '<div class="set-note">现在不是管理员，系统垃圾清理、服务与启动项管理会失败。要用这些功能，关掉本程序后右键选「以管理员身份运行」。</div>'
        : '')
    +   setGrp('当前文件 SHA256（本地计算）')
    +   '<div class="set-kv wide" style="display:block">'
    +     '<div class="set-hash"'+(SET.hash ? ' title="'+escHome(SET.hash)+'"' : '')+'>'+(hashShort || '读取中…')+'</div>'
    +     '<div class="btn-row" style="margin-top:8px">'
    +       '<button class="btn sm" id="setHashCopy">复制摘要</button>'
    +     '</div>'
    +   '</div>'
    + '</div>';
}

/* ── 面板 3：改动记录（最近改动 + 忘掉历史这两件事）── */
function setPaneLog(){
  const kindName = {optimize:'系统优化', revert:'撤销优化', startup:'启动项', tool:'工具', clean:'垃圾清理'};
  /* 一条改动记录。竖排 —— 横排会挤成一锅粥（这一页现在是单列，宽度够，照样竖排读得顺）。 */
  const logRow = function(o, compact){
    const canUndo = (o.kind === 'startup') && (o.items || []).map(opSuIdByName).filter(function(x){ return !!x; }).length;
    return '<div class="row" style="display:block;padding:8px 6px">'
      + '<div style="display:flex;align-items:center;justify-content:space-between;gap:8px">'
      +   '<span style="color:var(--tx3);font-size:11.5px">'+escHome(o.ts)+'</span>'
      +   '<span style="display:flex;gap:5px">'
      +     '<span class="chip">'+(kindName[o.kind] || o.kind || '操作')+'</span>'
      +     (o.reboot ? '<span class="chip warn">需重启</span>' : '')
      +     (o.fail ? ('<span class="chip warn">失败 '+escHome(String(o.fail))+'</span>') : '')
      +   '</span>'
      + '</div>'
      + '<div class="row-t" style="display:block;margin-top:4px">'+escHome(o.title || '')+'</div>'
      + (!compact && o.items && o.items.length ? ('<div class="row-d" style="font-size:11.5px;color:var(--tx3);white-space:normal">'+escHome(o.items.slice(0, 6).join('、'))+(o.items.length > 6 ? (' …共 '+o.items.length+' 项') : '')+'</div>') : '')
      + (!compact && o.err ? ('<div class="row-d" style="font-size:11.5px;color:var(--warn);white-space:normal">'+escHome(o.err)+'</div>') : '')
      + (canUndo ? ('<button class="btn sm" style="margin-top:7px" data-opundo="'+escHome(o.ts)+'">'+ico('refresh',14)+'把它们开回来</button>') : '')
      + '</div>';
  };
  const logBody = oplogState === 'loading'
    ? '<div class="set-note">正在读取…</div>'
    : (OPLOG.length
        ? (SETLOG_OPEN ? OPLOG.map(function(o){ return logRow(o, false); }).join('') : logRow(OPLOG[0], true))
        : '<div class="set-note">还没有操作记录。做完一次优化或调整启动项后，这里会留下可追溯的记录。</div>');

  return '<div class="card setcard">'
    + '<div class="set-fold" id="setLogFold">'
    +   '<span><span class="set-fold-t">最近改动</span>'
    +     '<span class="set-fold-n">'+(oplogState === 'loading' ? '读取中…' : (OPLOG.length ? (OPLOG.length + ' 条') : '还没有记录'))+'</span></span>'
    +   '<span style="display:flex;align-items:center;gap:10px">'
    +     (SETLOG_OPEN ? '<button class="btn sm" id="opReload">'+ico('refresh',13)+'刷新</button>' : '')
    +     '<span class="set-fold-x">'+(SETLOG_OPEN ? '收起' : '展开全部')+'</span>'
    +   '</span>'
    + '</div>'
    + '<div class="set-log"'+(SETLOG_OPEN ? '' : ' style="max-height:none"')+'>'+logBody+'</div>'
    + '</div>'
    + '<div class="card setcard" style="margin-top:12px">'
    +   setGrp('清理偏好')
    +   setRow('清空改动留痕', '清掉上面那份记录，清完没法追回',
              '<button class="btn sm" id="setClearLog">清空</button>')
    + '</div>';
}

/* ── 面板 4：帮助 ──
   本分支不含任何联系方式或外部服务入口；这里只指向软件内的本地帮助页。 */
function setPaneFeedback(){
  return '<div class="card setcard">'
    + setGrp('本机帮助')
    + '<div class="set-note" style="line-height:1.8">'
    +   '软件内已内置按症状组织的自检清单，并可在本机生成诊断报告。'
    +   '所有内容都在本机完成，不会上传。'
    + '</div>'
    + '<div class="btn-row" style="margin-top:10px">'
    +   '<button class="btn primary" data-fbact="svc">' + ico('message', 14) + '打开帮助与诊断</button>'
    + '</div>'
    + '</div>';
}

function fbAct(act){
  if(act === 'svc'){ closeSettings(); if(typeof go === 'function') go('svc'); return; }
}

/* about.js 是同步 XHR + eval 载入的独立模块，够不着主文件用 let 声明的 SET_OPEN
   （let 不挂 window）。这个桥给它一个只读入口：
   数据回来后主文件才知道「该不该顺手把设置窗重画一遍」。 */
window.setIsOpen = function(){ return !!SET_OPEN; };

/* ── 面板 5：关于（三卡版式原样搬进来，复用 about.js 现成的渲染）── */
function setPaneAbout(){
  /* about.js 是同步 XHR + eval 载进来的；它没跟上时（安装包缺文件）要给说法，不能甩空白 */
  if(typeof window.renderAboutPane !== 'function'){
    return '<div class="card"><div class="empty" style="padding:44px 20px">'
      + '<div class="empty-ico" style="color:var(--warn)">'+ico('warn', 28)+'</div>'
      + '<div class="empty-t">关于模块没载入</div>'
      + '<div class="empty-d">没读到 about.js，请检查安装包是否完整</div></div></div>';
  }
  /* 只主动拉一次：数据没到就每次渲染都去拉，拉回来又触发重画 —— 那是重试风暴（也容易递归） */
  if(!SET.about && !window.ABOUT && !SET_ABOUT_ASKED && typeof window.loadAbout === 'function'){
    SET_ABOUT_ASKED = true;
    window.loadAbout();
  }
  return window.renderAboutPane();
}

/* ════════ 设置弹窗本体 ════════ */

/* 左边导航。默认停在「通用」，切过去的那一项高亮 + 一道蓝竖条。
   「更新与校验」在有新版本时点个小点 —— 和信息 sane 那套一致（不弹窗、不打断）。 */
function setNavHtml(){
  return SET_TABS.map(function(t){
    const dot = (t.id === 'update' && UPD.state === 'new' && SET_TAB !== t.id) ? ' dot' : '';
    return '<button class="set-nav-i'+(SET_TAB === t.id ? ' on' : '')+dot+'" data-settab="'+t.id+'">'
      + ico(t.ico, 14) + '<span>'+t.n+'</span></button>';
  }).join('')
  + (SET.ver ? '<div class="set-nav-ver">'+verLabel(SET.ver)+'</div>' : '');
}

function settingsModalHtml(){
  return '<div class="set-box-t">'
    +   '<span class="modal-t" style="margin:0">'+ico('gear',18)+' 设置</span>'
    +   '<span class="set-box-sub">'
    +     (SET.pending
        ? '<span class="set-pend-in">'+ico('warn',13)+'有 '+SET.pending.n+' 项改动要重启才生效</span>'
        : '')
    +   '</span>'
    + '</div>'
    + '<div class="set-shell">'
    +   '<div class="set-nav">'+setNavHtml()+'</div>'
    +   '<div class="set-pane"><div class="set-scroll" id="setScroll">'+settingsBody()+'</div></div>'
    + '</div>'
    + '<div class="modal-f"><button class="btn" id="setClose">关闭</button></div>';
}

/* tab 可选：侧栏「关于」就是 openSettings('about') —— 它不再是一整页，
   点开设置窗直接停在「关于」那一面（2026-10-13）。 */
function openSettings(tab){
  const box = document.getElementById('modalBox'), modal = document.getElementById('modal');
  if(!box || !modal) return;
  if(tab && SET_TABS.some(function(t){ return t.id === tab; })) SET_TAB = tab;
  SET_OPEN = true;
  SET_ASKING = false;   /* 上次要是卡在"问话中"没复位，这里兜一下，否则窗怎么画都画不出来 */
  box.classList.add('setbox');
  box.style.width = '';   /* 看二维码 / 确认框会把宽度改成小的，回到设置要还原 */
  paintSettings();
  modal.classList.add('open');
  /* 数据是启动时预热的；万一那次没读到（后端还没就绪），开窗时补一次 */
  if(!SET.ver) loadSettings();
}

let SET_ASKING = false;   /* 设置窗里正开着确认框：#modalBox 这会儿装的是确认框，别去覆盖它 */

/* 🔴 2026-10-13 防重入：paintSettings → 面板 → loadAbout（没后端时**同步**回调）
   → about.js 的 abAfter() → 又 paintSettings → 无限递归（Maximum call stack size exceeded）。
   在 exe 里 loadAbout 是异步的不会撞，但浏览器直开 / 后端没就绪时必炸。
   渲染期间再进来的一律丢掉 —— 反正外面这次画完就是最新的。 */
let SET_PAINTING = false;
let SET_ABOUT_ASKED = false;   /* 设置窗已经主动拉过一次关于数据了，别每次重画都拉 */
function paintSettings(){
  const box = document.getElementById('modalBox');
  if(!box || !SET_OPEN) return;
  if(SET_PAINTING) return;
  /* 🔴 2026-10-13：设置弹窗里的「重置 / 清空」会弹确认框（同一个容器），
     而设置的数据是异步拉回来的 —— 确认框开着的时候正好有个回调返回，
     就会把确认框直接冲成设置内容，用户刚点的"确定"就这么没了（探针实测撞到过）。
     正在问话就一律不重画。 */
  if(SET_ASKING) return;
  SET_PAINTING = true;
  try{
    box.innerHTML = settingsModalHtml();
    box.classList.add('setbox');
    paintIcons(box);
    bindSettings(box);
  } finally { SET_PAINTING = false; }
  /* 内容装不下时给底部一道淡出（同清理确认窗的做法） */
  const sc = document.getElementById('setScroll');
  if(sc) sc.classList.toggle('set-more', sc.scrollHeight - sc.clientHeight > 4);
}

function closeSettings(){
  SET_OPEN = false;
  const box = document.getElementById('modalBox');
  if(box) box.classList.remove('setbox');
  closeModal();
}

/* 设置弹窗里的交互。以前这些绑在 bindPage 里（那时候设置是整页），
   现在弹窗内容每次重绘都是新节点，所以在 paintSettings 之后再绑一次。 */
function bindSettings(root){
  const q = function(sel){ return (root || document).querySelector(sel); };
  const qa = function(sel){ return (root || document).querySelectorAll(sel); };
  const close = q('#setClose');
  if(close) close.onclick = closeSettings;

  /* ── 左导航：切分类只换右边那块 ──
     整窗重画（paintSettings）而不是局部替换，是为了让 #setScroll 的滚动和绑定一起重建；
     差不到一个 repaint 的钱，别为了省这点再去维护两套节点。 */
  qa('[data-settab]').forEach(function(b){
    b.onclick = function(){
      const id = b.dataset.settab;
      if(id === SET_TAB) return;
      SET_TAB = id;
      /* 第一次翻到「关于」才去读 —— 关于 / 反馈共用同一份 GetAboutInfo */
      if((id === 'about' || id === 'feedback') && !SET.about && typeof window.loadAbout === 'function') window.loadAbout();
      paintSettings();
    };
  });

  /* ── 反馈面板：全是真链接 / 真号码，没有一个写死 ── */
  qa('[data-fbact]').forEach(function(b){ b.onclick = function(){ fbAct(b.dataset.fbact); }; });

  /* ── 关于面板：直接用 about.js 那套点击处理（渲染也是它的，行为必须一致）── */
  qa('[data-abact]').forEach(function(b){
    b.onclick = function(){
      if(typeof window.abClick === 'function') window.abClick({target: b});
      else if(b.dataset.abact === 'retry' && typeof window.loadAbout === 'function') window.loadAbout();
    };
  });

  qa('.switch[data-set]').forEach(function(sw){
    const act = function(){
      const k = sw.dataset.set;
      if(k === 'dark'){
        switchTheme(document.documentElement.dataset.theme === 'dark' ? 'light' : 'dark');
        /* switchTheme 走 startViewTransition 时换肤回调是下一帧才跑的，
           立刻重画读到的是旧主题 —— 等一帧再画，开关状态才跟得上 */
        setTimeout(function(){ if(SET_OPEN) paintSettings(); }, 60);
        return;
      }
      if(k === 'motion'){
        setMotion(!motionOff());
        if(SET_OPEN) paintSettings();
        toast(motionOff() ? '已减少动效' : '已恢复动效', 'ok');
        return;
      }
      if(k === 'auto'){
        if(SET.autoStart === null){ toast('状态还没读到，稍等一下再点', 'warn'); return; }
        sw.classList.add('dis');
        const to = !(SET.autoStart === true);
        callGo('SetAutoStart', to).then(function(msg){
          if(msg){ toast(msg, 'danger'); sw.classList.remove('dis'); return; }
          SET.autoStart = to;
          toast(to ? '已开启开机自启' : '已关闭开机自启', 'ok');
          if(SET_OPEN) paintSettings();
        }).catch(function(e){ toast('设置失败：' + e, 'danger'); sw.classList.remove('dis'); });
        return;
      }
    };
    sw.addEventListener('click', act);
    sw.addEventListener('keydown', function(e){
      if(e.key === 'Enter' || e.key === ' '){ e.preventDefault(); act(); }
    });
  });

  const hc = q('#setHashCopy');
  if(hc) hc.onclick = function(){
    if(!SET.hash){ toast('摘要还没读到', 'warn'); return; }
    callGo('CopyText', SET.hash).then(function(msg){ toast(msg ? ('复制失败：' + msg) : '摘要已复制', msg ? 'danger' : 'ok'); });
  };

  const cl = q('#setClearLog');
  if(cl) cl.onclick = function(){
    if(!OPLOG.length){ toast('没有记录可清', 'info'); return; }
    askThenDo('清空改动留痕？',
      '下面那份「最近改动」会被清空，清完<b>没法追回</b>。'
      + '<br><br>已经做过的优化、已经关掉的自启项不会跟着变回去，只是记录没了。',
      '清空', function(){
        callGo('ClearOpLog').then(function(ok){ toast(ok ? '改动留痕已清空' : '清空失败', ok ? 'ok' : 'danger'); loadOpLog(); })
          .catch(function(e){ toast('清空失败：' + e, 'danger'); });
      });
  };
  const lf = q('#setLogFold');
  if(lf) lf.onclick = function(e){
    if(e.target.closest('#opReload')) return;   /* 「刷新」是条上的按钮，点它不算折叠 */
    SETLOG_OPEN = !SETLOG_OPEN;
    if(SET_OPEN) paintSettings();
  };
  const opR = q('#opReload');
  if(opR) opR.onclick = function(){ toast('正在刷新操作日志…', 'info'); loadOpLog(); };
  (root || document).querySelectorAll('[data-opundo]').forEach(function(b){
    b.onclick = function(){ opUndoStartup(b.dataset.opundo); };
  });
}
/* ════════ 对主文件的门面（2026-10-13 拆模块时立）════════
   主文件（顶栏齿轮 / 命令面板 / F5 / update:progress 回调）只认 window.SetPanel，
   不认本文件里的任何函数名 —— 这样以后在模块里改名、重排，主文件一行都不用动。 */
window.SetPanel = {
  open:    function(tab){ openSettings(tab); },
  close:   function(){ closeSettings(); },
  isOpen:  function(){ return !!SET_OPEN; },
  refresh: function(){ if(SET_OPEN) paintSettings(); },
  /* 下载进度：后端约每 150ms 推一次。这里只改进度条那两个节点，不整窗重画 ——
     重画会让进度条每次从头开始闪。 */
  onProgress: function(done, total){
    if(UPD.state !== 'downloading') return;
    UPD.prog = {done: done || 0, total: total || 0};
    const fill = document.getElementById('updBarFill'), meta = document.getElementById('updMeta');
    const pct = total ? Math.min(100, Math.round(done / total * 100)) : 0;
    if(fill) fill.style.width = pct + '%';
    if(meta) meta.textContent = fmtSize(done) + (total ? (' / ' + fmtSize(total) + ' \u00b7 ' + pct + '%') : '');
  }
};
window.openSettings = openSettings;
window.paintSettings = paintSettings;
window.closeSettings = closeSettings;
/* about.js 也是独立模块，够不着这里的 let SET_OPEN —— 给它一个只读入口：
   数据回来后它才知道「该不该顺手把设置窗重画一遍」。 */
window.setIsOpen = function(){ return !!SET_OPEN; };
