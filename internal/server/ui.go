package server

import "net/http"

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(dashHTML))
}

const dashHTML = `<!DOCTYPE html>
<html lang="en"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Assay</title>
<style>
:root{--bg:#1a1410;--bg2:#241e18;--bg3:#2e261e;--rust:#c45d2c;--rl:#e8753a;--leather:#a0845c;--ll:#c4a87a;--cream:#f0e6d3;--cd:#bfb5a3;--cm:#7a7060;--gold:#d4a843;--green:#4a9e5c;--red:#c44040;--blue:#4a7ec4;--mono:'JetBrains Mono',Consolas,monospace;--serif:'Libre Baskerville',Georgia,serif}
*{margin:0;padding:0;box-sizing:border-box}body{background:var(--bg);color:var(--cream);font-family:var(--mono);font-size:13px;line-height:1.6}
.hdr{padding:.6rem 1.2rem;border-bottom:1px solid var(--bg3);display:flex;justify-content:space-between;align-items:center}
.hdr h1{font-family:var(--serif);font-size:1rem}.hdr h1 span{color:var(--rl)}
.main{max-width:900px;margin:0 auto;padding:1rem 1.2rem}
.btn{font-family:var(--mono);font-size:.68rem;padding:.3rem .6rem;border:1px solid;cursor:pointer;background:transparent;transition:.15s;white-space:nowrap}
.btn-p{border-color:var(--rust);color:var(--rl)}.btn-p:hover{background:var(--rust);color:var(--cream)}
.btn-d{border-color:var(--bg3);color:var(--cm)}.btn-d:hover{border-color:var(--red);color:var(--red)}
.btn-s{border-color:var(--green);color:var(--green)}.btn-s:hover{background:var(--green);color:var(--bg)}
.suite-card{background:var(--bg2);border:1px solid var(--bg3);padding:.7rem;margin-bottom:.4rem;cursor:pointer;transition:.1s}
.suite-card:hover{background:var(--bg3)}
.suite-card h3{font-size:.8rem;margin-bottom:.2rem;display:flex;align-items:center;gap:.4rem}
.suite-meta{font-size:.65rem;color:var(--cm);display:flex;gap:.7rem}
.pass-rate{font-weight:600}.pass-rate.good{color:var(--green)}.pass-rate.bad{color:var(--red)}
.method{font-size:.6rem;padding:.05rem .25rem;border-radius:2px;font-weight:600}
.m-GET{background:rgba(74,158,92,.15);color:var(--green)}.m-POST{background:rgba(74,126,196,.15);color:var(--blue)}.m-PUT{background:rgba(212,168,67,.15);color:var(--gold)}.m-DELETE{background:rgba(196,64,64,.15);color:var(--red)}
.test-row{display:flex;align-items:center;gap:.5rem;padding:.3rem .5rem;border-bottom:1px solid var(--bg3);font-size:.72rem}
.test-row .test-name{flex:1}
.result-row{display:flex;align-items:center;gap:.5rem;padding:.25rem .5rem;border-bottom:1px solid var(--bg3);font-size:.72rem}
.r-pass{color:var(--green)}.r-fail{color:var(--red)}.r-error{color:var(--gold)}
.modal-bg{position:fixed;top:0;left:0;right:0;bottom:0;background:rgba(0,0,0,.65);display:flex;align-items:center;justify-content:center;z-index:100}
.modal{background:var(--bg2);border:1px solid var(--bg3);padding:1.5rem;width:95%;max-width:700px;max-height:90vh;overflow-y:auto}
.modal h2{font-family:var(--serif);font-size:.95rem;margin-bottom:1rem}
label.fl{display:block;font-size:.65rem;color:var(--leather);text-transform:uppercase;letter-spacing:1px;margin-bottom:.2rem;margin-top:.5rem}
input[type=text],input[type=number],textarea,select{background:var(--bg);border:1px solid var(--bg3);color:var(--cream);padding:.35rem .5rem;font-family:var(--mono);font-size:.78rem;width:100%;outline:none}
textarea{resize:vertical;min-height:60px}
.form-row{display:flex;gap:.5rem}.form-row>*{flex:1}
.empty{text-align:center;padding:2rem;color:var(--cm);font-style:italic;font-family:var(--serif)}
</style>
<link href="https://fonts.googleapis.com/css2?family=Libre+Baskerville:ital@0;1&family=JetBrains+Mono:wght@400;600&display=swap" rel="stylesheet">
</head><body>
<div class="hdr"><h1><span>Assay</span></h1><button class="btn btn-p" onclick="showNewSuite()">+ Suite</button></div>
<div class="main"><div id="suiteList"></div><div id="detail" style="display:none;margin-top:1rem"></div></div>
<div id="modal"></div>
<script>
let suites=[],curSuite=null;
async function api(url,opts){return(await fetch(url,opts)).json()}
function esc(s){return String(s||'').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;')}
function timeAgo(d){if(!d)return'never';const s=Math.floor((Date.now()-new Date(d))/1e3);if(s<60)return s+'s ago';if(s<3600)return Math.floor(s/60)+'m ago';return Math.floor(s/3600)+'h ago'}

async function init(){const d=await api('/api/suites');suites=d.suites||[];renderSuites()}
function renderSuites(){
  document.getElementById('suiteList').innerHTML=suites.length?suites.map(s=>{
    const pCls=s.pass_rate>=100?'good':s.pass_rate>0?'':'bad';
    return '<div class="suite-card" onclick="openSuite(\''+s.id+'\')"><h3>'+esc(s.name)+'</h3>'+
      '<div class="suite-meta"><span>'+s.test_count+' tests</span><span class="pass-rate '+pCls+'">'+s.pass_rate.toFixed(0)+'% pass</span>'+
      '<span>Last run: '+timeAgo(s.last_run)+'</span>'+(s.base_url?'<span>'+esc(s.base_url)+'</span>':'')+'</div></div>'}).join(''):'<div class="empty">No test suites yet.</div>'
}

async function openSuite(id){
  curSuite=id;const[su,td,rd]=await Promise.all([api('/api/suites/'+id),api('/api/suites/'+id+'/tests'),api('/api/suites/'+id+'/runs')]);
  const tests=td.tests||[],runs=rd.runs||[];
  const testHTML=tests.length?tests.map(t=>
    '<div class="test-row"><span class="method m-'+t.method+'">'+t.method+'</span><span class="test-name">'+esc(t.name)+'</span>'+
    '<span style="color:var(--cm)">'+esc(t.path)+'</span><span style="color:var(--cm)">→ '+t.expect_code+'</span>'+
    '<span style="cursor:pointer;color:var(--cm);font-size:.6rem" onclick="editTest(\''+t.id+'\')">edit</span>'+
    '<span style="cursor:pointer;color:var(--cm);font-size:.6rem" onclick="delTest(\''+t.id+'\')">del</span></div>').join(''):'<div style="font-size:.75rem;color:var(--cm);padding:.5rem">No tests yet.</div>';
  const runHTML=runs.length?runs.slice(0,5).map(r=>
    '<div class="result-row"><span class="r-'+r.status+'">'+r.status.toUpperCase()+'</span>'+
    '<span>'+r.passed+'/'+((r.passed||0)+(r.failed||0))+' passed</span><span style="color:var(--cm)">'+r.total_ms+'ms</span>'+
    '<span style="color:var(--cm);cursor:pointer" onclick="showRun(\''+r.id+'\')">details</span>'+
    '<span style="color:var(--cm)">'+timeAgo(r.created_at)+'</span></div>').join(''):'';
  document.getElementById('detail').style.display='block';
  document.getElementById('detail').innerHTML=
    '<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:.5rem">'+
      '<div style="font-size:.75rem;color:var(--leather)">'+esc(su.name)+' · '+su.test_count+' tests</div>'+
      '<div style="display:flex;gap:.3rem"><button class="btn btn-s" onclick="runTests(\''+id+'\')">▶ Run All</button><button class="btn btn-p" onclick="showNewTest(\''+id+'\')">+ Test</button><button class="btn btn-d" onclick="if(confirm(\'Delete suite?\'))delSuite(\''+id+'\')">Del</button></div>'+
    '</div>'+testHTML+
    (runHTML?'<div style="font-size:.7rem;color:var(--leather);margin:1rem 0 .3rem">Recent Runs</div>'+runHTML:'');
}

async function runTests(sid){
  const r=await api('/api/suites/'+sid+'/run',{method:'POST'});
  showRun(r.id);openSuite(sid);init()
}

async function showRun(id){
  const r=await api('/api/runs/'+id);
  document.getElementById('modal').innerHTML='<div class="modal-bg" onclick="if(event.target===this)closeModal()"><div class="modal">'+
    '<h2>Run Results <span class="r-'+r.status+'">'+r.status.toUpperCase()+'</span></h2>'+
    '<div style="font-size:.7rem;color:var(--cm);margin-bottom:.5rem">'+r.passed+' passed, '+r.failed+' failed · '+r.total_ms+'ms total</div>'+
    (r.results||[]).map(t=>'<div class="result-row"><span class="r-'+t.status+'" style="width:35px">'+t.status+'</span><span style="flex:1">'+esc(t.test_name)+'</span>'+
      '<span style="color:var(--cm)">HTTP '+t.status_code+'</span><span style="color:var(--cm)">'+t.resp_time_ms+'ms</span>'+
      (t.error?'<span style="color:var(--red);font-size:.6rem">'+esc(t.error)+'</span>':'')+
    '</div>').join('')+
    '<button class="btn btn-d" style="margin-top:.5rem" onclick="closeModal()">Close</button></div></div>';
}

function showNewSuite(){
  document.getElementById('modal').innerHTML='<div class="modal-bg" onclick="if(event.target===this)closeModal()"><div class="modal">'+
    '<h2>New Suite</h2><label class="fl">Name</label><input type="text" id="ns-name">'+
    '<label class="fl">Base URL</label><input type="text" id="ns-url" placeholder="https://api.example.com">'+
    '<div style="display:flex;gap:.5rem;margin-top:1rem"><button class="btn btn-p" onclick="saveNewSuite()">Create</button><button class="btn btn-d" onclick="closeModal()">Cancel</button></div></div></div>';
}
async function saveNewSuite(){
  const body={name:document.getElementById('ns-name').value,base_url:document.getElementById('ns-url').value};
  if(!body.name){alert('Name required');return}
  await api('/api/suites',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});closeModal();init()
}

function showNewTest(sid){
  document.getElementById('modal').innerHTML='<div class="modal-bg" onclick="if(event.target===this)closeModal()"><div class="modal">'+
    '<h2>New Test</h2><label class="fl">Name</label><input type="text" id="nt-name" placeholder="GET /health returns 200">'+
    '<div class="form-row"><div><label class="fl">Method</label><select id="nt-method"><option>GET</option><option>POST</option><option>PUT</option><option>DELETE</option><option>PATCH</option></select></div>'+
    '<div><label class="fl">Path</label><input type="text" id="nt-path" value="/"></div>'+
    '<div><label class="fl">Expect Status</label><input type="number" id="nt-code" value="200"></div></div>'+
    '<label class="fl">Request Body (JSON)</label><textarea id="nt-body" rows="2"></textarea>'+
    '<label class="fl">Expect Body Contains</label><input type="text" id="nt-expect" placeholder="optional substring">'+
    '<div style="display:flex;gap:.5rem;margin-top:1rem"><button class="btn btn-p" onclick="saveNewTest(\''+sid+'\')">Create</button><button class="btn btn-d" onclick="closeModal()">Cancel</button></div></div></div>';
}
async function saveNewTest(sid){
  const body={name:document.getElementById('nt-name').value,method:document.getElementById('nt-method').value,path:document.getElementById('nt-path').value,expect_code:parseInt(document.getElementById('nt-code').value)||200,body:document.getElementById('nt-body').value,expect_body:document.getElementById('nt-expect').value};
  if(!body.name){alert('Name required');return}
  await api('/api/suites/'+sid+'/tests',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});closeModal();openSuite(sid);init()
}
function editTest(id){/* simplified: delete and recreate */}
async function delTest(id){await api('/api/tests/'+id,{method:'DELETE'});openSuite(curSuite)}
async function delSuite(id){await api('/api/suites/'+id,{method:'DELETE'});curSuite=null;document.getElementById('detail').style.display='none';init()}
function closeModal(){document.getElementById('modal').innerHTML=''}
init()
</script></body></html>`
