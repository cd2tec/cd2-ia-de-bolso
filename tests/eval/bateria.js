'use strict';
// Bateria de avaliação do mini agente (História 6, CIDB-26), contra o modelo de verdade. Roda os dois
// agentes que ela libera, do jeito que o cd2-ctl roda (agendador.go, aprofundar.go, resumo.go):
//   - resumo do alerta (História 3): os 30 casos de resumo-casos.json;
//   - aprofundamento (História 4): os 30 casos de aprofundamento-casos.json. O modelo escolhe uma das 3
//     consultas vinculadas (thinking ligado, como na CIDB-17) e o resumo sai com o resultado da escolhida.
// Mesmas funções do produto (web/condicao.js, web/aprofundar.js, web/resumo.js), mesmo pedido (temperatura 0,
// JSON schema), mesmas tentativas. Sem prazo aqui: mede o tempo de cada tentativa e conta depois o que
// caberia nos prazos do cd2-ctl. A consulta de aprofundamento não roda: o resultado de cada opção já está
// no caso (o tempo dela depende do servidor do banco, não do computador da pessoa).
//
// Também roda os controles (sem modelo, com resposta fixa): um resumo com número inventado (H6-CT05) e uma
// escolha errada (H6-CT06). Eles têm que ser pegos e contados; ficam fora dos totais.
//
//   node tests/eval/bateria.js
//   CD2_EVAL_MODEL_URL=http://127.0.0.1:8091/v1/chat/completions   (padrão)
//   CD2_EVAL_ROTULO=qwen3-dev-1     nome da rodada (vai no relatório)
//   CD2_EVAL_SAIDA=arquivo.json     grava o relatório completo
//   CD2_EVAL_SO=01,A03              roda só esses casos
const fs = require('fs');
const os = require('os');
const path = require('path');
const C = require('../../web/condicao.js');
const R = require('../../web/resumo.js');
const A = require('../../web/aprofundar.js');
const M = require('./metricas.js');

const URL = process.env.CD2_EVAL_MODEL_URL || 'http://127.0.0.1:8091/v1/chat/completions';
const ROTULO = process.env.CD2_EVAL_ROTULO || 'sem-rotulo';
const SO = (process.env.CD2_EVAL_SO || '').split(',').map(s => s.trim()).filter(Boolean);
const DESISTE_MS = 300000; // só para não ficar preso num pedido que não volta; não é critério

async function chamar(p, mensagens, nome, thinking){
  const body = { model:'cd2', messages:mensagens, stream:false, temperature:0, max_tokens:p.maxTokens,
    chat_template_kwargs:{ enable_thinking:thinking },
    response_format:{ type:'json_schema', json_schema:{ name:nome, strict:true, schema:p.schema } } };
  const ctl = new AbortController(), timer = setTimeout(() => ctl.abort(), DESISTE_MS);
  try{
    const r = await fetch(URL, { method:'POST', headers:{ 'Content-Type':'application/json' }, body:JSON.stringify(body), signal:ctl.signal });
    if(!r.ok) throw new Error('HTTP ' + r.status);
    const j = await r.json();
    return { texto:(j.choices && j.choices[0] && j.choices[0].message && j.choices[0].message.content) || '', uso:j.usage || null, tempos:j.timings || null };
  }finally{ clearTimeout(timer); }
}

const resultado = (colunas, linhas) => ({ ok:true, colunas, todas:linhas, linhas });

// Tentativas como no cd2-ctl: a segunda só quando a conferência recusa; erro do modelo não tenta de novo.
// fixa: resposta sem modelo (controles).
async function tentar(p, nome, thinking, conferir, deNovo, max, fixa){
  let msgs = p.mensagens;
  const tentativas = [];
  for(;;){
    const ini = Date.now();
    let r;
    try{ r = fixa != null ? { texto:fixa } : await chamar(p, msgs, nome, thinking); }
    catch(e){ tentativas.push({ ms:Date.now() - ini, erro:String(e.message || e) }); return { falha:'modelo', tentativas }; }
    const conf = conferir(r.texto);
    tentativas.push({ ms:Date.now() - ini, resposta:r.texto, ok:conf.ok, motivo:conf.motivo, detalhe:conf.detalhe,
      tokens:r.uso ? { entrada:r.uso.prompt_tokens, saida:r.uso.completion_tokens } : undefined });
    if(conf.ok) return { conf, tentativas };
    if(tentativas.length >= max || fixa != null) return { falha:conf.motivo, detalhe:conf.detalhe, tentativas };
    msgs = deNovo(msgs, r.texto, conf);
  }
}

