/* Resumo do alerta pelo modelo (Fase 2 do mini agente). Quando a condição do agendamento dispara,
   o cd2-ctl pede ao modelo até 3 frases sobre o resultado e a base de comparação. Este arquivo monta
   o pedido e confere a resposta; quem chama o modelo é o Go (o goja não faz rede).

   O número nunca vem do modelo: todo número e toda data do resumo precisam existir nos dados que
   foram enviados (linhas, valor conferido, base, limite, variação). Um só que não exista descarta o
   resumo inteiro, e o aviso sai como antes (o texto da condição). Sem DOM: roda no Node e no goja. */
(function(root){
"use strict";
const isNode = typeof module !== 'undefined' && module.exports;
const Chart = isNode ? require('./chart.js') : root.CD2Chart;

const MAX_FRASES = 3;
const MAX_FRASE = 160;     // caracteres por frase
const MAX_LINHAS = 20;     // linhas do resultado enviadas ao modelo
const MAX_COLUNAS = 8;
const MAX_CELULA = 60;
const MAX_TOKENS = 300;

const SCHEMA = {
  type:'object',
  properties:{ linhas:{ type:'array', minItems:1, maxItems:MAX_FRASES, items:{ type:'string', minLength:1, maxLength:MAX_FRASE } } },
  required:['linhas']
};

const objeto = v => v !== null && typeof v === 'object' && !Array.isArray(v);
const limpo = (v, n) => String(v == null ? '' : v).replace(/[\u0000-\u001f\u007f\s]+/g, ' ').trim().slice(0, n);

// ---- datas ----
const RE_ISO = /^(\d{4})-(\d{2})-(\d{2})(?:[T ][\d:.]+(?:Z|[+-]\d{2}:?\d{2})?)?$/;
// Data de uma célula ('2026-10-31', '2026-10-31T00:00:00Z') como { d, m, a }, ou null.
function dataDe(v){
  if(typeof v !== 'string') return null;
  const x = RE_ISO.exec(v.trim());
  return x ? { d:+x[3], m:+x[2], a:+x[1] } : null;
}
const dd = n => (n < 10 ? '0' : '') + n;
const dataBr = o => dd(o.d) + '/' + dd(o.m) + '/' + o.a;

// Célula como o modelo vê: data em dd/mm/aaaa, número no formato brasileiro, texto cortado.
function celula(v){
  if(v == null || v === '') return '';
  const d = dataDe(v);
  if(d) return dataBr(d);
  if(typeof v === 'number' || (typeof v === 'string' && Chart.num(v) != null && /\./.test(v))) return Chart.fmt(v);
  return limpo(v, MAX_CELULA);
}

// ---- o que vai para o modelo ----
// Os dados do resumo: o recorte do resultado que o modelo vê e a conferência usa.
function dados(consulta, resultado, julgamento){
  const r = resultado || {}, j = julgamento || {};
  const todas = (Array.isArray(r.todas) ? r.todas : Array.isArray(r.linhas) ? r.linhas : []).filter(objeto);
  const colunas = (Array.isArray(r.colunas) && r.colunas.length ? r.colunas : todas.length ? Object.keys(todas[0]) : [])
    .filter(c => typeof c === 'string' && c).slice(0, MAX_COLUNAS);
  const condicao = (consulta && consulta.agenda && consulta.agenda.condicao) || null;
  let linhas = todas.slice(0, MAX_LINHAS), comDisparo = false;
  const k = linhaDoDisparo(condicao, todas, j);
  if(k >= MAX_LINHAS){ linhas = todas.slice(0, MAX_LINHAS - 1).concat([todas[k]]); comDisparo = true; }
  return { alerta:limpo(j.texto, 400), colunas, linhas, total:todas.length, comDisparo, julgamento:j, condicao };
}

// Nas regras "o maior valor de" e "o menor valor de", uma linha do resultado é a que disparou o alerta. O pedido
// leva só as MAX_LINHAS primeiras; se ela estiver depois, entra no lugar da última, para o resumo sair do
// resultado de verdade (sem ela, o modelo apontava o maior ou o menor entre as linhas que recebeu). Devolve o
// índice da primeira linha com o valor conferido, ou -1.
function linhaDoDisparo(c, todas, j){
  if(!c || (c.agregacao !== 'maximo' && c.agregacao !== 'minimo') || !j.disparou) return -1;
  if(typeof j.valor !== 'number' || !isFinite(j.valor)) return -1;
  const nome = String(c.coluna || '').trim().toLowerCase();
  for(let i = 0; i < todas.length; i++){
    const col = Object.keys(todas[i]).find(x => x.toLowerCase() === nome);
    if(col != null && Chart.num(todas[i][col]) === j.valor) return i;
  }
  return -1;
}

const SISTEMA = [
  'Você escreve o resumo de um alerta de uma consulta ao banco de dados de uma empresa.',
  'Responda só com um objeto JSON {"linhas": ["...", "..."]}: de 1 a 3 frases curtas em português, cada uma com até 140 caracteres.',
  'Regras:',
  '- Use só números e datas do Resultado e da Base de comparação, escritos exatamente como estão. Não calcule nada: nada de somas, diferenças, médias, percentuais ou contagens novas.',
  '- A primeira frase diz o que disparou o alerta.',
  '- As outras destacam o que chama atenção no resultado (o maior, o menor, o que mais mudou).',
  '- Não dê conselhos, não explique causas e não invente.'
].join('\n');

// Base de comparação da regra, como a CIDB-11 pede ("a partir do resultado atual e da base de comparação"):
// o valor fixo da regra (maior/menor que) ou o valor da execução anterior ou da média das N anteriores.
// { texto, valor } ou null quando não há base (sem condição, ou variação ainda sem anterior).
function baseDe(d){
  const c = d.condicao, j = d.julgamento;
  if(!objeto(c)) return null;
  if(c.operador === 'maior' || c.operador === 'menor'){
    return typeof c.valor === 'number' && isFinite(c.valor) ? { texto:'valor fixo da regra = ' + Chart.fmt(c.valor), valor:c.valor } : null;
  }
  if(typeof j.base !== 'number' || !isFinite(j.base)) return null;
  const de = c.base === 'media' && j.n > 1 ? 'média das ' + j.n + ' execuções anteriores' : 'execução anterior';
  return { texto:de + ' = ' + Chart.fmt(j.base), valor:j.base };
}

// "as 20 primeiras de 30 linhas; " ou "20 de 30 linhas, com a linha que disparou o alerta (a última); ".
function recorte(d){
  if(d.total <= d.linhas.length) return '';
  if(d.comDisparo) return d.linhas.length + ' de ' + d.total + ' linhas, com a linha que disparou o alerta (a última); ';
  return 'as ' + d.linhas.length + ' primeiras de ' + d.total + ' linhas; ';
}

function mensagens(consulta, resultado, julgamento){
  const d = dados(consulta, resultado, julgamento), b = baseDe(d);
  const linhas = d.linhas.map(l => d.colunas.map(c => c + ': ' + celula(l[c])).join('; '));
  // Sem o nome da consulta: o resumo sai "a partir do resultado atual e da base de comparação" (CIDB-11). O nome
  // traz números que não são do resultado ("últimos 7 dias"); o modelo os copiava e a conferência recusava.
  const user = [
    'Alerta: ' + d.alerta,
    'Base de comparação: ' + (b ? b.texto : 'nenhuma'),
    'Resultado (' + recorte(d) + 'colunas: ' + d.colunas.join(', ') + '):',
    linhas.join('\n')
  ].join('\n');
  return [{ role:'system', content:SISTEMA }, { role:'user', content:user }];
}

// Pedido ao modelo, só quando a condição disparou e vai avisar. Fora disso, null: não há resumo.
function pedido(consulta, resultado, julgamento){
  const j = julgamento;
  if(!objeto(j) || !j.disparou || !j.aviso || !resultado || !resultado.ok) return null;
  return { mensagens:mensagens(consulta, resultado, j), schema:SCHEMA, maxTokens:MAX_TOKENS };
}

// ---- conferência ----
// Números de um texto livre (nomes de produto, título, frase do alerta): "350ml" -> 350.
function numerosDoTexto(t){
  const out = [];
  String(t == null ? '' : t).replace(RE_NUMERO, m => { const v = lerNumero(m); if(v != null) out.push(v); return m; });
  return out;
}
// O que o resumo pode citar (CIDB-11: "todo número presente no resumo existe no resultado ou na base de
// comparação"): os valores e as datas das células enviadas (números escritos em texto, como "350ml" ou
// "Loja 04", também são do resultado) e o valor da base. Mais dois números que o código tira do resultado e
// da base e que já estão no aviso de hoje: o valor conferido (a soma, o maior...) e a variação %. (Leitura
// nossa do critério, a confirmar com o produto.) Título, nomes de coluna e contagens não entram.
function permitidos(consulta, resultado, julgamento){
  const d = dados(consulta, resultado, julgamento), nums = [], datas = [];
  const add = v => { if(typeof v === 'number' && isFinite(v)) nums.push(Math.abs(v)); };
  for(const l of d.linhas){
    for(const c of d.colunas){
      const v = l[c], dt = dataDe(v);
      if(dt){ datas.push(dt); continue; }
      const n = Chart.num(v);
      if(n != null) add(n); else numerosDoTexto(v).forEach(add);
    }
  }
  const b = baseDe(d), j = d.julgamento;
  if(b) add(b.valor);
  [j.valor, j.variacao].forEach(v => add(typeof v === 'number' ? v : null));
  return { nums, datas };
}

// "1.234,56", "135.979", "12,5", "40" e o "-" na frente; o resto (%, mil, milhões) vem depois.
const RE_NUMERO = /\d{1,3}(?:\.\d{3})+(?:,\d+)?|\d+(?:,\d+)?/g;
function lerNumero(s){
  const t = String(s).replace(/\./g, '').replace(',', '.');
  const v = Number(t);
  return isFinite(v) ? v : null;
}
// Casas decimais escritas: "2,87" -> 2. A tolerância é meia unidade da última casa.
const casas = s => { const i = s.indexOf(','); return i < 0 ? 0 : s.length - i - 1; };
const MULT = [[/^\s*(?:milh(?:ão|ões|oes|ao))\b/i, 1e6], [/^\s*(?:bilh(?:ão|ões|oes|ao))\b/i, 1e9], [/^\s*mil\b/i, 1e3]];

const RE_DATA_BR = /\b(\d{1,2})\/(\d{1,2})(?:\/(\d{4}|\d{2}))?\b/g;
const RE_DATA_ISO = /\b(\d{4})-(\d{2})-(\d{2})\b/g;
const RE_HORA = /\b\d{1,2}:\d{2}\b/;

// Confere a resposta do modelo. { ok:true, linhas } ou { ok:false, motivo, detalhe }:
//   formato: não é o JSON pedido (ou frases vazias, longas ou demais);
//   numero: algum número ou data do resumo não existe nos dados.
function conferir(texto, consulta, resultado, julgamento){
  let s = String(texto == null ? '' : texto);
  const k = s.lastIndexOf('</think>'); if(k >= 0) s = s.slice(k + 8);
  let o = null;
  try{ o = JSON.parse(s.trim()); }catch(e){
    const a = s.indexOf('{'), b = s.lastIndexOf('}');
    if(a >= 0 && b > a){ try{ o = JSON.parse(s.slice(a, b + 1)); }catch(e2){ o = null; } }
  }
  if(!objeto(o) || !Array.isArray(o.linhas)) return { ok:false, motivo:'formato', detalhe:'não veio {"linhas": [...]}' };
  const linhas = o.linhas.map(l => typeof l === 'string' ? limpo(l, 1000) : '').filter(Boolean);
  if(!linhas.length || linhas.length > MAX_FRASES || linhas.length !== o.linhas.length) return { ok:false, motivo:'formato', detalhe:'de 1 a ' + MAX_FRASES + ' frases' };
  if(linhas.some(l => l.length > MAX_FRASE)) return { ok:false, motivo:'formato', detalhe:'frase com mais de ' + MAX_FRASE + ' caracteres' };

  const p = permitidos(consulta, resultado, julgamento);
  for(const l of linhas){
    const r = conferirFrase(l, p);
    if(!r.ok) return { ok:false, motivo:'numero', detalhe:r.detalhe };
  }
  return { ok:true, linhas };
}

function conferirFrase(frase, p){
  if(RE_HORA.test(frase)) return { ok:false, detalhe:'horário que não está nos dados' };
  let resto = frase, falha = null;
  // Datas antes dos números: "31/10/2026" não pode virar 31, 10 e 2026.
  resto = resto.replace(RE_DATA_ISO, (m, a, mm, d) => { if(!temData(p, +d, +mm, +a)) falha = falha || m; return ' '; });
  resto = resto.replace(RE_DATA_BR, (m, d, mm, a) => {
    const ano = a == null ? null : (a.length === 2 ? 2000 + +a : +a);
    if(!temData(p, +d, +mm, ano)) falha = falha || m;
    return ' ';
  });
  if(falha) return { ok:false, detalhe:'data ' + falha + ' não está nos dados' };
  RE_NUMERO.lastIndex = 0;
  let m;
  while((m = RE_NUMERO.exec(resto))){
    const bruto = m[0];
    let v = lerNumero(bruto), tol = 0.5 * Math.pow(10, -casas(bruto));
    const depois = resto.slice(m.index + bruto.length);
    for(const [re, f] of MULT){ if(re.test(depois)){ v *= f; tol *= f; break; } }
    if(v == null || !temNumero(p, v, tol)) return { ok:false, detalhe:'número ' + bruto + ' não está nos dados' };
  }
  return { ok:true };
}

function temData(p, d, m, a){
  return p.datas.some(x => x.d === d && x.m === m && (a == null || x.a === a));
}
function temNumero(p, v, tol){
  const t = Math.max(tol, 1e-9);
  return p.nums.some(x => Math.abs(x - v) <= t + Math.abs(x) * 1e-12);
}

// Por que uma execução que disparou saiu sem resumo, para o card do histórico. '' quando teve resumo.
const FALHAS = {
  prazo:'o modelo demorou mais que o prazo',
  modelo:'o modelo não respondeu',
  formato:'a resposta do modelo veio fora do formato',
  numero:'o resumo citava um número que não estava nos dados'
};
function textoFalha(r){
  if(!objeto(r) || (Array.isArray(r.linhas) && r.linhas.length) || !r.falha) return '';
  const vezes = r.tentativas > 1 ? ' (' + r.tentativas + ' tentativas)' : '';
  return 'Sem resumo da IA: ' + (FALHAS[r.falha] || 'falhou') + vezes + '.';
}

// Tentativas por resumo: a segunda só depois de a conferência recusar a primeira (formato ou número que não
// está nos dados), com o motivo no pedido. O mesmo limite do chat (web/ask.js, 2 tentativas por pergunta).
const MAX_TENTATIVAS = 2;
// Mensagens da tentativa seguinte: as anteriores, a resposta recusada e o motivo.
function novaTentativa(mensagens, resposta, conf){
  const motivo = (conf && conf.detalhe) || (conf && conf.motivo === 'formato' ? 'a resposta não era o JSON pedido' : 'a resposta foi recusada');
  return (Array.isArray(mensagens) ? mensagens : []).concat([
    { role:'assistant', content:String(resposta == null ? '' : resposta).slice(0, 2000) },
    { role:'user', content:'A resposta anterior foi recusada: ' + motivo + '. Responda de novo só com o objeto JSON {"linhas": [...]}, de 1 a 3 frases, usando só números e datas do Resultado e da Base de comparação, escritos exatamente como estão.' }
  ]);
}

// Texto do aviso com o resumo: as frases, uma por linha.
function textoAviso(linhas){ return (Array.isArray(linhas) ? linhas : []).join('\n'); }

const api = { SCHEMA, MAX_TENTATIVAS, MAX_FRASES, MAX_FRASE, MAX_LINHAS, MAX_TOKENS, dados, baseDe, mensagens, pedido, permitidos, conferir, novaTentativa, textoAviso, textoFalha, celula };
if(isNode) module.exports = api; else root.CD2Resumo = api;
})(typeof window !== 'undefined' ? window : globalThis);
