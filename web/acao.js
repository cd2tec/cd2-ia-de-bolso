/* Ação com confirmação humana (Fase 4 do mini agente, CIDB-21). O resumo do alerta (História 3) traz uma
   ação sugerida, presa a um enum; ela só roda depois do clique em "Aprovar" no card do histórico, e cada
   passo fica na auditoria do cd2-ctl (proposta, aprovada ou recusada, executada ou falhou, com data e hora).

   Este arquivo tem os tipos de ação, a leitura do que o modelo sugeriu, o conteúdo de cada ação (texto da
   cópia, rascunho de e-mail, arquivo CSV) e o estado da proposta a partir da auditoria. Quem executa é a
   página (cópia, download e mailto só funcionam a partir de um clique); quem grava a auditoria é o cd2-ctl.
   Sem DOM: roda no Node, no goja e na página. */
(function(root){
"use strict";
const isNode = typeof module !== 'undefined' && module.exports;

// As ações iniciais do brief (Fase 4). "nenhuma": o modelo não sugere nada e o card fica como na História 3.
const TIPOS = ['copiar_resumo', 'rascunho_email', 'gerar_csv'];
const NENHUMA = 'nenhuma';
const ENUM = TIPOS.concat([NENHUMA]);
const ROTULOS = {
  copiar_resumo:'Copiar o resumo',
  rascunho_email:'Abrir um rascunho de e-mail com o resumo',
  gerar_csv:'Gerar o arquivo CSV com o resultado'
};
// Eventos da auditoria, na ordem em que podem acontecer.
const EVENTOS = ['proposta', 'aprovada', 'recusada', 'executada', 'falhou'];
const MAX_CORPO = 1800; // caracteres do corpo do e-mail: o mailto vira URL, e URL longa demais não abre

const objeto = v => v !== null && typeof v === 'object' && !Array.isArray(v);

// O que o modelo pôs em "acao": { acao } com um tipo de TIPOS, '' para "nenhuma" ou sem o campo, e
// { acao:'', fora } quando veio algo fora do enum (a proposta é descartada; o resumo continua valendo).
function lerAcao(v){
  if(v == null || v === NENHUMA) return { acao:'' };
  if(typeof v === 'string' && TIPOS.includes(v)) return { acao:v };
  return { acao:'', fora:String(typeof v === 'string' ? v : JSON.stringify(v)).slice(0, 60) };
}

const rotulo = t => ROTULOS[t] || '';

// Linha do aviso (notificação) com a ação sugerida. O balão do Windows não tem botões: a decisão é no card.
function textoAviso(t){
  return ROTULOS[t] ? 'Ação sugerida: ' + ROTULOS[t] + '. Abra o histórico para aprovar ou recusar.' : '';
}

// ---- estado a partir da auditoria ----
// execucao: a do histórico, com resumo.acao e auditoria [{ evento, acao, em, por, detalhe }].
// null quando não houve proposta. fase: pendente | aprovada (sem registro do resultado) | recusada |
// executada | falhou.
function estado(execucao){
  const e = objeto(execucao) ? execucao : {}, t = objeto(e.resumo) ? e.resumo.acao : '';
  if(!ROTULOS[t]) return null;
  const evs = (Array.isArray(e.auditoria) ? e.auditoria : []).filter(x => objeto(x) && EVENTOS.includes(x.evento));
  const achar = (...nomes) => evs.find(x => nomes.includes(x.evento)) || null;
  const proposta = achar('proposta'), decisao = achar('aprovada', 'recusada'), resultado = achar('executada', 'falhou');
  let fase = 'pendente';
  if(decisao) fase = decisao.evento === 'recusada' ? 'recusada' : resultado ? resultado.evento : 'aprovada';
  return { tipo:t, rotulo:ROTULOS[t], fase, proposta, decisao, resultado, eventos:evs };
}

const dd = n => (n < 10 ? '0' : '') + n;
function quando(iso){
  const d = new Date(iso);
  if(isNaN(d)) return '';
  return dd(d.getDate()) + '/' + dd(d.getMonth() + 1) + '/' + d.getFullYear() + ' às ' + dd(d.getHours()) + ':' + dd(d.getMinutes()) + ':' + dd(d.getSeconds());
}
const QUEM = { proposta:'Sugerida pela IA', aprovada:'Aprovada', recusada:'Recusada', executada:'Executada', falhou:'Falhou' };
// Uma linha da auditoria para o card: "Aprovada por maria em 24/11/2026 às 10:32:05".
function textoEvento(ev){
  if(!objeto(ev) || !QUEM[ev.evento]) return '';
  const por = ev.evento !== 'proposta' && ev.por ? ' por ' + ev.por : '';
  const q = quando(ev.em);
  return QUEM[ev.evento] + por + (q ? ' em ' + q : '') + (ev.detalhe ? ': ' + ev.detalhe : '') + '.';
}

// ---- o que cada ação faz ----
const linhasResumo = e => (objeto(e) && objeto(e.resumo) && Array.isArray(e.resumo.linhas) ? e.resumo.linhas : []).map(String);

// Texto copiado: as frases do resumo, uma por linha.
function textoCopia(execucao){ return linhasResumo(execucao).join('\n'); }

// Rascunho de e-mail: mailto sem destinatário (quem escolhe e envia é a pessoa), com o resumo no corpo.
// { ok, url } ou { ok:false, erro }.
function rascunhoEmail(nome, execucao){
  const linhas = linhasResumo(execucao);
  if(!linhas.length) return { ok:false, erro:'a execução não tem resumo' };
  const e = execucao, q = quando(e.em);
  let corpo = linhas.join('\n') + '\n\n' + 'Consulta: ' + String(nome || '') + (q ? '\nRodou em ' + q : '') +
    '\nResumo da IA de Bolso: os números foram conferidos com o resultado.';
  if(corpo.length > MAX_CORPO) corpo = corpo.slice(0, MAX_CORPO - 1) + '…';
  const assunto = 'IA de Bolso · ' + String(nome || 'alerta');
  return { ok:true, url:'mailto:?subject=' + encodeURIComponent(assunto) + '&body=' + encodeURIComponent(corpo) };
}

// Arquivo CSV com o resultado da execução (o mesmo CSV do botão do card). { ok, nome, conteudo } ou
// { ok:false, erro }. agora: o horário do nome do arquivo.
function arquivoCsv(execucao, agora){
  const r = objeto(execucao) && objeto(execucao.resultado) ? execucao.resultado : null;
  const todas = r && r.ok ? (Array.isArray(r.todas) ? r.todas : Array.isArray(r.linhas) ? r.linhas : []) : [];
  if(!r || !r.ok) return { ok:false, erro:'a execução não tem resultado' };
  if(!todas.length) return { ok:false, erro:'o resultado não tem linhas' };
  const colunas = Array.isArray(r.colunas) && r.colunas.length ? r.colunas : Object.keys(todas[0]);
  const Rep = isNode ? require('./report.js') : root.CD2Report; // só na página e nos testes (o goja não gera arquivo)
  return { ok:true, nome:Rep.fileName(agora || new Date()), conteudo:Rep.toCsv(colunas, todas) };
}

const api = { TIPOS, NENHUMA, ENUM, ROTULOS, EVENTOS, MAX_CORPO, lerAcao, rotulo, textoAviso, estado, textoEvento, textoCopia, rascunhoEmail, arquivoCsv };
if(isNode) module.exports = api; else root.CD2Acao = api;
})(typeof window !== 'undefined' ? window : globalThis);
