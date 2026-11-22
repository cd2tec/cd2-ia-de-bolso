/* Condição do agendamento: decide, só com código, se uma execução agendada merece aviso.
   O modelo não entra aqui: o número vem da consulta salva, e a regra é limite ou variação.
   Sem DOM: o cd2-ctl carrega este mesmo arquivo no interpretador JS embutido (goja).

   Forma da condição (agenda.condicao, opcional; sem ela o agendamento avisa toda vez, como antes):
     { coluna:'total', agregacao:'primeira'|'soma'|'media'|'maximo'|'minimo',
       operador:'maior'|'menor'|'subiu'|'caiu'|'variou', valor:4,
       base:'anterior'|'media', n:7 }
   maior e menor comparam com o valor fixo. subiu, caiu e variou comparam, em %, com a execução
   agendada anterior ou com a média das n anteriores (base e n só valem para esses três). */
(function(root){
"use strict";
const isNode = typeof module !== 'undefined' && module.exports;
const Chart = isNode ? require('./chart.js') : root.CD2Chart;

const AGREGACOES = ['primeira', 'soma', 'media', 'maximo', 'minimo'];
const OPERADORES = ['maior', 'menor', 'subiu', 'caiu', 'variou'];
const BASES = ['anterior', 'media'];
const MAX_N = 30;     // maior n da média; também o máximo de execuções que o cd2-ctl lê para a base
const MAX_COLUNA = 128;

const relativo = op => op === 'subiu' || op === 'caiu' || op === 'variou';
const objeto = v => v !== null && typeof v === 'object' && !Array.isArray(v);
const inteiro = v => typeof v === 'number' && isFinite(v) && Math.floor(v) === v;

const ERRO_COLUNA = 'Informe a coluna do resultado que a condição confere.';
const ERRO_VALOR = 'Informe um número na condição.';
const ERRO_PCT = 'A variação é um percentual a partir de 0.';
const ERRO_N = 'A média usa de 2 a ' + MAX_N + ' execuções.';
const ERRO_FORMA = 'Condição inválida.';

function validar(c){
  if(c == null) return { ok:true };
  if(!objeto(c) || AGREGACOES.indexOf(c.agregacao) < 0 || OPERADORES.indexOf(c.operador) < 0) return { ok:false, erro:ERRO_FORMA };
  if(typeof c.coluna !== 'string' || !c.coluna.trim() || c.coluna.length > MAX_COLUNA) return { ok:false, erro:ERRO_COLUNA };
  if(typeof c.valor !== 'number' || !isFinite(c.valor)) return { ok:false, erro:ERRO_VALOR };
  if(relativo(c.operador)){
    if(c.valor < 0) return { ok:false, erro:ERRO_PCT };
    if(BASES.indexOf(c.base) < 0) return { ok:false, erro:ERRO_FORMA };
    if(c.base === 'media' && !(inteiro(c.n) && c.n >= 2 && c.n <= MAX_N)) return { ok:false, erro:ERRO_N };
  }
  return { ok:true };
}

// Coluna do resultado com o nome da condição (sem caixa), ou null.
function acharColuna(linhas, nome){
  const n = String(nome).trim().toLowerCase();
  for(const l of linhas){
    if(!objeto(l)) continue;
    for(const k of Object.keys(l)) if(k.toLowerCase() === n) return k;
  }
  return null;
}

// O número que a condição confere, tirado das linhas: { ok, valor } ou { ok:false, motivo }.
function metrica(c, linhas){
  const ls = Array.isArray(linhas) ? linhas.filter(objeto) : [];
  if(!ls.length) return { ok:false, motivo:'vazio' };
  const col = acharColuna(ls, c.coluna);
  if(col == null) return { ok:false, motivo:'sem_coluna' };
  if(c.agregacao === 'primeira'){
    const v = Chart.num(ls[0][col]);
    return v == null ? { ok:false, motivo:'sem_numero' } : { ok:true, valor:v };
  }
  const vs = ls.map(l => Chart.num(l[col])).filter(v => v != null);
  if(!vs.length) return { ok:false, motivo:'sem_numero' };
  if(c.agregacao === 'soma') return { ok:true, valor:vs.reduce((a, b) => a + b, 0) };
  if(c.agregacao === 'media') return { ok:true, valor:vs.reduce((a, b) => a + b, 0) / vs.length };
  if(c.agregacao === 'maximo') return { ok:true, valor:Math.max.apply(null, vs) };
  return { ok:true, valor:Math.min.apply(null, vs) };
}

// ---- textos ----
const fmt = v => Chart.fmt(v);
const fmtPct = v => Chart.fmt(Math.round(Math.abs(v) * 10) / 10) + '%';
function rotulo(c){
  const col = String(c.coluna).trim();
  return { primeira:col, soma:'a soma de ' + col, media:'a média de ' + col, maximo:'o maior valor de ' + col, minimo:'o menor valor de ' + col }[c.agregacao] || col;
}
function textoBase(c, n){
  if(c.base === 'anterior') return 'à execução anterior';
  return n === 1 ? 'à única execução anterior' : 'à média das ' + n + ' anteriores';
}
const OPS_TEXTO = { maior:'for maior que', menor:'for menor que', subiu:'subir mais de', caiu:'cair mais de', variou:'variar mais de' };

// "Avisa só quando a soma de ruptura for maior que 4." Para a janela Agendar.
function resumo(c){
  if(!c || !validar(c).ok) return '';
  const a = rotulo(c);
  if(!relativo(c.operador)) return 'Avisa só quando ' + a + ' ' + OPS_TEXTO[c.operador] + ' ' + fmt(c.valor) + '.';
  return 'Avisa só quando ' + a + ' ' + OPS_TEXTO[c.operador] + ' ' + fmtPct(c.valor) + ' em relação ' + textoBase(c, c.n) + '.';
}

const MOTIVOS = {
  vazio:'A consulta não trouxe linhas. A condição não foi conferida.',
  erro:'A consulta não rodou. A condição não foi conferida.',
  sem_coluna:'A coluna da condição não veio no resultado. Confira a condição do agendamento.',
  sem_numero:'A coluna da condição não trouxe número. Confira a condição do agendamento.'
};

// Julgamento de uma execução.
//   c: a condição (null: sem condição, devolve null);
//   resultado: o Resultado da execução ({ ok, motivo, todas });
//   anteriores: as linhas (todas) das execuções agendadas anteriores que deram certo, da mais nova
//   para a mais velha.
// aviso diz se a execução notifica e conta como novidade. Sem condição avaliável por erro do banco
// ou condição que não casa com o resultado, avisa (senão o alerta pararia calado); resultado vazio
// e falta de base não avisam.
function julgar(c, resultado, anteriores){
  if(c == null) return null;
  if(!validar(c).ok) return { avaliada:false, disparou:false, aviso:true, motivo:'condicao', texto:'A condição do agendamento é inválida.' };
  const r = resultado || {};
  if(!r.ok){
    const vazio = r.motivo === 'vazio';
    return { avaliada:false, disparou:false, aviso:!vazio, motivo:vazio ? 'vazio' : 'erro', texto:MOTIVOS[vazio ? 'vazio' : 'erro'] };
  }
  const m = metrica(c, r.todas || r.linhas);
  if(!m.ok){
    const aviso = m.motivo !== 'vazio';
    return { avaliada:false, disparou:false, aviso, motivo:m.motivo, texto:MOTIVOS[m.motivo] };
  }
  const v = m.valor, nome = rotulo(c), Nome = c.agregacao === 'primeira' ? nome : nome.charAt(0).toUpperCase() + nome.slice(1);
  if(!relativo(c.operador)){
    const disparou = c.operador === 'maior' ? v > c.valor : v < c.valor;
    const texto = Nome + ' = ' + fmt(v) + (c.operador === 'maior'
      ? (disparou ? ', acima de ' : ', não passou de ') : (disparou ? ', abaixo de ' : ', não ficou abaixo de ')) + fmt(c.valor) + '.';
    return { avaliada:true, disparou, aviso:disparou, valor:v, texto };
  }
  const quantos = c.base === 'anterior' ? 1 : c.n;
  const bases = [];
  for(const ls of (Array.isArray(anteriores) ? anteriores : [])){
    if(bases.length >= quantos) break;
    const b = metrica(c, ls);
    if(b.ok) bases.push(b.valor);
  }
  if(!bases.length){
    return { avaliada:false, disparou:false, aviso:false, motivo:'sem_base', valor:v,
      texto:Nome + ' = ' + fmt(v) + '. Ainda não há execução anterior para comparar.' };
  }
  const base = bases.reduce((a, b) => a + b, 0) / bases.length;
  // Base zero: qualquer valor diferente de zero é variação sem tamanho (infinita).
  const variacao = base === 0 ? (v === 0 ? 0 : (v > 0 ? Infinity : -Infinity)) : (v - base) / Math.abs(base) * 100;
  const disparou = c.operador === 'subiu' ? variacao > c.valor : c.operador === 'caiu' ? -variacao > c.valor : Math.abs(variacao) > c.valor;
  const contra = textoBase(c, bases.length);
  let mudou;
  if(!isFinite(variacao)) mudou = (variacao > 0 ? ' subiu' : ' caiu') + ' de 0 para ' + fmt(v);
  else if(variacao === 0) mudou = ' ficou igual (' + fmt(v) + ')';
  else mudou = (variacao > 0 ? ' subiu ' : ' caiu ') + fmtPct(variacao) + ' (' + fmt(base) + ' para ' + fmt(v) + ')';
  const texto = Nome + mudou + ' em relação ' + contra + (disparou ? '.' : ', dentro do limite de ' + fmtPct(c.valor) + '.');
  return { avaliada:true, disparou, aviso:disparou, valor:v, base, variacao:isFinite(variacao) ? variacao : null, n:bases.length, texto };
}

// Linha do card do histórico: "Avisou: …", "Sem aviso: …" ou só o texto (erro que avisou).
function textoJulgamento(j){
  if(!objeto(j) || typeof j.texto !== 'string' || !j.texto) return '';
  if(j.disparou) return 'Avisou: ' + j.texto;
  if(!j.aviso) return 'Sem aviso: ' + j.texto;
  return j.texto;
}

// ---- colunas para a regra ----
// Colunas de um resultado ({ colunas, todas|linhas }) e se cada uma é número: [{ nome, numero }].
// É o que a consulta salva guarda no 👍 (só nomes e tipos, nunca valores).
function colunasDe(r){
  if(!r || !Array.isArray(r.colunas)) return [];
  const ls = (Array.isArray(r.todas) ? r.todas : Array.isArray(r.linhas) ? r.linhas : []).filter(objeto);
  return r.colunas.filter(c => typeof c === 'string' && c).map(nome => {
    const vs = ls.map(l => l[nome]).filter(v => v != null && v !== '');
    return { nome, numero:vs.length > 0 && vs.every(v => Chart.num(v) != null) };
  });
}
// Nome legível de uma coluna: 'valor_total' -> 'Valor total'; 'Venda (R$)' fica como está.
function rotuloColuna(nome){
  const t = String(nome == null ? '' : nome).replace(/[_]+/g, ' ').replace(/\s+/g, ' ').trim();
  return t ? t.charAt(0).toUpperCase() + t.slice(1) : '';
}
// As colunas numéricas que a regra pode conferir, na ordem: o y da consulta primeiro.
//   c: a consulta salva (c.colunas do 👍; c.y); ultimo: o resultado da última execução, ou null;
//   atual: a coluna já escolhida na condição (fica na lista mesmo que tenha sumido, para não trocar calada).
function opcoesColuna(c, ultimo, atual){
  c = c || {};
  let cols = Array.isArray(c.colunas) && c.colunas.length ? c.colunas.filter(objeto) : colunasDe(ultimo);
  let nums = cols.filter(x => x.numero && typeof x.nome === 'string').map(x => x.nome);
  if(!cols.length && c.y) nums = [c.y];
  const lista = [];
  const pôr = n => { if(n && !lista.some(x => x.toLowerCase() === String(n).toLowerCase())) lista.push(String(n)); };
  if(c.y && nums.some(n => n.toLowerCase() === String(c.y).toLowerCase())) pôr(c.y);
  nums.forEach(pôr);
  if(atual) pôr(atual);
  return lista;
}
// Resultado de uma linha só: a agregação não muda nada e sai da tela.
function umaLinha(c, ultimo){
  if(c && typeof c.umaLinha === 'boolean') return c.umaLinha;
  const ls = ultimo && (Array.isArray(ultimo.todas) ? ultimo.todas : ultimo.linhas);
  return Array.isArray(ls) && ls.length === 1;
}
// "Último resultado: 12.702" para orientar o limite, ou '' se não dá para calcular.
function textoUltimo(cond, ultimo){
  if(!cond || !ultimo || !ultimo.ok || typeof cond.coluna !== 'string' || !cond.coluna.trim()) return '';
  const m = metrica({ coluna:cond.coluna, agregacao:AGREGACOES.indexOf(cond.agregacao) >= 0 ? cond.agregacao : 'primeira' }, ultimo.todas || ultimo.linhas);
  return m.ok ? 'Último resultado: ' + fmt(m.valor) + '.' : '';
}

// ---- a janela Agendar ----
// Estado do bloco "Quando avisar": textos do jeito que os campos dão. coluna: a coluna sugerida (o y salvo).
function tela(c, coluna){
  const ok = objeto(c) && validar(c).ok;
  return {
    ligado:ok,
    coluna:ok ? c.coluna : String(coluna || ''),
    agregacao:ok ? c.agregacao : 'primeira',
    operador:ok ? c.operador : 'maior',
    valor:ok ? String(c.valor).replace('.', ',') : '',
    base:ok && c.base ? c.base : 'anterior',
    n:ok && inteiro(c.n) ? String(c.n) : '7'
  };
}
// Número digitado com vírgula ou ponto decimal (sem separador de milhar), ou NaN.
function numeroDe(s){
  const t = String(s == null ? '' : s).trim().replace(',', '.');
  return /^-?\d+(\.\d+)?$/.test(t) ? Number(t) : NaN;
}
// Estado da tela -> { ok, condicao } (condicao null quando avisa sempre) ou { ok:false, erro }.
function daTela(t){
  if(!t || !t.ligado) return { ok:true, condicao:null };
  const c = { coluna:String(t.coluna || '').trim(), agregacao:t.agregacao, operador:t.operador, valor:numeroDe(t.valor) };
  if(relativo(c.operador)){
    c.base = t.base;
    if(c.base === 'media') c.n = /^\s*\d+\s*$/.test(String(t.n)) ? Number(t.n) : NaN;
  }
  const v = validar(c);
  return v.ok ? { ok:true, condicao:c } : v;
}

const api = { AGREGACOES, OPERADORES, BASES, MAX_N, relativo, validar, metrica, resumo, julgar, textoJulgamento, tela, daTela, numeroDe,
  colunasDe, rotuloColuna, opcoesColuna, umaLinha, textoUltimo };
if(isNode) module.exports = api; else root.CD2Condicao = api;
})(typeof window !== 'undefined' ? window : globalThis);
