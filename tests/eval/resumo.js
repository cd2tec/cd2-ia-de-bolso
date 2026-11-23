'use strict';
// Bateria do resumo do alerta (Fase 2), contra o modelo de verdade. O brief pede esta bateria antes de
// liberar a fase: mede quantos resumos passam na conferência dos números e quanto o modelo leva.
//
// Os casos (resumo-casos.json) são consultas salvas com condição que dispara e o resultado delas, tirados
// do banco fictício erp_pdv (~/work/cd2/erp-pdv-mock, gerar_casos_resumo.py). Não precisa de banco:
// só do modelo no ar (o da IA de Bolso aberta serve).
//
//   node tests/eval/resumo.js
//   CD2_EVAL_MODEL_URL=http://127.0.0.1:8081/v1/chat/completions   (padrão)
//   CD2_EVAL_MIN_OK=0.9      aprovação mínima (fração dos casos) para passar
//   CD2_EVAL_MAX_MS=25000    o pior tempo aceito (o prazo do cd2-ctl)
//   CD2_EVAL_SAIDA=arquivo   grava o relatório completo em JSON
//   CD2_EVAL_SO=01,02        roda só esses casos
const fs = require('fs');
const path = require('path');
const C = require('../../web/condicao.js');
const R = require('../../web/resumo.js');

const URL = process.env.CD2_EVAL_MODEL_URL || 'http://127.0.0.1:8081/v1/chat/completions';
const MIN_OK = Number(process.env.CD2_EVAL_MIN_OK || 0.9);
const MAX_MS = Number(process.env.CD2_EVAL_MAX_MS || 25000);
const SO = (process.env.CD2_EVAL_SO || '').split(',').map(s => s.trim()).filter(Boolean);

async function chamar(p){
  const body = { model:'cd2', messages:p.mensagens, stream:false, temperature:0, max_tokens:p.maxTokens,
    response_format:{ type:'json_schema', json_schema:{ name:'resumo', strict:true, schema:p.schema } } };
  const ctl = new AbortController(), timer = setTimeout(() => ctl.abort(), MAX_MS * 4);
  try{
    const r = await fetch(URL, { method:'POST', headers:{ 'Content-Type':'application/json' }, body:JSON.stringify(body), signal:ctl.signal });
    if(!r.ok) throw new Error('HTTP ' + r.status);
    const j = await r.json();
    return (j.choices && j.choices[0] && j.choices[0].message && j.choices[0].message.content) || '';
  }finally{ clearTimeout(timer); }
}

const pct = (a, b) => b ? Math.round(a / b * 1000) / 10 + '%' : '-';
function quantil(xs, q){ const s = xs.slice().sort((a, b) => a - b); return s.length ? s[Math.min(s.length - 1, Math.floor(q * s.length))] : 0; }

async function main(){
  let casos = JSON.parse(fs.readFileSync(path.join(__dirname, 'resumo-casos.json'), 'utf8'));
  if(SO.length) casos = casos.filter(c => SO.includes(c.id));
  console.log('Resumo do alerta: ' + casos.length + ' casos em ' + URL + '\n');
  const rel = [];
  for(const c of casos){
    const res = { ok:true, colunas:c.colunas, todas:c.linhas, linhas:c.linhas };
    const j = C.julgar(c.consulta.agenda.condicao, res, c.anteriores);
    const p = R.pedido(c.consulta, res, j);
    const item = { id:c.id, rotulo:c.rotulo, alerta:j && j.texto };
    if(!p){ item.falha = 'nao_disparou'; rel.push(item); console.log(c.id + '  ✗ não disparou (caso mal montado)'); continue; }
    const ini = Date.now();
    let texto = '';
    try{ texto = await chamar(p); }catch(e){ item.falha = 'modelo'; item.detalhe = String(e.message || e); }
    item.ms = Date.now() - ini;
    item.resposta = texto;
    if(!item.falha){
      const conf = R.conferir(texto, c.consulta, res, j);
      if(conf.ok) item.linhas = conf.linhas;
      else { item.falha = conf.motivo; item.detalhe = conf.detalhe; }
    }
    if(!item.falha && item.ms > MAX_MS){ item.falha = 'prazo'; item.detalhe = item.ms + ' ms'; }
    rel.push(item);
    const marca = item.falha ? '✗ ' + item.falha + (item.detalhe ? ' (' + item.detalhe + ')' : '') : '✓';
    console.log(c.id + '  ' + marca + '  ' + (item.ms / 1000).toFixed(1) + ' s  ' + c.rotulo);
    if(item.linhas) item.linhas.forEach(l => console.log('      ' + l));
  }

  const ok = rel.filter(r => !r.falha).length, ms = rel.filter(r => r.ms != null).map(r => r.ms);
  const porMotivo = {};
  rel.filter(r => r.falha).forEach(r => { porMotivo[r.falha] = (porMotivo[r.falha] || 0) + 1; });
  const media = ms.length ? Math.round(ms.reduce((a, b) => a + b, 0) / ms.length) : 0;
  const pior = ms.length ? Math.max.apply(null, ms) : 0;
  console.log('\nAprovados: ' + ok + ' de ' + rel.length + ' (' + pct(ok, rel.length) + ')');
  Object.keys(porMotivo).forEach(m => console.log('  ' + m + ': ' + porMotivo[m]));
  console.log('Tempo: média ' + (media / 1000).toFixed(1) + ' s · p90 ' + (quantil(ms, 0.9) / 1000).toFixed(1) + ' s · pior ' + (pior / 1000).toFixed(1) + ' s');
  const passou = rel.length > 0 && ok / rel.length >= MIN_OK && pior <= MAX_MS;
  console.log(passou ? '\nPASSOU (mínimo ' + pct(MIN_OK, 1) + ', prazo ' + MAX_MS / 1000 + ' s)'
                     : '\nNÃO PASSOU (mínimo ' + pct(MIN_OK, 1) + ', prazo ' + MAX_MS / 1000 + ' s)');
  if(process.env.CD2_EVAL_SAIDA){
    fs.writeFileSync(process.env.CD2_EVAL_SAIDA, JSON.stringify({ url:URL, aprovados:ok, total:rel.length, porMotivo, mediaMs:media, piorMs:pior, casos:rel }, null, 1));
  }
  process.exitCode = passou ? 0 : 1;
}

main().catch(e => { console.error(e); process.exitCode = 2; });