async function rodarResumo(id, consulta, res, j, aprof, fixa){
  const p = R.pedido(consulta, res, j, aprof);
  if(!p) return { id, falha:'nao_disparou', tentativas:[] };
  const t = await tentar(p, 'resumo', false, tx => R.conferir(tx, consulta, res, j, aprof), R.novaTentativa, R.MAX_TENTATIVAS, fixa);
  const item = { id, tentativas:t.tentativas };
  if(t.falha){ item.falha = t.falha; item.detalhe = t.detalhe; return item; }
  item.linhas = t.conf.linhas;
  item.acao = t.conf.acao;
  item.frasesTrocadas = M.frasesTrocadas(item.linhas, res.todas, consulta.y);
  return item;
}

async function casoResumo(c){
  const res = resultado(c.colunas, c.linhas);
  const j = C.julgar(c.consulta.agenda.condicao, res, c.anteriores);
  return Object.assign({ etapa:'resumo', rotulo:c.rotulo, alerta:j && j.texto }, await rodarResumo(c.id, c.consulta, res, j, null, c.fixa));
}

async function casoAprofundamento(c){
  const res = resultado(c.colunas, c.linhas);
  const j = C.julgar(c.consulta.agenda.condicao, res, c.anteriores);
  const ops = A.opcoes(c.consulta, c.opcoes.map(o => ({ id:o.id, nome:o.nome })));
  const p = A.pedido(c.consulta, j, ops);
  const esc = { etapa:'escolha', id:c.id, rotulo:c.rotulo, alerta:j && j.texto, esperada:c.esperada, aceitaveis:c.aceitaveis, controle:!!c.controle };
  if(!p){ esc.falha = 'nao_disparou'; esc.tentativas = []; return [esc]; }
  const t = await tentar(p, 'escolha', true, tx => A.conferir(tx, ops), A.novaTentativa, A.MAX_TENTATIVAS, c.fixaEscolha);
  esc.tentativas = t.tentativas;
  if(t.falha){ esc.falha = t.falha; esc.detalhe = t.detalhe; }
  else esc.escolha = t.conf.nome;
  if(c.controle) return [esc];
  // Sem escolha válida o resumo sai como na História 3 (sem aprofundamento), igual ao cd2-ctl.
  const op = esc.escolha ? c.opcoes.find(o => o.nome === esc.escolha) : null;
  const aprof = op ? { resultado:resultado(op.colunas, op.linhas) } : null;
  const rs = Object.assign({ etapa:'resumo_aprof', rotulo:c.rotulo, alerta:j && j.texto, aprofundou:op ? op.nome : null },
    await rodarResumo(c.id, c.consulta, res, j, aprof));
  return [esc, rs];
}

// ---- controles (sem modelo) ----
function controles(resumos, aprofs){
  const out = [];
  const r = resumos[0];
  if(r) out.push(Object.assign({}, r, { id:'CT05', controle:true,
    fixa:JSON.stringify({ linhas:['O faturamento chegou a 987.654,32 no período.'], acao:'nenhuma' }) }));
  const a = aprofs[0];
  if(a){
    const errada = a.opcoes.map(o => o.nome).find(n => !a.aceitaveis.includes(n));
    out.push(Object.assign({}, a, { id:'CT06', controle:true, fixaEscolha:JSON.stringify({ escolha:errada }) }));
  }
  return out;
}

function ambiente(){
  const cpus = os.cpus();
  return { rotulo:ROTULO, url:URL, quando:new Date().toISOString(), so:os.platform() + ' ' + os.release(),
    cpu:cpus.length ? cpus[0].model : '', nucleos:cpus.length, memoriaGB:Math.round(os.totalmem() / 1073741824 * 10) / 10,
    node:process.version, threads:process.env.CD2_EVAL_THREADS || null, simulacao:process.env.CD2_EVAL_SIMULACAO || null };
}

async function modeloNoAr(){
  try{
    const r = await fetch(URL.replace(/\/v1\/chat\/completions$/, '/v1/models'));
    const j = await r.json();
    return (j.data && j.data[0] && j.data[0].id) || '';
  }catch(e){ return ''; }
}

