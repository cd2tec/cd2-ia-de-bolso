/* Aprofundamento condicional (Fase 3 do mini agente, CIDB-16). No agendamento, a pessoa vincula até 3
   consultas salvas como aprofundamentos (agenda.aprofundamentos: [ids]). Quando a condição dispara, o
   modelo escolhe uma delas, com a saída presa a um enum dos nomes das vinculadas, e o cd2-ctl roda a
   escolhida com o SQL salvo, como qualquer consulta agendada. O resumo (resumo.js) passa a incluir o
   resultado dela.

   Este arquivo valida o vínculo, monta o pedido da escolha e confere a resposta; quem chama o modelo e
   roda a consulta é o Go. Sem DOM: roda no Node, no goja e na página. */
(function(root){
"use strict";
const isNode = typeof module !== 'undefined' && module.exports;
const Res = isNode ? require('./resumo.js') : root.CD2Resumo;

const MAX = 3;              // consultas vinculadas por agendamento
const MAX_TENTATIVAS = 2;   // o mesmo limite do resumo (resumo.js) e do chat (ask.js)
const MAX_TOKENS = 60;
const MAX_NOME = 120;

const ERRO_LIMITE = 'No máximo ' + MAX + ' consultas de aprofundamento.';
const ERRO_SEM_CONDICAO = 'Aprofundamento só com "Só quando…": ele roda quando a regra dispara.';
const ERRO_PROPRIA = 'A própria consulta não pode ser aprofundamento dela.';
const ERRO_NAO_EXISTE = 'Uma das consultas de aprofundamento não existe mais. Desmarque-a.';
const ERRO_FORMA = 'Aprofundamentos inválidos.';

const objeto = v => v !== null && typeof v === 'object' && !Array.isArray(v);
const limpo = (v, n) => String(v == null ? '' : v).replace(/[\u0000-\u001f\u007f\s]+/g, ' ').trim().slice(0, n);

// Os ids vinculados na agenda da consulta, sem repetição, até MAX (o limite duro também vale no agendador).
function ids(consulta){
  const a = consulta && consulta.agenda, l = a && Array.isArray(a.aprofundamentos) ? a.aprofundamentos : [];
  const out = [];
  for(const x of l) if(typeof x === 'string' && x && out.indexOf(x) < 0) out.push(x);
  return out.slice(0, MAX);
}

// Confere o vínculo antes de salvar: até MAX, só com condição, consultas que existem e não a própria.
// lista: as consultas salvas ([{id, nome}]); idPropria: a consulta que está sendo agendada.
function validar(escolhidos, temCondicao, lista, idPropria){
  if(escolhidos == null) return { ok:true };
  if(!Array.isArray(escolhidos) || escolhidos.some(x => typeof x !== 'string' || !x)) return { ok:false, erro:ERRO_FORMA };
  if(!escolhidos.length) return { ok:true };
  if(new Set(escolhidos).size > MAX) return { ok:false, erro:ERRO_LIMITE };
  if(!temCondicao) return { ok:false, erro:ERRO_SEM_CONDICAO };
  if(escolhidos.indexOf(idPropria) >= 0) return { ok:false, erro:ERRO_PROPRIA };
  const existe = new Set((Array.isArray(lista) ? lista : []).filter(objeto).map(c => c.id));
  if(escolhidos.some(x => !existe.has(x))) return { ok:false, erro:ERRO_NAO_EXISTE };
  return { ok:true };
}

// As opções da escolha: as vinculadas que existem, na ordem da agenda, com o nome que o modelo vê. Nomes
// repetidos ganham " (2)", " (3)" para o enum não ter dois valores iguais.
function opcoes(consulta, lista){
  const porId = new Map((Array.isArray(lista) ? lista : []).filter(objeto).map(c => [c.id, c]));
  const out = [], usados = new Set();
  for(const id of ids(consulta)){
    const c = porId.get(id);
    if(!c || id === (consulta && consulta.id)) continue;
    const base = limpo(c.nome, MAX_NOME) || 'Consulta ' + (out.length + 1);
    let nome = base, k = 2;
    while(usados.has(nome)) nome = base + ' (' + (k++) + ')';
    usados.add(nome);
    out.push({ id, nome });
  }
  return out;
}

const SISTEMA = [
  'Você ajuda a investigar um alerta de uma consulta ao banco de dados de uma empresa.',
  'Escolha, da lista de consultas de aprofundamento, a que mais ajuda a explicar o alerta.',
  'Responda só com um objeto JSON {"escolha": "<nome>"}, com um dos nomes da lista escrito exatamente como está.'
].join('\n');

function schema(ops){
  return { type:'object', properties:{ escolha:{ type:'string', enum:ops.map(o => o.nome) } }, required:['escolha'] };
}

// Pedido da escolha, só quando a condição disparou e vai avisar, e há opções. Fora disso, null: o fluxo é
// o da História 3. O modelo vê a frase do alerta, a base de comparação e os nomes das opções (não o
// resultado inteiro, para a escolha sair rápida).
function pedido(consulta, julgamento, ops){
  const j = julgamento;
  if(!objeto(j) || !j.disparou || !j.aviso || !Array.isArray(ops) || !ops.length) return null;
  const b = Res.baseDe(Res.dados(consulta, { todas:[] }, j));
  const user = [
    'Alerta: ' + limpo(j.texto, 400),
    'Base de comparação: ' + (b ? b.texto : 'nenhuma'),
    'Consultas de aprofundamento:',
    ops.map(o => '- ' + o.nome).join('\n')
  ].join('\n');
  return { mensagens:[{ role:'system', content:SISTEMA }, { role:'user', content:user }], schema:schema(ops), maxTokens:MAX_TOKENS };
}

// Confere a resposta: { ok:true, id, nome } ou { ok:false, motivo:'formato'|'fora', detalhe }.
function conferir(texto, ops){
  let s = String(texto == null ? '' : texto);
  const k = s.lastIndexOf('</think>'); if(k >= 0) s = s.slice(k + 8);
  let o = null;
  try{ o = JSON.parse(s.trim()); }catch(e){
    const a = s.indexOf('{'), b = s.lastIndexOf('}');
    if(a >= 0 && b > a){ try{ o = JSON.parse(s.slice(a, b + 1)); }catch(e2){ o = null; } }
  }
  if(!objeto(o) || typeof o.escolha !== 'string') return { ok:false, motivo:'formato', detalhe:'não veio {"escolha": "..."}' };
  const op = (Array.isArray(ops) ? ops : []).find(x => x.nome === o.escolha.trim());
  if(!op) return { ok:false, motivo:'fora', detalhe:'"' + limpo(o.escolha, 80) + '" não é uma das consultas da lista' };
  return { ok:true, id:op.id, nome:op.nome };
}

// Mensagens da tentativa seguinte: as anteriores, a resposta recusada e o motivo.
function novaTentativa(mensagens, resposta, conf){
  const motivo = (conf && conf.detalhe) || 'a resposta foi recusada';
  return (Array.isArray(mensagens) ? mensagens : []).concat([
    { role:'assistant', content:String(resposta == null ? '' : resposta).slice(0, 500) },
    { role:'user', content:'A resposta anterior foi recusada: ' + motivo + '. Responda de novo só com o objeto JSON {"escolha": "<nome>"}, com um dos nomes da lista escrito exatamente como está.' }
  ]);
}

// Card do histórico: o que foi aprofundado, ou por que não. '' sem aprofundamento.
const FALHAS = {
  prazo:'o modelo demorou mais que o prazo para escolher',
  modelo:'o modelo não respondeu',
  formato:'a escolha do modelo veio fora do formato',
  fora:'o modelo escolheu uma consulta fora da lista',
  consulta:'a consulta escolhida não rodou',
  sem_consulta:'as consultas de aprofundamento não existem mais'
};
function textoAprofundamento(a){
  if(!objeto(a)) return '';
  if(!a.falha) return a.nome ? 'Aprofundou em: ' + a.nome : '';
  const vezes = a.tentativas > 1 ? ' (' + a.tentativas + ' tentativas)' : '';
  const em = a.nome && a.falha === 'consulta' ? ' ("' + a.nome + '")' : '';
  return 'Sem aprofundamento: ' + (FALHAS[a.falha] || 'falhou') + em + vezes + '.';
}

const api = { MAX, MAX_TENTATIVAS, MAX_TOKENS, ERRO_LIMITE, ids, validar, opcoes, pedido, conferir, novaTentativa, textoAprofundamento };
if(isNode) module.exports = api; else root.CD2Aprofundar = api;
})(typeof window !== 'undefined' ? window : globalThis);
