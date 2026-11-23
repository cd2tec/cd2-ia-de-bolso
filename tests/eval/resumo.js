'use strict';
// Bateria do resumo do alerta (Fase 2), contra o modelo de verdade. O brief (seção 7) pede a bateria
// antes de liberar a fase, com as métricas "resumos sem número inventado" e "latência no hardware mínimo
// (8 GB de memória, sem placa de vídeo)". O brief não fixa limites: este script só mede e mostra; quem
// decide se basta é o dono do produto.
//
// Os casos (resumo-casos.json) são consultas salvas com condição que dispara e o resultado delas, tirados
// do banco fictício erp_pdv (~/work/cd2/erp-pdv-mock, gerar_casos_resumo.py). Não precisa de banco:
// só do modelo no ar, de preferência só para a bateria (a IA aberta na tela disputa o modelo).
//
//   node tests/eval/resumo.js
//   CD2_EVAL_MODEL_URL=http://127.0.0.1:8081/v1/chat/completions   (padrão)
//   CD2_EVAL_SAIDA=arquivo   grava o relatório completo em JSON
//   CD2_EVAL_SO=01,02        roda só esses casos
const fs = require('fs');
const path = require('path');
const C = require('../../web/condicao.js');
const R = require('../../web/resumo.js');

const URL = process.env.CD2_EVAL_MODEL_URL || 'http://127.0.0.1:8081/v1/chat/completions';
// Prazo do cd2-ctl (PrazoResumo em ctl/internal/agenda/resumo.go): só para contar quantos passariam dele.
const PRAZO_CTL_MS = 25000;
// Só para não ficar preso num pedido que não volta; não é critério.
const DESISTE_MS = 300000;
const SO = (process.env.CD2_EVAL_SO || '').split(',').map(s => s.trim()).filter(Boolean);

async function chamar(p){
  const body = { model:'cd2', messages:p.mensagens, stream:false, temperature:0, max_tokens:p.maxTokens,
    response_format:{ type:'json_schema', json_schema:{ name:'resumo', strict:true, schema:p.schema } } };
  const ctl = new AbortController(), timer = setTimeout(() => ctl.abort(), DESISTE_MS);
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
    rel.push(item);
    const marca = item.falha ? '✗ ' + item.falha + (item.detalhe ? ' (' + item.detalhe + ')' : '') : '✓';
    console.log(c.id + '  ' + marca + '  ' + (item.ms / 1000).toFixed(1) + ' s  ' + c.rotulo);
    if(item.linhas) item.linhas.forEach(l => console.log('      ' + l));
  }

  // Métricas do brief: resumos sem número inventado e latência. O resto é para entender as falhas.
  const respondidos = rel.filter(r => r.falha !== 'modelo' && r.falha !== 'nao_disparou');
  const formato = respondidos.filter(r => r.falha === 'formato').length;
  const legiveis = respondidos.length - formato;
  const inventado = respondidos.filter(r => r.falha === 'numero').length;
  const ms = rel.filter(r => r.ms != null && r.falha !== 'modelo').map(r => r.ms);
  const media = ms.length ? Math.round(ms.reduce((a, b) => a + b, 0) / ms.length) : 0;
  const pior = ms.length ? Math.max.apply(null, ms) : 0;
  const s1 = x => (x / 1000).toFixed(1) + ' s';
  console.log('\nCasos: ' + rel.length + ' · modelo respondeu: ' + respondidos.length + ' · fora do formato: ' + formato);
  console.log('Resumos sem número inventado: ' + (legiveis - inventado) + ' de ' + legiveis + ' (' + pct(legiveis - inventado, legiveis) + ')');
  console.log('Latência: média ' + s1(media) + ' · p90 ' + s1(quantil(ms, 0.9)) + ' · pior ' + s1(pior)
    + ' · acima do prazo do cd2-ctl (' + PRAZO_CTL_MS / 1000 + ' s): ' + ms.filter(x => x > PRAZO_CTL_MS).length);
  const erros = rel.filter(r => r.falha === 'modelo' || r.falha === 'nao_disparou').length;
  if(erros) console.log('Sem resposta do modelo ou caso mal montado: ' + erros);
  if(process.env.CD2_EVAL_SAIDA){
    fs.writeFileSync(process.env.CD2_EVAL_SAIDA, JSON.stringify({ url:URL, total:rel.length, respondidos:respondidos.length, formato, semNumeroInventado:legiveis - inventado, legiveis, mediaMs:media, p90Ms:quantil(ms, 0.9), piorMs:pior, casos:rel }, null, 1));
  }
}

main().catch(e => { console.error(e); process.exitCode = 2; });
