package main

import "net/http"

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

const indexHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>home-broadband</title>
<script>
/* 主题先行：localStorage 里有手动选择就用，没有就跟随系统（CSS media query 处理） */
try{var _t=localStorage.getItem('hb-theme');
if(_t==='dark'||_t==='light')document.documentElement.setAttribute('data-theme',_t);}catch(e){}
</script>
<style>
/* 全局兜底：hidden 属性必须真的藏住，不被后面的 display 规则覆盖 */
[hidden]{display:none!important}
/* ===== 主题：默认深色；浅色可手动选，或跟随系统 ===== */
:root{
  color-scheme:dark;
  --bg:#0e1116;
  --side:#131922;
  --card:#161c26;
  --card2:#1b2230;
  --field:#0e1116;
  --border:#242e3d;
  --text:#e9ecf2;
  --dim:#8d96a6;
  --accent:#10b981;
  --accent-d:#0b9b6c;
  --accent-soft:rgba(16,185,129,.13);
  --ok:#34d399;
  --warn:#fbbf24;
  --bad:#f87171;
  --shadow:0 1px 2px rgba(0,0,0,.35);
  --overlay:rgba(5,8,12,.6);
  --radius:12px;
}
html[data-theme="light"]{
  color-scheme:light;
  --bg:#f2f4f7;
  --side:#ffffff;
  --card:#ffffff;
  --card2:#f6f8fb;
  --field:#ffffff;
  --border:#e2e6ec;
  --text:#181c24;
  --dim:#687180;
  --accent:#059669;
  --accent-d:#047857;
  --accent-soft:rgba(5,150,105,.1);
  --ok:#059669;
  --warn:#b45309;
  --bad:#dc2626;
  --shadow:0 1px 2px rgba(25,35,55,.09);
  --overlay:rgba(40,50,70,.35);
  --radius:12px;
}
@media (prefers-color-scheme:light){
  html:not([data-theme]){
    color-scheme:light;
    --bg:#f2f4f7;
    --side:#ffffff;
    --card:#ffffff;
    --card2:#f6f8fb;
    --field:#ffffff;
    --border:#e2e6ec;
    --text:#181c24;
    --dim:#687180;
    --accent:#059669;
    --accent-d:#047857;
    --accent-soft:rgba(5,150,105,.1);
    --ok:#059669;
    --warn:#b45309;
    --bad:#dc2626;
    --shadow:0 1px 2px rgba(25,35,55,.09);
    --overlay:rgba(40,50,70,.35);
    --radius:12px;
  }
}
*{box-sizing:border-box}
html{-webkit-text-size-adjust:100%}
body{margin:0;min-height:100vh;background:var(--bg);color:var(--text);
  font:14px/1.6 -apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Hiragino Sans GB","Microsoft YaHei",sans-serif;
  -webkit-font-smoothing:antialiased}
.mono{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-variant-numeric:tabular-nums}
.dim{color:var(--dim)}
.small{font-size:12px}
.spacer{flex:1}
::-webkit-scrollbar{width:10px;height:10px}
::-webkit-scrollbar-thumb{background:var(--border);border-radius:8px;border:2px solid transparent;background-clip:content-box}
::-webkit-scrollbar-track{background:transparent}

/* ===== 布局 ===== */
.app{display:flex;min-height:100vh}
.side{width:226px;flex:none;background:var(--side);border-right:1px solid var(--border);
  display:flex;flex-direction:column;position:sticky;top:0;height:100vh;z-index:30}