const s1 = ms => (ms / 1000).toFixed(1) + ' s';
function linha(it){
  const marca = it.falha ? '✗ ' + it.falha + (it.detalhe ? ' (' + String(it.detalhe).slice(0, 80) + ')' : '') : '✓';
  const extra = it.etapa === 'escolha' ? (it.escolha ? ' → ' + it.escolha + ((it.aceitaveis || []).includes(it.escolha) ? '' : '  [ERRADA; esperada: ' + it.esperada + ']') : '') : '';
  return (it.controle ? '[controle] ' : '') + it.id + ' ' + it.etapa.padEnd(12) + marca + '  ' + s1(M.msTotal(it)) + extra;
}

async function main(){
  const ler = f => JSON.parse(fs.readFileSync(path.join(__dirname, f), 'utf8'));
  let resumos = ler('resumo-casos.json'), aprofs = ler('aprofundamento-casos.json');
  const ctrl = controles(resumos, aprofs);
  if(SO.length){ resumos = resumos.filter(c => SO.includes(c.id)); aprofs = aprofs.filter(c => SO.includes(c.id)); }
  const amb = ambiente();
  amb.modelo = await modeloNoAr();
  console.log('Bateria ' + ROTULO + ': ' + resumos.length + ' resumos + ' + aprofs.length + ' aprofundamentos em ' + URL + ' (' + amb.modelo + ')\n');
  const itens = [];
  const ini = Date.now();
  for(const c of ctrl){
    const xs = c.id === 'CT05' ? [Object.assign(await casoResumo(c), { controle:true })] : await casoAprofundamento(c);
    xs.forEach(x => { itens.push(x); console.log(linha(x)); });
  }
  for(const c of resumos){ const x = await casoResumo(c); itens.push(x); console.log(linha(x)); }
  for(const c of aprofs){ const xs = await casoAprofundamento(c); xs.forEach(x => { itens.push(x); console.log(linha(x)); }); }

  const ct05 = itens.find(i => i.id === 'CT05'), ct06 = itens.find(i => i.id === 'CT06');
  const metricas = {
    escolha:M.escolha(itens.filter(i => i.etapa === 'escolha')),
    resumo:M.resumo(itens.filter(i => i.etapa === 'resumo')),
    resumoComAprofundamento:M.resumo(itens.filter(i => i.etapa === 'resumo_aprof')),
    resumoTodos:M.resumo(itens.filter(i => i.etapa !== 'escolha')),
    controles:{
      CT05_numeroInventadoPego:!!(ct05 && ct05.falha === 'numero'),
      CT06_escolhaErradaContada:!!(ct06 && ct06.escolha && M.escolha([Object.assign({}, ct06, { controle:false })]).acertos === 0)
    },
    duracaoMin:Math.round((Date.now() - ini) / 6000) / 10
  };
  const e = metricas.escolha, r = metricas.resumoTodos;
  console.log('\nEscolha do aprofundamento: ' + e.acertos + '/' + e.casos + ' (' + e.acertoPct + '%) · estrito ' + e.estritos + ' · no prazo ' + e.acertosNoPrazo
    + ' · latência média ' + s1(e.latencia.media) + ', p90 ' + s1(e.latencia.p90) + ', acima de 15 s: ' + e.latencia.acimaDoPrazo);
  console.log('Resumo (60): 1ª tentativa sem número inventado ' + r.primeiraSemInventado + '/' + r.primeiraLegiveis + ' · aprovados ' + r.aprovados + '/' + r.casos
    + ' · no prazo ' + r.aprovadosNoPrazo + ' · latência média ' + s1(r.latencia.media) + ', p90 ' + s1(r.latencia.p90) + ', acima de 25 s: ' + r.latencia.acimaDoPrazo);
  console.log('Frases com maior/menor trocados: ' + r.comFraseTrocada.length + ' · controles: CT05 ' + metricas.controles.CT05_numeroInventadoPego + ', CT06 ' + metricas.controles.CT06_escolhaErradaContada
    + ' · ' + metricas.duracaoMin + ' min');
  if(process.env.CD2_EVAL_SAIDA) fs.writeFileSync(process.env.CD2_EVAL_SAIDA, JSON.stringify({ ambiente:amb, metricas, itens }, null, 1));
}

if(require.main === module) main().catch(e => { console.error(e); process.exitCode = 2; });
module.exports = { controles };