.brand{display:flex;align-items:center;gap:10px;padding:18px 16px;font-weight:700;font-size:15px;letter-spacing:.2px}
.logo{width:32px;height:32px;flex:none;border-radius:10px;color:#fff;display:flex;align-items:center;justify-content:center;
  background:linear-gradient(135deg,var(--accent),#0ea5e9);box-shadow:0 4px 12px rgba(16,185,129,.35)}
.logo svg{width:18px;height:18px}
.nav{padding:6px 10px;display:flex;flex-direction:column;gap:4px}
.nav button{display:flex;align-items:center;gap:11px;width:100%;padding:10px 12px;border:0;border-radius:10px;
  background:transparent;color:var(--dim);font-size:14px;cursor:pointer;text-align:left;transition:background .12s,color .12s}
.nav button:hover{background:var(--card);color:var(--text)}
.nav button.on{background:var(--accent-soft);color:var(--accent);font-weight:600}
.nav button svg{width:17px;height:17px;flex:none}
.badge{margin-left:auto;font-size:11px;font-weight:600;background:var(--card2);border:1px solid var(--border);
  color:var(--dim);border-radius:20px;padding:1px 9px;font-variant-numeric:tabular-nums}
.badge:empty{display:none}
.nav button.on .badge{background:var(--accent);border-color:transparent;color:#fff}
.side-foot{margin-top:auto;padding:12px;border-top:1px solid var(--border);display:flex;align-items:center;gap:6px}
.main{flex:1;min-width:0;display:flex;flex-direction:column}
.topbar{display:flex;align-items:center;gap:12px;padding:15px 28px;border-bottom:1px solid var(--border);
  position:sticky;top:0;background:var(--bg);z-index:20}
.topbar h1{font-size:20px;margin:0;font-weight:700;letter-spacing:.2px}
.content{padding:24px 28px 60px;max-width:1140px;width:100%;margin:0 auto}
/* 视图切换动效：从 hidden 切回来时动画重播 */
.view{animation:viewIn .22s ease}
@keyframes viewIn{from{opacity:0;transform:translateY(8px)}to{opacity:1;transform:none}}
@media (prefers-reduced-motion:reduce){.view{animation:none}}

/* ===== 通用控件 ===== */
button{font:inherit;font-size:13px;font-weight:500;color:var(--text);background:var(--card);
  border:1px solid var(--border);border-radius:10px;padding:7px 14px;cursor:pointer;
  display:inline-flex;align-items:center;gap:7px;white-space:nowrap;box-shadow:var(--shadow);
  transition:border-color .14s,color .14s,background .14s,transform .07s}
button:hover:not(:disabled){border-color:var(--accent);color:var(--accent)}
button:active:not(:disabled){transform:translateY(1px)}
button:disabled{opacity:.45;cursor:default}
button.primary{background:var(--accent);border-color:transparent;color:#fff;font-weight:600;box-shadow:0 4px 14px rgba(16,185,129,.3)}
button.primary:hover:not(:disabled){background:var(--accent-d);border-color:transparent;color:#fff}
.iconbtn{padding:7px;border-radius:9px;background:transparent;border:1px solid transparent;
  color:var(--dim);box-shadow:none;display:inline-flex;align-items:center;justify-content:center}
.iconbtn:hover:not(:disabled){color:var(--accent);background:var(--accent-soft);border-color:transparent}
.iconbtn.danger:hover:not(:disabled){color:var(--bad);background:rgba(248,113,113,.1)}
.menuwrap{position:relative}
.menu{position:absolute;top:calc(100% + 8px);right:0;min-width:172px;background:var(--card);
  border:1px solid var(--border);border-radius:12px;box-shadow:0 14px 36px rgba(0,0,0,.28);
  padding:6px;z-index:60}
.menu[hidden]{display:none}
.menu button{display:flex;width:100%;align-items:center;gap:10px;border:0;background:transparent;
  box-shadow:none;padding:9px 12px;font-size:13px;font-weight:500;color:var(--text);border-radius:8px}
.menu button:hover{background:var(--card2);color:var(--text);border-color:transparent}
.menu button .check{margin-left:auto;color:var(--accent);font-weight:700;visibility:hidden}
.ghlink{display:flex;align-items:center;gap:10px;width:100%;padding:9px 12px;border-radius:10px;
  color:var(--dim);text-decoration:none;font-size:13px}
.ghlink:hover{background:var(--card);color:var(--text)}
.ghlink svg{width:16px;height:16px;flex:none}
svg{width:15px;height:15px;stroke:currentColor;fill:none;stroke-width:1.8;stroke-linecap:round;stroke-linejoin:round;flex:none}
select,input[type=search],input[type=text],input[type=password]{font:inherit;font-size:13px;background:var(--field);
  border:1px solid var(--border);color:var(--text);border-radius:10px;padding:7px 11px;width:100%}
select:focus,input[type=search]:focus,input[type=text]:focus,input[type=password]:focus{outline:none;border-color:var(--accent);box-shadow:0 0 0 3px var(--accent-soft)}
textarea{width:100%;min-height:300px;background:var(--field);border:1px solid var(--border);color:var(--text);
  border-radius:10px;font:12px/1.8 ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;padding:10px 12px;resize:vertical}
textarea:focus{outline:none;border-color:var(--accent)}
label.chk{display:flex;align-items:center;gap:8px;font-size:13px;cursor:pointer;margin:0}
label.chk input{margin:0;accent-color:var(--accent);width:15px;height:15px}

/* ===== 仪表盘 ===== */
.stats{display:grid;grid-template-columns:repeat(4,1fr);gap:14px;margin-bottom:28px}
.stat{background:var(--card);border:1px solid var(--border);border-radius:var(--radius);padding:18px 20px;box-shadow:var(--shadow)}
.stat .num{font-size:28px;font-weight:700;font-variant-numeric:tabular-nums;letter-spacing:-.5px}
.stat .num.small{font-size:16px;padding:6px 0}
.stat .lbl{color:var(--dim);font-size:12px;margin-top:6px}
.secttl{font-size:14px;font-weight:700;margin:0 0 12px;display:flex;align-items:center;gap:8px}
.secttl .count{font-weight:400}

/* ===== 工具栏 ===== */
.toolbar{display:flex;align-items:center;gap:10px;margin-bottom:16px;flex-wrap:wrap}
.toolbar h2{font-size:16px;margin:0;font-weight:700;display:flex;align-items:center;gap:8px}

/* ===== 出口卡片 ===== */
.exit{background:var(--card);border:1px solid var(--border);border-radius:var(--radius);
  padding:14px 18px;margin-bottom:12px;box-shadow:var(--shadow);transition:border-color .14s}
.exit:hover{border-color:var(--accent)}
.etop{display:flex;align-items:center;gap:12px}
.dot{width:9px;height:9px;flex:none;border-radius:50%;background:var(--dim);box-shadow:0 0 8px currentColor}
.dot.up{background:var(--ok);color:var(--ok)}
.dot.starting{background:var(--warn);color:var(--warn);animation:pulse 1.2s ease-in-out infinite}
.dot.failed{background:var(--bad);color:var(--bad)}
.dot.stopped{background:var(--dim);color:transparent;box-shadow:none}
@keyframes pulse{0%,100%{opacity:1}50%{opacity:.3}}
.ip{font-size:16px;font-weight:700;letter-spacing:-.3px}
.meta{color:var(--dim);font-size:12px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.ebot{display:flex;align-items:flex-start;gap:10px;margin-top:12px;padding-top:12px;border-top:1px solid var(--border)}
.ebot .lbl{font-size:11px;color:var(--dim);padding-top:5px;flex:none}
.socksbtn{background:var(--card2);font-variant-numeric:tabular-nums}
.errline{padding:10px 0 0;color:var(--bad);font-size:12px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}

/* ===== chips ===== */
.chips{display:flex;gap:8px;flex-wrap:wrap}
.chip{border:1px solid var(--border);border-radius:8px;padding:4px 10px;font-size:12px;color:var(--dim);
  cursor:pointer;background:var(--card2);box-shadow:none;font-weight:500}
.chip span{opacity:.75;font-family:ui-monospace,Menlo,Consolas,monospace}
.chip:hover{border-color:var(--accent);color:var(--accent)}
.chip.none{border-style:dashed;cursor:default}
.chip.none:hover{border-color:var(--border);color:var(--dim)}

/* ===== 节点视图 ===== */
.card{background:var(--card);border:1px solid var(--border);border-radius:var(--radius);
  padding:14px 18px;margin-bottom:12px;box-shadow:var(--shadow)}
.card.narrow{max-width:720px;padding:22px 24px}
.formfoot{display:flex;align-items:center;gap:10px;margin-top:22px;padding-top:18px;border-top:1px solid var(--border)}
.nghead{display:flex;align-items:center;gap:10px;margin-bottom:6px}
.nghead b{font-size:15px}
.rgexit{border-top:1px solid var(--border);padding:6px 0 4px;margin-top:6px}
.rgexit:first-of-type{border-top:0;margin-top:0;padding-top:0}
.rgexit-head{display:flex;align-items:center;gap:9px;padding:6px 0;font-size:13px;font-weight:600}
.rgexit .nrow{margin-left:18px}
.nrow{display:flex;align-items:center;gap:10px;padding:11px 0;border-top:1px solid var(--border)}
.ninfo{display:flex;flex-direction:column;gap:2px;min-width:0}
.ninfo b{font-size:13px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.nrow .acts{display:flex;align-items:center;gap:6px;flex:none}
details.adv{border:1px solid var(--border);border-radius:12px;padding:0 14px;margin-top:14px}
details.adv summary{cursor:pointer;padding:12px 2px;font-size:13px;font-weight:600;color:var(--dim);
  list-style:none;display:flex;align-items:center;gap:8px}
details.adv summary::-webkit-details-marker{display:none}
details.adv summary::after{content:'›';margin-left:auto;transition:transform .15s;font-size:16px}
details.adv[open] summary::after{transform:rotate(90deg)}
details.adv .f{margin-top:12px}
details.adv .f:last-child{margin-bottom:14px}
.bindlbl{display:flex;align-items:center;gap:8px;font-size:12px;color:var(--dim);flex:none}
.bindlbl select{width:190px}
.count{color:var(--dim);font-size:12px}

/* ===== 任务 ===== */
.job{background:var(--card);border:1px solid var(--border);border-radius:var(--radius);
  box-shadow:var(--shadow);padding:14px 18px;margin-bottom:12px}
.job .top{display:flex;align-items:center;gap:10px;margin-bottom:10px}
.job .top strong{font-weight:600;font-size:13px}
.steps{display:flex;flex-wrap:wrap;gap:8px}
.step{display:flex;align-items:center;gap:6px;font-size:12px;color:var(--dim);
  border:1px solid var(--border);border-radius:8px;padding:4px 10px;background:var(--card2)}
.step.ok{color:var(--ok);border-color:rgba(52,211,153,.45)}
.step.failed{color:var(--bad);border-color:rgba(248,113,113,.45)}
.step.running{color:var(--warn);border-color:rgba(251,191,36,.45)}
.spin{animation:rot 1s linear infinite;transform-origin:center}
@keyframes rot{to{transform:rotate(360deg)}}

/* ===== 空状态 ===== */
.empty{border:1px dashed var(--border);border-radius:var(--radius);padding:52px 20px;text-align:center;background:var(--card)}
.empty-t{font-size:15px;font-weight:600;margin-bottom:6px}

/* ===== 弹窗 ===== */
.modal{position:fixed;inset:0;background:var(--overlay);display:none;align-items:center;justify-content:center;z-index:50;padding:20px}
.modal.open{display:flex}
.sheet{background:var(--card);border:1px solid var(--border);border-radius:16px;
  box-shadow:0 24px 64px rgba(0,0,0,.4);width:min(680px,100%);max-height:88vh;display:flex;flex-direction:column}
.sheet .head{display:flex;align-items:center;gap:10px;padding:14px 18px;border-bottom:1px solid var(--border)}
.sheet .head h2{font-size:15px;margin:0;font-weight:700}
.sheet .body{overflow:auto;padding:18px}
.sheet .foot{display:flex;align-items:center;gap:10px;padding:14px 18px;border-top:1px solid var(--border)}
label.f{display:block;margin-bottom:18px}
label.f[hidden]{display:none}
label.f>span{display:block;color:var(--dim);font-size:12px;margin-bottom:7px;font-weight:500}
.seg{display:grid;grid-template-columns:repeat(3,1fr);gap:8px}
.seg button{border:1px solid var(--border);background:var(--card2);border-radius:12px;
  padding:10px 12px;display:flex;flex-direction:column;gap:4px;align-items:flex-start;
  text-align:left;box-shadow:none;font-weight:500}
.seg button b{font-size:13px}
.seg button em{font-style:normal;font-size:11px;color:var(--dim)}
.seg button.sel{border-color:var(--accent);background:rgba(52,211,153,.08)}
.seg button.sel b{color:var(--accent)}
.regions{display:grid;grid-template-columns:repeat(auto-fill,minmax(150px,1fr));gap:8px;max-height:230px;overflow:auto}
.rg{border:1px solid var(--border);background:var(--card2);border-radius:10px;padding:9px 12px;cursor:pointer;
  text-align:left;display:block;width:100%;box-shadow:none}
.rg:hover{border-color:var(--accent)}
.rg.sel{border-color:var(--accent);background:var(--accent-soft);box-shadow:0 0 0 1px var(--accent)}
.rg b{font-weight:600;font-size:13px;display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.rg em{display:block;font-style:normal;color:var(--dim);font-size:11px;margin-top:3px}
.stepper{display:flex;align-items:center;width:fit-content;border:1px solid var(--border);border-radius:10px;overflow:hidden;background:var(--field)}
.stepper button{border:0;border-radius:0;background:transparent;padding:7px 13px;box-shadow:none}
.stepper button:hover{color:var(--accent)}
.stepper input[type=text]{width:58px;text-align:center;background:transparent;border:0;border-left:1px solid var(--border);
  border-right:1px solid var(--border);border-radius:0;padding:7px 0;font-variant-numeric:tabular-nums;box-shadow:none}
.stepper input:focus{outline:none;box-shadow:none}
.hint{color:var(--dim);font-size:12px;margin-top:7px}
.hint code{background:var(--card2);border:1px solid var(--border);border-radius:6px;padding:0 6px;font-size:11px}
.hint.bad{color:var(--bad)}
.setrow{display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-top:18px}
.updsec{margin-top:20px;padding-top:16px;border-top:1px solid var(--border)}
.updrow{display:flex;align-items:center;gap:10px}
.updver{font-size:13px}
.updver b{font-weight:700}
.updver span{color:var(--dim);margin-left:8px}
.updnotes{margin-top:12px;padding:12px;background:var(--card2);border:1px solid var(--border);border-radius:10px;
  font-size:12px;line-height:1.7;color:var(--dim);white-space:pre-wrap;max-height:180px;overflow:auto}
.kv{display:grid;grid-template-columns:80px 1fr;gap:6px 14px;margin:0 0 16px;font-size:13px}
.kv dt{color:var(--dim)}
.kv dd{margin:0;word-break:break-all}
.share{padding:12px;background:var(--card2);border:1px solid var(--border);border-radius:10px;
  word-break:break-all;font-size:12px;line-height:1.7;margin-bottom:10px;font-family:ui-monospace,Menlo,Consolas,monospace}
.editbar{display:flex;align-items:flex-end;gap:12px;flex-wrap:wrap;padding:14px 0;border-top:1px solid var(--border);margin-top:6px}
.ef{display:block}
.ef>span{display:block;color:var(--dim);font-size:11px;margin-bottom:5px;font-weight:500}
.ef input{width:160px}
.credrow{display:flex;align-items:flex-end;gap:12px;flex-wrap:wrap;margin-bottom:12px}
.credrow .ef input{width:200px}
.chead{display:flex;align-items:center;gap:10px;margin:16px 0 10px;padding-top:14px;border-top:1px solid var(--border)}
.chead h3{font-size:13px;margin:0;font-weight:700}
.client{border:1px solid var(--border);border-radius:10px;padding:10px 14px;margin-bottom:10px;background:var(--card2)}
.crow{display:flex;align-items:center;gap:10px}
.cemail{font-weight:600;font-size:13px}
.cid{color:var(--dim);font-size:11px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;max-width:300px;
  font-family:ui-monospace,Menlo,Consolas,monospace}
.client .share{margin:10px 0 0}
.share button{margin-top:10px}

/* ===== toast ===== */
.toast{position:fixed;left:50%;bottom:26px;transform:translateX(-50%) translateY(8px);background:var(--card);
  border:1px solid var(--border);border-radius:12px;box-shadow:0 12px 32px rgba(0,0,0,.3);
  padding:10px 18px;font-size:13px;z-index:80;opacity:0;pointer-events:none;transition:opacity .18s,transform .18s}
.toast.show{opacity:1;transform:translateX(-50%) translateY(0)}
.toast.bad{border-color:var(--bad);color:var(--bad)}

/* ===== 移动端 ===== */
@media(max-width:900px){
  .app{flex-direction:column}
  .side{width:100%;height:auto;position:sticky;top:0;flex-direction:row;align-items:center;
    border-right:0;border-bottom:1px solid var(--border);padding:0 6px}
  .brand{padding:10px 8px;font-size:13px}
  .brand .bname{display:none}
  .logo{width:28px;height:28px}
  .nav{flex-direction:row;flex:1;padding:6px;gap:2px}
  .nav button{justify-content:center;padding:9px 8px;gap:7px}
  .nav button .nlbl{display:none}
  .badge{margin-left:0}
  .side-foot{border:0;margin:0;padding:6px}
  .side-foot .ghlink{display:none}
  .topbar{padding:12px 16px;flex-wrap:wrap}
  .topbar h1{font-size:17px}
  .content{padding:16px 14px 60px}
  .stats{grid-template-columns:repeat(2,1fr);gap:10px;margin-bottom:20px}
  .stat{padding:14px 16px}
  .stat .num{font-size:22px}
  .etop{flex-wrap:wrap}
  .setrow{grid-template-columns:1fr}
}
</style>
</head>
<body>
<div class="app">
  <aside class="side">
    <div class="brand">
      <span class="logo"><svg viewBox="0 0 24 24" style="stroke:#fff"><circle cx="12" cy="12" r="9"/><path d="M3 12h18"/><path d="M12 3c3.2 3.6 3.2 14.4 0 18"/><path d="M12 3c-3.2 3.6-3.2 14.4 0 18"/></svg></span>
      <span class="bname">home-broadband</span>
    </div>
    <nav class="nav">
      <button data-view="dash" class="on">
        <svg viewBox="0 0 24 24"><rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/></svg>
        <span class="nlbl">总览</span>
      </button>
      <button data-view="exits">
        <svg viewBox="0 0 24 24"><path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4"/><path d="m10 17 5-5-5-5"/><path d="M15 12H3"/></svg>
        <span class="nlbl">出口</span><span class="badge" id="navEcount"></span>
      </button>
      <button data-view="nodes">
        <svg viewBox="0 0 24 24"><path d="M8 6h13"/><path d="M8 12h13"/><path d="M8 18h13"/><path d="M3.5 6h.01"/><path d="M3.5 12h.01"/><path d="M3.5 18h.01"/></svg>
        <span class="nlbl">节点</span><span class="badge" id="navNcount"></span>
      </button>
      <button data-view="settings">
        <svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
        <span class="nlbl">设置</span>
      </button>
    </nav>
    <div class="side-foot">
      <a class="ghlink" href="https://github.com/hao6789/Home-Broadband" target="_blank" rel="noopener" title="GitHub">
        <svg viewBox="0 0 24 24" style="fill:currentColor;stroke:none"><path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12"/></svg>
        <span>GitHub</span>
      </a>
    </div>
  </aside>

  <div class="main">
    <div class="topbar">
      <h1 id="viewTitle">总览</h1>
      <span class="dim small" id="panel"></span>
      <span class="spacer"></span>
      <div class="menuwrap">
        <button class="iconbtn" id="themeBtn" title="主题"></button>
        <div class="menu" id="themeMenu" hidden></div>
      </div>
      <button id="newnode">
        <svg viewBox="0 0 24 24"><path d="M4 7h16"/><path d="M4 12h16"/><path d="M4 17h10"/></svg>
        新建节点
      </button>
      <button class="primary" id="newexit">
        <svg viewBox="0 0 24 24"><path d="M12 5v14"/><path d="M5 12h14"/></svg>
        新建出口
      </button>
    </div>

    <div class="content">
      <section class="view" id="view-dash">
        <div class="stats">
          <div class="stat"><div class="num" id="stUp">–</div><div class="lbl">在线出口</div></div>
          <div class="stat"><div class="num" id="stNodes">–</div><div class="lbl">节点总数</div></div>
          <div class="stat"><div class="num small" id="stBackend">–</div><div class="lbl">节点后端</div></div>
          <div class="stat"><div class="num small mono" id="stIP">–</div><div class="lbl">本机公网 IP</div></div>
        </div>
        <h2 class="secttl">进行中的任务</h2>
        <div id="jobs"></div>
      </section>

      <section class="view" id="view-exits" hidden>
        <div class="toolbar">
          <h2>出口 <span class="count" id="ecount"></span></h2>
          <span class="spacer"></span>
          <button id="stopall">
            <svg viewBox="0 0 24 24"><rect x="6" y="6" width="12" height="12" rx="1"/></svg>
            全部停止
          </button>
        </div>
        <div id="list"></div>
      </section>

      <section class="view" id="view-settings" hidden>
        <div class="card narrow">
          <label class="f"><span>访问口令</span>
            <input id="setPw" type="password" spellcheck="false" autocomplete="new-password" placeholder="留空则不改"></label>
          <div class="hint">改完只影响新登录，当前这个浏览器不会被踢下线。</div>

          <label class="f" style="margin-top:18px"><span>访问路径</span>
            <input id="setPath" type="text" spellcheck="false" placeholder="留空则去掉路径前缀"></label>
          <div class="hint" id="setPathHint">界面挂在这个路径下，扫端口的探不到。只能用字母数字和 - _。</div>

          <label class="f" style="margin-top:18px"><span>节点后端</span>
            <select id="setBackend"></select></label>
          <div class="hint" id="setBackendHint">节点从哪来。装了 3x-ui 就能直接接管，没有就用自建。</div>

          <label class="chk" style="margin-top:18px"><input type="checkbox" id="setResi"> 只用家宽节点</label>
          <div class="hint" id="setResiHint">vpngate 里混着一批它自己的机房机器，出口一眼看得出是数据中心。勾着就只挑志愿者家宽。</div>

          <div class="setrow">
            <label class="f" style="margin:0"><span>监听端口</span>
              <input id="setPort" type="text" inputmode="numeric" spellcheck="false"></label>
            <label class="f" style="margin:0"><span>本地监听地址</span>
              <select id="setListen">
                <option value="0.0.0.0">所有网卡（0.0.0.0）</option>
                <option value="127.0.0.1">仅本机（127.0.0.1）</option>
              </select></label>
          </div>
          <div class="hint bad" id="setPortHint">改端口或监听地址会切换监听，保存后要用新地址重新打开界面。</div>

          <div class="setrow" style="margin-top:18px">
            <label class="f" style="margin:0"><span>HTTPS 证书路径</span>
              <input id="setCert" type="text" spellcheck="false" placeholder="留空则用 HTTP，如 /root/cert/域名/fullchain.pem"></label>
            <label class="f" style="margin:0"><span>HTTPS 私钥路径</span>
              <input id="setKey" type="text" spellcheck="false" placeholder="如 /root/cert/域名/privkey.pem"></label>
          </div>
          <div class="hint">两个都填启用 HTTPS，改完用 https:// 打开。证书可以用 h 菜单的「证书管理」申请。</div>
          <div class="hint" id="setTLSInfo" hidden></div>

          <div class="updsec">
            <div class="updrow">
              <div class="updver">版本 <b id="updCur">-</b><span id="updLatest"></span></div>
              <span class="spacer"></span>
              <button id="updCheck">检查更新</button>
              <button class="primary" id="updApply" hidden>更新到 <span id="updApplyVer"></span></button>
            </div>
            <div class="updnotes" id="updNotes" hidden></div>
            <div class="updrow" style="margin-top:10px">
              <div class="dim small">面板卡住或改了配置没生效，重启面板试试。<br>出口会短暂断开后自动重连。</div>
              <span class="spacer"></span>
              <button id="panelRestart">重启面板</button>
            </div>
          </div>

          <div class="formfoot">
            <span class="spacer"></span>
            <button class="primary" id="setSave">保存设置</button>
          </div>
        </div>
      </section>

      <section class="view" id="view-nodes" hidden>
        <div class="toolbar">
          <h2>节点 <span class="count" id="ncount"></span></h2>
          <span class="spacer"></span>
          <button id="subBtn">
            <svg viewBox="0 0 24 24"><path d="M4 11a9 9 0 0 1 9 9"/><path d="M4 4a16 16 0 0 1 16 16"/><circle cx="5" cy="19" r="1"/></svg>
            订阅地址
          </button>
          <button id="exportAll">
            <svg viewBox="0 0 24 24"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><path d="M7 10l5 5 5-5"/><path d="M12 15V3"/></svg>
            复制全部链接
          </button>
        </div>
        <div id="nodelist"></div>
      </section>
    </div>
  </div>
</div>

<div class="modal" id="wizard">
  <div class="sheet">
    <div class="head">
      <h2>新建出口</h2>
      <span class="spacer"></span>
      <button class="iconbtn" data-close="wizard" title="关闭">
        <svg viewBox="0 0 24 24"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>
      </button>
    </div>
    <div class="body">
      <label class="f">
        <span>地区</span>
        <div class="seg" id="rgmode">
          <button data-rgmode="auto" class="sel"><b>不限地区</b><em>按速度自动挑</em></button>
          <button data-rgmode="each"><b>每个国家</b><em>每个国家各来几个</em></button>
          <button data-rgmode="one"><b>指定地区</b><em>只建这个国家的</em></button>
        </div>
        <div id="rgpick" hidden style="margin-top:10px">
          <input type="search" id="rgfilter" placeholder="筛选地区">
          <div class="regions" id="regions" style="margin-top:8px"></div>
        </div>
      </label>
      <label class="f">
        <span id="countlabel">数量</span>
        <div class="stepper">
          <button id="minus" title="减少">
            <svg viewBox="0 0 24 24"><path d="M5 12h14"/></svg>
          </button>
          <input id="count" type="text" inputmode="numeric" value="3">
          <button id="plus" title="增加">
            <svg viewBox="0 0 24 24"><path d="M12 5v14"/><path d="M5 12h14"/></svg>
          </button>
        </div>
        <div class="hint" id="availhint"></div>
      </label>
      <label class="f" id="tplwrap">
        <span>节点模板</span>
        <select id="tpl"></select>
        <div class="hint" id="tplhint"></div>
      </label>
    </div>
    <div class="foot">
      <span class="count" id="wzhint"></span>
      <span class="spacer"></span>
      <button data-close="wizard">取消</button>
      <button class="primary" id="go">开始</button>
    </div>
  </div>
</div>

<div class="modal" id="newnodebox">
  <div class="sheet">
    <div class="head">
      <h2>新建节点</h2>
      <span class="spacer"></span>
      <button class="iconbtn" data-close="newnodebox" title="关闭">
        <svg viewBox="0 0 24 24"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>
      </button>
    </div>
    <div class="body">
      <label class="f">
        <span>协议</span>
        <select id="nproto">
          <option value="vless">VLESS</option>
          <option value="vmess">VMess</option>
          <option value="trojan">Trojan</option>
        </select>
      </label>
      <label class="f">
        <span>传输</span>
        <select id="nnet">
          <option value="tcp">TCP</option>
          <option value="ws">WebSocket</option>
          <option value="grpc">gRPC</option>
          <option value="httpupgrade">HTTPUpgrade</option>
          <option value="xhttp">XHTTP</option>
        </select>
      </label>
      <label class="f">
        <span>安全</span>
        <select id="nsec">
          <option value="none">无</option>
          <option value="tls">TLS</option>
          <option value="reality">REALITY</option>
        </select>
        <div class="hint" id="nsechint"></div>
      </label>
      <details class="adv" id="nadvwrap" hidden>
        <summary>高级选项<span class="dim" id="nadvcount"></span></summary>
      <label class="f" id="nvisionwrap" hidden>
        <span>流控</span>
        <label class="chk"><input type="checkbox" id="nvision"> xtls-rprx-vision</label>
      </label>
      <label class="f" id="nsniwrap" hidden>
        <span>域名 SNI</span>
        <input id="nsni" type="text" placeholder="留空用 localhost，将生成自签证书">
      </label>
      <label class="f" id="ncertwrap" hidden>
        <span>证书路径</span>
        <input id="ncert" type="text" placeholder="留空生成自签证书，如 /etc/ssl/x.crt">
      </label>
      <label class="f" id="nkeywrap" hidden>
        <span>私钥路径</span>
        <input id="nkey" type="text" placeholder="与证书成对填写，如 /etc/ssl/x.key">
      </label>
      <label class="f" id="ndestwrap" hidden>
        <span>借用站点</span>
        <input id="ndest" type="text" placeholder="留空用 www.tesla.com:443">
      </label>
      <label class="f" id="npathwrap" hidden>
        <span id="npathlabel">路径</span>
        <input id="npath" type="text" placeholder="留空自动生成">
      </label>
      </details>
      <label class="f">
        <span>端口</span>
        <input id="nport" type="text" inputmode="numeric" placeholder="留空随机分配">
      </label>
      <label class="f">
        <span>备注</span>
        <input id="nremark" type="text" placeholder="留空自动命名">
      </label>
    </div>
    <div class="foot">
      <span class="count" id="nnhint"></span>
      <span class="spacer"></span>
      <button data-close="newnodebox">取消</button>
      <button class="primary" id="ncreate">创建</button>
    </div>
  </div>
</div>

<div class="modal" id="detail">
  <div class="sheet">
    <div class="head">
      <h2 id="dtitle">节点</h2>
      <span class="spacer"></span>
      <button class="iconbtn danger" id="ddel" title="删除这个入站">
        <svg viewBox="0 0 24 24"><path d="M3 6h18"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6"/><path d="M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
      </button>
      <button class="iconbtn" data-close="detail" title="关闭">
        <svg viewBox="0 0 24 24"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>
      </button>
    </div>
    <div class="body" id="dbody"></div>
  </div>
</div>

<div class="modal" id="credbox">
  <div class="sheet">
    <div class="head">
      <h2>SOCKS5 访问凭据</h2>
      <span class="count" id="crtitle"></span>
      <span class="spacer"></span>
      <button class="iconbtn" data-close="credbox" title="关闭">
        <svg viewBox="0 0 24 24"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>
      </button>
    </div>
    <div class="body">
      <div class="share" id="crurl"></div>
      <div class="credrow">
        <label class="ef"><span>用户名</span>
          <input id="cruser" type="text" spellcheck="false"></label>
        <label class="ef"><span>口令</span>
          <input id="crpass" type="text" spellcheck="false"></label>
        <button id="crrand" title="随机生成一套">
          <svg viewBox="0 0 24 24"><path d="M21 12a9 9 0 1 1-3-6.7L21 8"/><path d="M21 3v5h-5"/></svg>
          随机
        </button>
      </div>
      <div class="hint">改完立即生效，已连上的会话不断；用旧凭据的客户端要改配置。</div>
    </div>
    <div class="foot">
      <button id="crcopy">
        <svg viewBox="0 0 24 24"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
        复制地址
      </button>
      <span class="spacer"></span>
      <button data-close="credbox">取消</button>
      <button class="primary" id="crsave">保存</button>
    </div>
  </div>
</div>

<div class="modal" id="export">
  <div class="sheet">
    <div class="head">
      <h2>节点链接</h2>
      <span class="count" id="excount"></span>
      <span class="spacer"></span>
      <button id="copyall">
        <svg viewBox="0 0 24 24"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
        全部复制
      </button>
      <button class="iconbtn" data-close="export" title="关闭">
        <svg viewBox="0 0 24 24"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>
      </button>
    </div>
    <div class="body"><textarea id="exbox" spellcheck="false" readonly></textarea></div>
  </div>
</div>

<div class="modal" id="subbox">
  <div class="sheet">
    <div class="head">
      <h2>订阅</h2>
      <span class="count" id="subcount"></span>
      <span class="spacer"></span>
      <button class="iconbtn" data-close="subbox" title="关闭">
        <svg viewBox="0 0 24 24"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>
      </button>
    </div>
    <div class="body">
      <label class="f"><span>订阅地址</span>
        <input id="suburl" type="text" spellcheck="false" readonly></label>
      <div class="hint">客户端里新建订阅填这条。以后加出口、删出口都会自己跟上，不用重新配。</div>
      <div class="hint bad">地址最后那串口令等于密码，别发群里。</div>
    </div>
    <div class="foot">
      <span class="spacer"></span>
      <button id="subreset">换一串口令</button>
      <button class="primary" id="subcopy">复制地址</button>
    </div>
  </div>
</div>

<div class="toast" id="toast"></div>

<script>
const $ = s => document.querySelector(s);

/* ---- 主题：跟随系统 / 浅色 / 深色，存在 localStorage ---- */
const THEME_ICONS = {
  auto:'<svg viewBox="0 0 24 24"><rect x="2" y="4" width="20" height="14" rx="2"/><path d="M8 22h8"/></svg>',
  light:'<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/></svg>',
  dark:'<svg viewBox="0 0 24 24"><path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z"/></svg>'
};
const THEME_LABEL = {auto:'跟随系统', light:'浅色模式', dark:'深色模式'};
function themeMode(){
  try{ return localStorage.getItem('hb-theme') || 'auto'; }catch(e){ return 'auto'; }
}
function applyTheme(mode){
  const root = document.documentElement;
  if(mode === 'light' || mode === 'dark') root.setAttribute('data-theme', mode);
  else { root.removeAttribute('data-theme'); mode = 'auto'; }
  try{ localStorage.setItem('hb-theme', mode); }catch(e){}
  renderThemeMenu();
}
/* 主题下拉：三个选项带文字，当前项打勾 */
function renderThemeMenu(){
  const cur = themeMode();
  const btn = $('#themeBtn');
  if(btn){
    btn.innerHTML = THEME_ICONS[cur];
    btn.title = '主题：' + THEME_LABEL[cur] + '（点击选择）';
  }
  document.querySelectorAll('#themeMenu [data-theme-opt]').forEach(b => {
    const on = b.dataset.themeOpt === cur;
    const c = b.querySelector('.check');
    if(c) c.style.visibility = on ? 'visible' : 'hidden';
  });
}
(function buildThemeMenu(){
  $('#themeMenu').innerHTML = ['auto', 'light', 'dark'].map(k =>
    '<button data-theme-opt="' + k + '">' + THEME_ICONS[k]
    + '<span>' + THEME_LABEL[k] + '</span><span class="check">✓</span></button>').join('');
  document.querySelectorAll('#themeMenu [data-theme-opt]').forEach(b => {
    b.onclick = () => {
      applyTheme(b.dataset.themeOpt);
      $('#themeMenu').hidden = true;
      toast('主题：' + THEME_LABEL[themeMode()]);
    };
  });
})();
$('#themeBtn').onclick = e => {
  e.stopPropagation();
  const m = $('#themeMenu');
  m.hidden = !m.hidden;
  if(!m.hidden) renderThemeMenu();
};
document.addEventListener('click', e => {
  if(!e.target.closest('.menuwrap')) $('#themeMenu').hidden = true;
});
applyTheme(themeMode());

/* ---- 视图切换：总览 / 出口 / 节点 ---- */
const VIEW_TITLES = {dash:'总览', exits:'出口', nodes:'节点', settings:'设置'};
let polling = false; // poll 防重叠标记：必须在首次 switchView 调用之前初始化
function switchView(v){
  if(!VIEW_TITLES[v]) v = 'dash';
  document.querySelectorAll('.nav button').forEach(b =>
    b.classList.toggle('on', b.dataset.view === v));
  ['dash', 'exits', 'nodes', 'settings'].forEach(k => { $('#view-' + k).hidden = (k !== v); });
  $('#viewTitle').textContent = VIEW_TITLES[v];
  // 右上角按钮跟随页面：出口页只留新建出口，节点页只留新建节点，总览页两个都留
  $('#newexit').style.display = (v === 'nodes' || v === 'settings') ? 'none' : '';
  $('#newnode').style.display = (v === 'exits' || v === 'settings') ? 'none' : '';
  if(v === 'settings') loadSettings();
  try{ localStorage.setItem('hb-view', v); }catch(e){}
  poll(); // 切视图立即刷一次，不用等下个 3 秒周期
}
document.querySelector('.nav').addEventListener('click', e => {
  const b = e.target.closest('[data-view]');
  if(b) switchView(b.dataset.view);
});
(function(){
  let v = 'dash';
  try{ v = localStorage.getItem('hb-view') || 'dash'; }catch(e){}
  switchView(v);
})();

const ICON = {
  copy:'<svg viewBox="0 0 24 24"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>',
  stop:'<svg viewBox="0 0 24 24"><rect x="6" y="6" width="12" height="12" rx="1"/></svg>',
  redo:'<svg viewBox="0 0 24 24"><path d="M21 12a9 9 0 1 1-3-6.7L21 8"/><path d="M21 3v5h-5"/></svg>',
  ok:'<svg viewBox="0 0 24 24"><path d="M20 6 9 17l-5-5"/></svg>',
  bad:'<svg viewBox="0 0 24 24"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>',
  run:'<svg viewBox="0 0 24 24" class="spin"><path d="M21 12a9 9 0 1 1-6.2-8.5"/></svg>',
  wait:'<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/></svg>',
  plus:'<svg viewBox="0 0 24 24"><path d="M12 5v14"/><path d="M5 12h14"/></svg>',
  trash:'<svg viewBox="0 0 24 24"><path d="M3 6h18"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6"/><path d="M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>',
  x:'<svg viewBox="0 0 24 24"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>',
  lock:'<svg viewBox="0 0 24 24"><rect x="3" y="11" width="18" height="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>'
};

// 界面挂在随机前缀下，请求一律走相对路径
async function api(path, opts){
  const ctl = new AbortController();
  const to = setTimeout(() => ctl.abort(), 15000);
  let r;
  try{
    r = await fetch(path.replace(/^\//, ''), Object.assign({}, opts, {signal: ctl.signal}));
  }catch(e){
    if(e && e.name === 'AbortError') throw new Error('请求超时');
    throw e;
  }finally{
    clearTimeout(to);
  }
  if(r.status === 401){
    // 会话过期：刷一次，服务端会直接给登录页，不会死循环
    location.reload();
    throw new Error('未登录');
  }
  const d = await r.json().catch(()=>({}));
  if(!r.ok) throw new Error(d.error || ('HTTP '+r.status));
  return d;
}
function esc(s){ return String(s == null ? '' : s).replace(/[&<>"']/g, c =>
  ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c])); }

let toastTimer;
function toast(msg, bad){
  const el = $('#toast');
  el.textContent = msg;
  el.className = 'toast show' + (bad ? ' bad' : '');
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { el.className = 'toast'; }, 2400);
}
async function copy(text){
  // navigator.clipboard 只在 HTTPS/localhost 下存在，而面板通常是 http://IP 访问，
  // 所以必须留一条 execCommand 兜底路径，否则复制在正常使用场景里必然失败。
  if(navigator.clipboard && window.isSecureContext){
    try{ await navigator.clipboard.writeText(text); toast('已复制'); return; }
    catch(e){}
  }
  const ta = document.createElement('textarea');
  ta.value = text;
  ta.setAttribute('readonly', '');
  // 放在视口内但不可见：置于视口外会让 iOS 在聚焦时滚动页面
  ta.style.cssText = 'position:fixed;top:0;left:0;width:1px;height:1px;opacity:0;padding:0;border:0';
  document.body.appendChild(ta);
  const prev = document.activeElement;
  ta.focus();
  ta.setSelectionRange(0, ta.value.length);
  let ok = false;
  try{ ok = document.execCommand('copy'); }catch(e){}
  ta.remove();
  if(prev && prev.focus) prev.focus();
  toast(ok ? '已复制' : '复制失败，请手动选中', !ok);
}

let view = {exits:[], direct:[], panel:'', backend:'', public_ip:''};
let inbounds = [];

// 自建模式下入站由 home-broadband 自己管，界面要提供新建入口；
// 接管 3x-ui 时入站归面板管，这里只读不写。
function isNative(){ return view.backend === 'native'; }
const BACKEND_NAME = {'native':'自建 Xray', '3x-ui':'3x-ui'};
function backendName(){ return BACKEND_NAME[view.backend] || '3x-ui'; }

const STATUS = {up:'已连通', starting:'连接中', failed:'失败', stopped:'已停止'};

/* ---- 总览 ---- */
function renderDash(){
  const n = view.exits.length;
  const up = view.exits.filter(e => e.status === 'up').length;
  $('#stUp').innerHTML = up + '<span class="dim small"> / ' + n + '</span>';
  const nn = view.exits.reduce((a, e) => a + (e.inbounds || []).length, 0)
    + (view.direct || []).length;
  $('#stNodes').textContent = nn;
  $('#stBackend').textContent = backendName();
  $('#stIP').textContent = view.public_ip || '—';
}

/* ---- 出口 ---- */
function renderExits(){
  const list = $('#list');
  const n = view.exits.length;
  $('#ecount').textContent = n ? n + ' 个' : '';
  $('#navEcount').textContent = n || '';
  $('#exportAll').disabled = !view.exits.some(e => e.inbounds && e.inbounds.length);
  $('#stopall').disabled = !n;
  // 按地区排，地区放前面，一眼认出是哪个国家的出口
  const exits = view.exits.slice().sort((a, b) =>
    (a.country || a.region || '').localeCompare(b.country || b.region || '')
    || (a.exit_ip || '').localeCompare(b.exit_ip || ''));

  if(!n){
    list.innerHTML = '<div class="empty"><div class="empty-t">还没有出口</div>'
      + '<div class="dim small">新建一个出口，选地区、配节点，十几秒就能用</div>'
      + '<div style="margin-top:16px"><button class="primary" id="newexit2">'
      + ICON.plus + '新建出口</button></div></div>';
    return;
  }

  list.innerHTML = exits.map(e => {
    const label = e.exit_ip || (e.status === 'starting' ? '连接中…' : '—');
    const chips = (e.inbounds || []).length
      ? e.inbounds.map(i => '<button class="chip" data-detail="' + i.id + '" title="点击查看详情：'
          + esc((i.remark || i.protocol) + ' · ' + i.protocol + ' :' + i.port) + '">'
          + esc(i.remark || i.protocol) + '<span> :' + i.port + '</span></button>').join('')
      : '<span class="chip none">无节点</span>';
    const err = e.status === 'failed' && e.err
      ? '<div class="errline" title="' + esc(e.err) + '">' + esc(e.err) + '</div>' : '';
    const place = esc(e.country || e.region || '—');
    return '<div class="exit">'
      + '<div class="etop">'
      +   '<span class="dot ' + e.status + '" title="' + (STATUS[e.status] || e.status) + '"></span>'
      +   '<span class="ip">' + place + '</span>'
      +   '<span class="meta mono">' + esc(label) + ' · ' + esc(e.host) + '</span>'
      +   '<span class="spacer"></span>'
      +   '<button class="socksbtn" data-cred="' + e.slot + '" title="SOCKS5 访问凭据">'
      +     ICON.lock + '<span class="mono">:' + e.port + '</span></button>'
      +   '<button data-swap="' + e.slot + '">换节点</button>'
      +   '<button data-stop="' + e.slot + '">停止</button>'
      + '</div>'
      + '<div class="ebot"><span class="lbl">节点</span>'
      +   '<span class="chips">' + chips + '</span></div>'
      + err + '</div>';
  }).join('');
}

/* ---- 节点：按地区分组，地区下再按出口分，IP 跟在地区后面 ---- */
function renderNodes(){
  const box = $('#nodelist');
  const withNodes = view.exits.filter(e => (e.inbounds || []).length);
  const unbound = view.direct || [];
  const total = withNodes.reduce((a, e) => a + e.inbounds.length, 0) + unbound.length;
  $('#ncount').textContent = total ? total + ' 个' : '';
  $('#navNcount').textContent = total || '';

  // 一行一个节点：名字 + 协议端口，右边是看得见的操作按钮，不再藏点击
  const nodeRow = (i, bound) => {
    const name = esc(i.remark || i.protocol);
    const sub = esc(i.protocol) + ' · 端口 ' + i.port;
    const bindCtl = bound
      ? '<button data-unbind="' + esc(i.tag) + '" data-name="' + name + '">解绑</button>'
      : '<label class="bindlbl">绑定到<select class="obind" data-tag="' + esc(i.tag) + '">'
        + exitOptions('') + '</select></label>';
    return '<div class="nrow">'
      + '<div class="ninfo"><b>' + name + '</b><span class="dim small">' + sub + '</span></div>'
      + '<span class="spacer"></span>'
      + '<div class="acts">'
      + '<button data-detail="' + i.id + '">详情</button>'
      + bindCtl
      + '<button class="iconbtn danger" data-delone="' + i.id + '" data-name="'
      +   name + ' :' + i.port + '" title="删除这个入站">' + ICON.trash + '</button>'
      + '</div></div>';
  };

  // 先按地区归类，地区内按出口 IP 排
  const byRegion = {};
  const order = [];
  withNodes.forEach(e => {
    const r = e.country || e.region || '其他';
    if(!byRegion[r]){ byRegion[r] = []; order.push(r); }
    byRegion[r].push(e);
  });
  order.sort((a, b) => a.localeCompare(b));
  Object.keys(byRegion).forEach(r =>
    byRegion[r].sort((a, b) => (a.exit_ip || '').localeCompare(b.exit_ip || '')));

  let html = order.map(r => {
    const exits = byRegion[r];
    const nn = exits.reduce((a, e) => a + e.inbounds.length, 0);
    return '<div class="card"><div class="nghead"><b>' + esc(r) + '</b>'
      + '<span class="count">' + exits.length + ' 个出口 · ' + nn + ' 个节点</span></div>'
      + exits.map(e =>
          '<div class="rgexit"><div class="rgexit-head">'
          + '<span class="dot ' + e.status + '" title="' + (STATUS[e.status] || e.status) + '"></span>'
          + '<span class="mono">' + esc(e.exit_ip || e.host) + '</span>'
          + '<span class="dim small">SOCKS5 :' + e.port + '</span>'
          + '</div>'
          + e.inbounds.map(i => nodeRow(i, true)).join('')
          + '</div>').join('')
      + '</div>';
  }).join('');

  if(unbound.length){
    const hasUp = view.exits.some(e => e.status === 'up');
    html += '<div class="card"><div class="nghead"><b>未绑定</b>'
      + '<span class="count">' + unbound.length + ' 个，走直连</span>'
      + '<span class="spacer"></span>'
      + '<button data-delorphans="1">' + ICON.trash + '删除未绑定</button></div>'
      + unbound.map(i => nodeRow(i, false)).join('')
      + (hasUp ? ''
          : '<div class="dim small" style="padding:8px 0">没有连通的出口，先去「出口」页开一个</div>')
      + '</div>';
  }

  if(!html){
    html = '<div class="empty"><div class="empty-t">还没有节点</div>'
      + '<div class="dim small">新建一个节点，再绑到出口上</div>'
      + '<div style="margin-top:16px"><button class="primary" id="newnode2">'
      + ICON.plus + '新建节点</button></div></div>';
  }
  box.innerHTML = html;
}

function renderJobs(jobs){
  const box = $('#jobs');
  if(!jobs.length){
    box.innerHTML = '<div class="dim small" style="padding:6px 2px">暂无任务</div>';
    return;
  }
  box.innerHTML = jobs.map(j => {
    const steps = j.steps.map(s => {
      const ic = {ok:ICON.ok, failed:ICON.bad, running:ICON.run}[s.status] || ICON.wait;
      const t = s.detail ? s.label + ' — ' + s.detail : s.label;
      return '<span class="step ' + s.status + '" title="' + esc(t) + '">' + ic
        + esc(s.status === 'ok' && s.detail ? s.detail : s.label) + '</span>';
    }).join('');
    const close = j.status === 'running' ? ''
      : '<button class="iconbtn" data-job="' + esc(j.id) + '" title="关闭">' + ICON.x + '</button>';
    return '<div class="job"><div class="top"><strong>' + esc(j.summary) + '</strong>'
      + '<span class="count">' + j.done + '/' + j.total + '</span>'
      + '<span class="spacer"></span>' + close + '</div>'
      + '<div class="steps">' + steps + '</div></div>';
  }).join('');
}

async function poll(){
  if(polling) return; // 上一次还没回来，跳过：避免堆积和旧响应覆盖新数据
  polling = true;
  try{
    view = await api('/api/exits');
    const bn = backendName();
    const pn = view.panel || '';
    $('#panel').textContent = pn
      ? (pn.toLowerCase() === bn.toLowerCase() ? bn : bn + ' · ' + pn)
      : (view.panel_info || '');
    renderDash();
    renderExits();
    renderNodes();
  }catch(e){
    console.warn('poll /api/exits 失败:', e && e.message || e);
  }finally{
    polling = false;
  }
  try{ renderJobs(await api('/api/jobs') || []); }catch(e){}
}

// ---- 新建向导 ----
let regions = [], region = '', regionsLoaded = false;
let rgmode = 'auto'; // auto=不限地区 each=每个国家 one=指定地区
// 模式换算成给后端的 region 参数：''=不限 '*'=每个国家，否则是国家码
function effRegion(){ return rgmode === 'each' ? '*' : rgmode === 'one' ? region : ''; }
function syncRgMode(){
  document.querySelectorAll('#rgmode [data-rgmode]').forEach(b =>
    b.classList.toggle('sel', b.dataset.rgmode === rgmode));
  $('#rgpick').hidden = rgmode !== 'one';
}

function openModal(id){ $('#' + id).classList.add('open'); }
function closeModal(id){ $('#' + id).classList.remove('open'); }

document.addEventListener('click', e => {
  const c = e.target.closest('[data-close]');
  if(c) closeModal(c.dataset.close);
});
document.addEventListener('keydown', e => {
  if(e.key === 'Escape') document.querySelectorAll('.modal.open')
    .forEach(m => m.classList.remove('open'));
});
document.querySelectorAll('.modal').forEach(m => {
  m.onclick = e => { if(e.target === m) m.classList.remove('open'); };
});

function renderRegions(){
  const kw = $('#rgfilter').value.trim().toLowerCase();
  const list = regions.filter(r => !kw
    || r.code.toLowerCase().includes(kw) || r.name.toLowerCase().includes(kw));
  $('#regions').innerHTML = list.map(r => '<button class="rg' + (region === r.code ? ' sel' : '')
      + '" data-rg="' + esc(r.code) + '"><b>' + esc(r.name || r.code) + '</b>'
      + '<em>' + r.available + ' 个空闲 · ' + r.best_speed_mbps.toFixed(0) + ' Mbps</em></button>')
    .join('');
  updateAvail();
}

function availOf(code){
  if(code === '') return regions.reduce((a, r) => a + r.available, 0);
  if(code === '*') return regions.reduce((a, r) => a + r.available, 0);
  const r = regions.find(x => x.code === code);
  return r ? r.available : 0;
}

function updateAvail(){
  const want = Number($('#count').value) || 0;
  const hint = $('#availhint');
  const er = effRegion();
  // 选了"每个国家"时，数量的意思是每国几个，提示要给出总条数
  $('#countlabel').textContent = er === '*' ? '每个国家几个' : '数量';
  if(er === '*'){
    const n = regions.length;
    const total = Math.min(n * want, availOf('*'));
    hint.className = 'hint';
    hint.textContent = n
      ? n + ' 个国家 × ' + want + '，一共 ' + total + ' 条出口'
      : '还没有可用节点';
    $('#go').disabled = !n || !want;
    return;
  }
  const avail = availOf(er);
  hint.textContent = avail ? '可用 ' + avail + ' 个节点' : '这个地区没有空闲节点';
  hint.className = 'hint' + (want > avail ? ' bad' : '');
  if(want > avail && avail) hint.textContent = '只剩 ' + avail + ' 个，将全部使用';
  $('#go').disabled = !avail;
}

async function loadWizard(){
  try{
    regions = await api('/api/regions') || [];
    regionsLoaded = true;
    renderRegions();
  }catch(e){ toast('读取地区失败: ' + e.message, true); }

  const sel = $('#tpl');
  $('#tplwrap').hidden = false;
  try{
    // 已经挂在出口上的多半是上一批复制出来的，拿它当模板会套娃，
    // 所以把没绑出口的排在前面并默认选中
    const v = await api('/api/exits');
    const free = v.direct || [];
    const bound = (v.exits || []).flatMap(e => e.inbounds || []);
    inbounds = free.concat(bound);
    if(!inbounds.length){
      sel.innerHTML = '<option value="0">还没有节点</option>';
      $('#tplhint').textContent = '先用上面的「新建节点」建一个，之后这里可以按它批量生成';
      return;
    }
    const opt = i => '<option value="' + i.id + '">'
      + esc(i.remark || ('端口 ' + i.port)) + ' · ' + esc(i.protocol)
      + ' :' + i.port + '</option>';
    sel.innerHTML =
      (free.length ? '<optgroup label="未绑定出口">' + free.map(opt).join('') + '</optgroup>' : '')
      + (bound.length ? '<optgroup label="已挂在出口上">' + bound.map(opt).join('') + '</optgroup>' : '')
      + '<option value="0">只开出口，不建节点</option>';
    $('#tplhint').textContent = '每个出口复制一份，客户端 UUID 保持一致，只有端口不同';
  }catch(e){
    sel.innerHTML = '<option value="0">' + backendName() + '不可用</option>';
    $('#tplhint').textContent = e.message;
  }
}

document.addEventListener('click', e => {
  if(e.target.closest('#newexit') || e.target.closest('#newexit2')){
    openModal('wizard');
    rgmode = 'auto'; region = '';
    syncRgMode();
    if(!regionsLoaded) loadWizard(); else { renderRegions(); loadWizard(); }
  }
  const mg = e.target.closest('[data-rgmode]');
  if(mg){
    rgmode = mg.dataset.rgmode;
    // "每个国家"是批量，默认每国 1 个，免得一点就开出几十条
    if(rgmode === 'each' && Number($('#count').value) > 3) $('#count').value = '1';
    // 切到"指定地区"还没选国家时，默认选中空闲最多的
    if(rgmode === 'one' && !region && regions.length) region = regions[0].code;
    syncRgMode();
    renderRegions();
    return;
  }
  const rg = e.target.closest('[data-rg]');
  if(rg){
    region = rg.dataset.rg;
    renderRegions();
  }
});

// ---- 新建节点 ----
document.addEventListener('click', e => {
  if(e.target.closest('#newnode') || e.target.closest('#newnode2')){
    $('#nnhint').textContent = '';
    syncNodeForm();
    openModal('newnodebox');
  }
});

// 表单随协议/传输/安全层联动：只露出当前组合真正用得到的字段
function syncNodeForm(){
  const proto = $('#nproto').value;
  const net   = $('#nnet').value;
  const sec   = $('#nsec').value;

  // REALITY 靠模仿 TLS 握手工作，套在自带头部的传输上没有意义
  const realityOK = net === 'tcp' || net === 'xhttp' || net === 'grpc';
  const secSel = $('#nsec');
  for(const o of secSel.options){
    if(o.value === 'reality') o.disabled = !realityOK;
  }
  if(secSel.value === 'reality' && !realityOK) secSel.value = 'none';

  const cur = secSel.value;
  $('#nsniwrap').hidden  = cur !== 'tls';
  $('#ncertwrap').hidden = cur !== 'tls';
  $('#nkeywrap').hidden  = cur !== 'tls';
  $('#ndestwrap').hidden = cur !== 'reality';

  // Vision 只在 VLESS + 裸 TCP + TLS/REALITY 下有效
  const visionOK = proto === 'vless' && net === 'tcp' && cur !== 'none';
  $('#nvisionwrap').hidden = !visionOK;
  if(!visionOK) $('#nvision').checked = false;

  const needPath = net === 'ws' || net === 'httpupgrade' || net === 'xhttp' || net === 'grpc';
  $('#npathwrap').hidden = !needPath;
  $('#npathlabel').textContent = net === 'grpc' ? '服务名' : '路径';

  $('#nsechint').textContent =
    cur === 'reality' ? '密钥与 shortId 自动生成' :
    cur === 'tls'     ? '不填证书就用自签，链接会带证书指纹' : '';

  // 高级选项：有可用项才露出来，刚露出来就自动展开
  const advIds = ['nvisionwrap', 'nsniwrap', 'ncertwrap', 'nkeywrap', 'ndestwrap', 'npathwrap'];
  const advN = advIds.filter(id => !$('#' + id).hidden).length;
  const adv = $('#nadvwrap');
  const wasHidden = adv.hidden;
  adv.hidden = !advN;
  $('#nadvcount').textContent = advN ? '（' + advN + ' 项可用）' : '';
  if(wasHidden && advN) adv.open = true;
}
$('#nproto').onchange = syncNodeForm;
$('#nnet').onchange = syncNodeForm;
$('#nsec').onchange = syncNodeForm;

$('#ncreate').onclick = async e => {
  const q = new URLSearchParams({
    protocol: $('#nproto').value,
    network:  $('#nnet').value,
    security: $('#nsec').value,
    port:     ($('#nport').value || '').trim(),
    remark:   ($('#nremark').value || '').trim(),
    path:     ($('#npath').value || '').trim(),
    sni:      ($('#nsni').value || '').trim(),
    cert:     ($('#ncert').value || '').trim(),
    key:      ($('#nkey').value || '').trim(),
    dest:     ($('#ndest').value || '').trim(),
  });
  if($('#nvision').checked) q.set('vision', '1');
  e.target.disabled = true;
  try{
    const r = await api('/api/panel/inbound/new?' + q.toString(), {method:'POST'});
    toast('已创建 ' + r.protocol + ' 节点，端口 ' + r.port);
    closeModal('newnodebox');
    $('#nport').value = '';
    $('#nremark').value = '';
    poll();
  }catch(err){ toast(err.message, true); }
  e.target.disabled = false;
};

$('#rgfilter').oninput = renderRegions;
$('#minus').onclick = () => { step(-1); };
$('#plus').onclick = () => { step(1); };
function step(d){
  const el = $('#count');
  el.value = Math.min(20, Math.max(1, (Number(el.value) || 1) + d));
  updateAvail();
}
$('#count').oninput = updateAvail;

$('#go').onclick = async e => {
  const er = effRegion();
  const want = Math.min(Number($('#count').value) || 1, availOf(er) || 1);
  const tpl = $('#tpl').value || '0';
  e.target.disabled = true;
  try{
    await api('/api/provision?count=' + want
      + (er === '*' ? '&every=1' : '&region=' + encodeURIComponent(er))
      + '&template=' + tpl, {method:'POST'});
    closeModal('wizard');
    poll();
  }catch(err){ toast(err.message, true); }
  e.target.disabled = false;
};

// ---- 出口操作 ----
document.addEventListener('click', async e => {
  const stop = e.target.closest('[data-stop]');
  if(stop){
    const x = view.exits.find(v => v.slot === Number(stop.dataset.stop));
    const nn = x && x.inbounds ? x.inbounds.length : 0;
    const ip = x ? (x.exit_ip || x.host || '') : '';
    if(!confirm('停止出口' + (ip ? ' ' + ip : '') + '？'
      + (nn ? '上面 ' + nn + ' 个节点会回到直连，节点本身不会删除。' : '这个出口上没有节点。'))) return;
    stop.disabled = true;
    try{ await api('/api/stop?slot=' + stop.dataset.stop, {method:'POST'}); }
    catch(err){ toast(err.message, true); }
    poll();
    return;
  }
  const swap = e.target.closest('[data-swap]');
  if(swap){
    const x = view.exits.find(v => v.slot === Number(swap.dataset.swap));
    const ip = x ? (x.exit_ip || x.host || '') : '';
    if(!confirm('给出口' + (ip ? ' ' + ip : '') + '换个节点？切换时会断一下。')) return;
    swap.disabled = true;
    try{
      await api('/api/swap?slot=' + swap.dataset.swap, {method:'POST'});
      toast('正在换节点');
    }catch(err){ toast(err.message, true); }
    poll();
    return;
  }
  const cred = e.target.closest('[data-cred]');
  if(cred){ openCred(Number(cred.dataset.cred)); return; }
  const job = e.target.closest('[data-job]');
  if(job){
    try{ await api('/api/jobs/dismiss?id=' + job.dataset.job, {method:'POST'}); }catch(err){}
    poll();
    return;
  }
  const unbind = e.target.closest('[data-unbind]');
  if(unbind){
    if(!confirm('把 ' + unbind.dataset.name + ' 从出口解绑？它会回到直连。')) return;
    unbind.disabled = true;
    try{
      await api('/api/xui/bind?tag=' + encodeURIComponent(unbind.dataset.unbind)
        + '&host=', {method:'POST'});
      toast('已解绑');
    }catch(err){ toast(err.message, true); }
    poll();
    return;
  }
  const del = e.target.closest('[data-delorphans]');
  if(del){
    const list = view.direct || [];
    if(!confirm('删除这 ' + list.length + ' 个未绑定节点？此操作不可撤销。')) return;
    del.disabled = true;
    try{
      await api('/api/xui/delete?ids=' + list.map(i => i.id).join(','), {method:'POST'});
      toast('已清理 ' + list.length + ' 个入站');
    }catch(err){ toast(err.message, true); }
    poll();
    return;
  }

  const one = e.target.closest('[data-delone]');
  if(one){
    if(!confirm('删除入站 ' + one.dataset.name + '？此操作不可撤销。')) return;
    one.disabled = true;
    try{
      await api('/api/xui/delete?ids=' + one.dataset.delone, {method:'POST'});
      toast('已删除 ' + one.dataset.name);
    }catch(err){ toast(err.message, true); one.disabled = false; }
    poll();
  }
});

$('#stopall').onclick = async e => {
  if(!confirm('停止全部 ' + view.exits.length + ' 个出口？')) return;
  e.target.disabled = true;
  for(const x of view.exits){
    try{ await api('/api/stop?slot=' + x.slot, {method:'POST'}); }catch(err){}
  }
  poll();
};

// ---- 节点详情 ----
let curDetail = null;

// 详情弹窗的重绘要跟轮询解耦：正在编辑时被 poll 刷掉输入会很烦
async function openDetail(id){
  $('#dbody').innerHTML = '<div class="empty"><div class="dim">读取中…</div></div>';
  curDetail = null;
  $('#ddel').disabled = true;
  openModal('detail');
  try{
    const d = await api('/api/xui/detail?id=' + id);
    curDetail = d;
    renderDetail(d);
  }catch(err){
    $('#dbody').innerHTML = '<div class="empty"><div class="dim">读取失败: ' + esc(err.message) + '</div></div>';
  }
}

// 出口下拉：列出所有已连通的隧道，外加"直连"。绑定按 Xray 的 inboundTag 走。
function exitOptions(currentHost){
  const up = view.exits.filter(e => e.status === 'up');
  return '<option value=""' + (currentHost ? '' : ' selected') + '>直连（不走隧道）</option>'
    + up.map(e => '<option value="' + esc(e.host) + '"'
        + (e.host === currentHost ? ' selected' : '') + '>'
        + esc((e.exit_ip || e.host) + ' · ' + (e.country || e.region)) + '</option>').join('');
}

function renderDetail(d){
  const owner = view.exits.find(x => (x.inbounds || []).some(i => i.id === d.id));

  const clients = (d.clients || []).map((c, i) => {
    const link = (d.links || [])[i] || '';
    return '<div class="client">'
      + '<div class="crow">'
      +   '<span class="cemail">' + esc(c.email) + '</span>'
      +   '<span class="cid">' + esc(c.id) + '</span>'
      +   '<span class="spacer"></span>'
      +   (link ? '<button class="iconbtn" data-copy="' + esc(link) + '" title="复制链接">' + ICON.copy + '</button>' : '')
      +   '<button class="iconbtn" data-creset="' + esc(c.email) + '" title="换一套凭据，旧链接立即失效">' + ICON.redo + '</button>'
      +   '<button class="iconbtn" data-cdel="' + esc(c.email) + '" title="删除这个客户端">' + ICON.trash + '</button>'
      + '</div>'
      + (link ? '<div class="share">' + esc(link) + '</div>' : '')
      + '</div>';
  }).join('');

  $('#dtitle').textContent = (d.remark || '节点') + '　:' + d.port;
  $('#dbody').innerHTML = '<dl class="kv">'
    + '<dt>出口</dt><dd><select id="dbind" data-tag="' + esc(d.tag) + '">'
    +   exitOptions(owner ? owner.host : '') + '</select></dd>'
    + '<dt>协议</dt><dd>' + esc(d.protocol) + '　' + esc(d.network || '')
    +   (d.tls && d.tls !== 'none' ? '　' + esc(d.tls) : '') + '</dd>'
    + '<dt>监听</dt><dd>' + esc(d.listen || '0.0.0.0') + '</dd>'
    + '</dl>'
    + ('<div class="editbar">'
    +   '<label class="ef"><span>备注</span>'
    +     '<input id="dremark" type="text" value="' + esc(d.remark || '') + '"></label>'
    +   '<label class="ef"><span>端口</span>'
    +     '<input id="dport" type="text" inputmode="numeric" value="' + d.port + '"></label>'
    +   '<label class="chk"><input type="checkbox" id="denable"'
    +     (d.enable === false ? '' : ' checked') + '> 启用</label>'
    +   '<span class="spacer"></span>'
    +   '<button class="primary" id="dsave">保存</button>'
    + '</div>'
    + '<div class="chead"><h3>客户端</h3><span class="count">'
    +   (d.clients || []).length + ' 个</span><span class="spacer"></span>'
    +   '<button id="dcadd">' + ICON.plus + '添加</button></div>'
    + (clients || '<div class="empty"><div class="dim">没有客户端</div></div>'));
}

// 未绑定区的出口下拉，选中即绑
document.addEventListener('change', async e => {
  const sel = e.target.closest('.obind');
  if(!sel || !sel.value) return;
  sel.disabled = true;
  try{
    await api('/api/xui/bind?tag=' + encodeURIComponent(sel.dataset.tag)
      + '&host=' + encodeURIComponent(sel.value), {method:'POST'});
    toast('已绑定');
    poll();
  }catch(err){ toast(err.message, true); sel.disabled = false; }
});

// 出口下拉改动即生效。绑定按 inboundTag 走，host 传空表示解绑回直连。
document.addEventListener('change', async e => {
  const sel = e.target.closest('#dbind');
  if(!sel) return;
  sel.disabled = true;
  try{
    await api('/api/xui/bind?tag=' + encodeURIComponent(sel.dataset.tag)
      + '&host=' + encodeURIComponent(sel.value), {method:'POST'});
    toast(sel.value ? '已绑定' : '已解绑');
    poll();
  }catch(err){ toast(err.message, true); }
  sel.disabled = false;
});

document.addEventListener('click', async e => {
  const link = e.target.closest('[data-detail]');
  if(link) return openDetail(link.dataset.detail);

  if(e.target.closest('#dsave')){
    const btn = e.target.closest('#dsave');
    btn.disabled = true;
    const q = new URLSearchParams({
      id: curDetail.id,
      port: ($('#dport').value || '').trim(),
      remark: ($('#dremark').value || '').trim(),
      enable: $('#denable').checked ? '1' : '0',
    });
    try{
      await api('/api/panel/inbound/update?' + q, {method:'POST'});
      toast('已保存');
      await openDetail(curDetail.id);
      poll();
    }catch(err){ toast(err.message, true); btn.disabled = false; }
    return;
  }

  const add = e.target.closest('#dcadd');
  if(add){
    add.disabled = true;
    try{
      await api('/api/panel/client/add?id=' + curDetail.id, {method:'POST'});
      toast('已添加客户端');
      await openDetail(curDetail.id);
    }catch(err){ toast(err.message, true); add.disabled = false; }
    return;
  }

  const del = e.target.closest('[data-cdel]');
  if(del){
    if(!confirm('删除客户端 ' + del.dataset.cdel + '？它的链接会立即失效。')) return;
    del.disabled = true;
    try{
      await api('/api/panel/client/del?id=' + curDetail.id
        + '&email=' + encodeURIComponent(del.dataset.cdel), {method:'POST'});
      toast('已删除');
      await openDetail(curDetail.id);
    }catch(err){ toast(err.message, true); del.disabled = false; }
    return;
  }

  const reset = e.target.closest('[data-creset]');
  if(reset){
    if(!confirm('重置 ' + reset.dataset.creset + ' 的凭据？已分发的旧链接会立即失效。')) return;
    reset.disabled = true;
    try{
      await api('/api/panel/client/reset?id=' + curDetail.id
        + '&email=' + encodeURIComponent(reset.dataset.creset), {method:'POST'});
      toast('已重置');
      await openDetail(curDetail.id);
    }catch(err){ toast(err.message, true); reset.disabled = false; }
    return;
  }

  // 详情弹窗里删掉当前这个入站
  const dd = e.target.closest('#ddel');
  if(dd && curDetail){
    const name = (curDetail.remark || curDetail.protocol || '节点') + ' :' + curDetail.port;
    if(!confirm('删除入站 ' + name + '？它的所有客户端链接都会失效，且不可撤销。')) return;
    dd.disabled = true;
    try{
      await api('/api/xui/delete?ids=' + curDetail.id, {method:'POST'});
      toast('已删除 ' + name);
      curDetail = null;
      closeModal('detail');
      poll();
    }catch(err){ toast(err.message, true); dd.disabled = false; }
  }
});

document.addEventListener('click', e => {
  const c = e.target.closest('[data-copy]');
  if(c) copy(c.dataset.copy);
});

// ---- SOCKS5 凭据 ----
let curCred = null;

function socksURL(host, port, user, pass){
  if(!user) return 'socks5://' + host + ':' + port;
  return 'socks5://' + user + ':' + pass + '@' + host + ':' + port;
}

// SOCKS5 端口监听在母机（跑 home-broadband 的这台服务器）上，客户端要连的是母机的
// 公网 IPv4，流量再从出口 IP 出去。出口 IP 是"出去以后"的地址，不能当连接地址。
// public_ip 是后端探测到的母机公网地址；探测不到才退回访问面板用的主机名。
function credHost(e){
  return view.public_ip || location.hostname || e.host;
}

function openCred(slot){
  const e = view.exits.find(x => x.slot === slot);
  if(!e){ toast('这个出口不在了', true); return; }
  curCred = {slot: slot, port: e.port, host: credHost(e)};
  $('#crtitle').textContent = (e.country || e.region) + ' · :' + e.port;
  $('#cruser').value = e.socks_user || '';
  $('#crpass').value = e.socks_pass || '';
  refreshCredURL();
  openModal('credbox');
}

function refreshCredURL(){
  if(!curCred) return;
  $('#crurl').textContent = socksURL(curCred.host, curCred.port,
    $('#cruser').value.trim(), $('#crpass').value.trim());
}
$('#cruser').oninput = refreshCredURL;
$('#crpass').oninput = refreshCredURL;

$('#crrand').onclick = () => {
  // 客户端和服务端都要能识别，只用无歧义、无需转义的字符
  const abc = 'abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789';
  const gen = n => Array.from(crypto.getRandomValues(new Uint8Array(n)))
    .map(v => abc[v % abc.length]).join('');
  $('#cruser').value = 'hb' + gen(6);
  $('#crpass').value = gen(14);
  refreshCredURL();
};

$('#crcopy').onclick = () => { copy($('#crurl').textContent); };

$('#crsave').onclick = async e => {
  if(!curCred) return;
  const btn = e.target; btn.disabled = true;
  const q = new URLSearchParams({
    slot: curCred.slot,
    user: $('#cruser').value.trim(),
    pass: $('#crpass').value.trim(),
  });
  try{
    const r = await api('/api/cred?' + q, {method:'POST'});
    $('#cruser').value = r.user;
    $('#crpass').value = r.pass;
    refreshCredURL();
    toast('已保存，立即生效');
    poll();
  }catch(err){ toast(err.message, true); }
  btn.disabled = false;
};

// ---- 导出 ----
$('#exportAll').onclick = async () => {
  const ids = view.exits.flatMap(x => (x.inbounds || []).map(i => i.id));
  if(!ids.length){ toast('还没有节点可导出', true); return; }
  $('#exbox').value = '读取中…';
  $('#excount').textContent = '';
  openModal('export');
  try{
    const d = await api('/api/xui/links?ids=' + ids.join(','));
    $('#exbox').value = (d.links || []).join('\n');
    $('#excount').textContent = (d.links || []).length + ' 条';
  }catch(err){ $('#exbox').value = '导出失败: ' + err.message; }
};
$('#copyall').onclick = () => { const v = $('#exbox').value; if(v) copy(v); };

// ---- 订阅 ----
// 地址用 location.origin 拼：后端给的是访问路径下的相对部分，
// 这样反代、改端口、换路径之后拿到的都是用户此刻真正能访问的地址。
async function showSub(d){
  $('#suburl').value = location.origin + d.path;
  const n = view ? view.exits.flatMap(x => (x.inbounds || [])).length : 0;
  $('#subcount').textContent = n ? n + ' 个节点' : '还没有节点，先开出口再建节点链接';
}
$('#subBtn').onclick = async () => {
  $('#suburl').value = '读取中…';
  $('#subcount').textContent = '';
  openModal('subbox');
  try{ showSub(await api('/api/sub')); }
  catch(err){ $('#suburl').value = '读取失败: ' + err.message; }
};
$('#subcopy').onclick = () => { const v = $('#suburl').value; if(v) copy(v); };
$('#subreset').onclick = async e => {
  if(!confirm('换一串口令？旧地址立刻失效，已经配过的客户端要重新填一次。')) return;
  e.target.disabled = true;
  try{
    showSub(await api('/api/sub/reset', {method:'POST'}));
    toast('已换新地址');
  }catch(err){ toast(err.message, true); }
  e.target.disabled = false;
};

// ---- 设置：改密码 / 改路径 / 改端口 / 改本地监听 ----
let curSettings = null;
let curBackend = null;

// 后端切换：把本机能用的模式列出来，装了的可选，没装的置灰并说明原因
async function loadBackendModes(){
  const sel = $('#setBackend');
  const hint = $('#setBackendHint');
  try{
    const m = await api('/api/panel/mode');
    curBackend = m.mode || '';
    sel.innerHTML = '<option value="">自动（按本机装了什么挑）</option>'
      + (m.modes || []).map(x =>
          '<option value="' + esc(x.mode) + '"' + (x.available ? '' : ' disabled')
          + '>' + esc(x.label) + (x.available ? '' : '（没装）') + '</option>').join('');
    sel.value = curBackend;
    const bad = (m.modes || []).filter(x => !x.available);
    hint.textContent = m.describe
      ? ('当前：' + m.describe + (bad.length ? '。灰掉的是本机没装的。' : ''))
      : '节点从哪来。装了 3x-ui 就能直接接管，没有就用自建。';
  }catch(err){
    sel.innerHTML = '<option value="">读取失败</option>';
    hint.textContent = err.message;
  }
}

async function loadSettings(){
  $('#setPw').value = '';
  $('#setPath').value = '';
  $('#setPathHint').textContent = '读取中…';
  loadBackendModes();
  try{
    const s = await api('/api/settings');
    curSettings = s;
    $('#setPath').value = (s.base_path || '').replace(/^\//, '');
    $('#setPort').value = s.port || '';
    $('#setListen').value = s.listen_addr || '0.0.0.0';
    $('#setResi').checked = s.residential_only !== false;
    $('#setCert').value = s.tls_cert || '';
    $('#setKey').value = s.tls_key || '';
    const ti = s.tls_info;
    const tiel = $('#setTLSInfo');
    if(ti){
      tiel.hidden = false;
      tiel.textContent = '证书：' + (ti.cn || '未知域名') + '，有效期至 ' + ti.not_after
        + '（还剩 ' + ti.days_left + ' 天）';
      tiel.classList.toggle('bad', ti.days_left < 14);
    } else {
      tiel.hidden = true;
      tiel.classList.remove('bad');
    }
    $('#setPathHint').textContent = '界面挂在这个路径下，扫端口的探不到。只能用字母数字和 - _。';
    $('#updCur').textContent = s.version || '-';
    $('#updLatest').textContent = '';
    $('#updNotes').hidden = true;
    $('#updApply').hidden = true;
    $('#updCheck').disabled = false;
    $('#updCheck').textContent = '检查更新';
  }catch(err){ $('#setPathHint').textContent = '读取失败: ' + err.message; }
};

// 检查更新：问后端 GitHub 最新版，有新版就亮出更新按钮和更新内容
$('#updCheck').onclick = async e => {
  e.target.disabled = true;
  e.target.textContent = '检查中…';
  try{
    const u = await api('/api/update/check');
    $('#updCur').textContent = u.current || '-';
    if(u.has_update){
      $('#updLatest').textContent = '有新版本 ' + u.latest;
      $('#updApplyVer').textContent = u.latest;
      $('#updApply').hidden = false;
      $('#updNotes').textContent = u.notes || '（这个版本没写更新说明）';
      $('#updNotes').hidden = false;
    } else {
      $('#updLatest').textContent = '已是最新';
      $('#updApply').hidden = true;
      $('#updNotes').hidden = true;
    }
  }catch(err){ toast(err.message, true); }
  e.target.disabled = false;
  e.target.textContent = '检查更新';
};

// 重启面板：和 3x-ui 的重启面板一样，服务重启后自动刷新页面
$('#panelRestart').onclick = async e => {
  if(!confirm('重启面板？出口会短暂断开后自动重连。')) return;
  e.target.disabled = true;
  e.target.textContent = '重启中…';
  try{
    await api('/api/restart', {method:'POST'});
    toast('面板正在重启，几秒后自动刷新');
    setTimeout(() => location.reload(), 6000);
  }catch(err){
    toast(err.message, true);
    e.target.disabled = false;
    e.target.textContent = '重启面板';
  }
};

// 一键更新：后端下载替换二进制并重启服务，进程重启期间界面会短暂断连
$('#updApply').onclick = async e => {
  if(!confirm('更新到 ' + $('#updApplyVer').textContent + '？服务会重启，界面会短暂断开。')) return;
  e.target.disabled = true;
  e.target.textContent = '更新中…';
  try{
    const r = await api('/api/update/apply', {method:'POST'});
    if(r.restarting){
      $('#updNotes').textContent = '已下载新版本，服务正在重启，几秒后刷新页面即可。';
      $('#updNotes').hidden = false;
      toast('更新中，服务重启后刷新页面');
      // 给服务重启留点时间再自动刷新
      setTimeout(() => location.reload(), 6000);
    } else {
      toast(r.message || '已是最新版');
      e.target.disabled = false;
      e.target.textContent = '更新到 ' + $('#updApplyVer').textContent;
    }
  }catch(err){
    toast(err.message, true);
    e.target.disabled = false;
    e.target.textContent = '更新到 ' + $('#updApplyVer').textContent;
  }
};

// 端口/监听地址变了要提示用户之后从新地址进；密码/路径可原地生效
function nextURL(port, listen, path, tls){
  const host = (listen && listen !== '0.0.0.0') ? listen : location.hostname;
  // 按切换后的新状态拼协议：刚关掉 HTTPS 时 location.protocol 还是 https:，不能直接用
  const proto = tls ? 'https:' : 'http:';
  return proto + '//' + host + ':' + port + (path ? '/' + path : '') + '/';
}

$('#setSave').onclick = async e => {
  e.target.disabled = true;
  const body = {};
  const pw = $('#setPw').value.trim();
  if(pw) body.password = pw;
  body.base_path = $('#setPath').value.trim();
  const port = parseInt($('#setPort').value.trim(), 10);
  if(port) body.port = port;
  body.listen_addr = $('#setListen').value;
  body.residential_only = $('#setResi').checked;
  const tlsCert = $('#setCert').value.trim();
  const tlsKey = $('#setKey').value.trim();
  body.tls_cert = tlsCert;
  body.tls_key = tlsKey;
  const tlsOn = !!(tlsCert && tlsKey);

  const portChanged = curSettings && (port !== curSettings.port
    || body.listen_addr !== (curSettings.listen_addr || '0.0.0.0'));
  const tlsChanged = curSettings && (tlsCert !== (curSettings.tls_cert || '')
    || tlsKey !== (curSettings.tls_key || ''));
  const addrChanged = portChanged || tlsChanged;

  try{
    // 后端和其它设置分属两个接口，先切后端：切失败就别继续，免得用户以为整单都生效了
    const backend = $('#setBackend').value;
    if(curBackend !== null && backend !== curBackend){
      const r = await api('/api/panel/mode', {
        method:'POST',
        headers:{'Content-Type':'application/json'},
        body: JSON.stringify({mode: backend}),
      });
      curBackend = r.mode || '';
      $('#setBackendHint').textContent = '当前：' + (r.describe || r.kind || '已切换');
    }
    await api('/api/settings', {
      method:'POST',
      headers:{'Content-Type':'application/json'},
      body: JSON.stringify(body),
    });
    if(addrChanged){
      const url = nextURL(port, body.listen_addr, body.base_path, tlsOn);
      $('#setPortHint').innerHTML = '监听已切换，请从新地址打开：<a href="' + esc(url) + '">' + esc(url) + '</a>';
      toast('监听已切换，用新地址重新打开');
      // 端口/协议变了当前连接会断，不自动跳转，让用户看清新地址
    } else {
      toast('已保存');
      // 路径可能变了，重新加载到新路径下
      const np = body.base_path;
      const cur = (curSettings && curSettings.base_path || '').replace(/^\//, '');
      if(np !== cur){
        location.href = location.protocol + '//' + location.host
          + (np ? '/' + np : '') + '/';
        return;
      }
      poll();
    }
  }catch(err){ toast(err.message, true); }
  e.target.disabled = false;
};

poll();
setInterval(poll, 3000);
// 后台标签页会被浏览器节流，回前台/聚焦时补刷一次
document.addEventListener('visibilitychange', () => { if(!document.hidden) poll(); });
window.addEventListener('focus', poll);
</script>
</body>
</html>`
