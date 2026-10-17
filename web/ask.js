/* Roteiro de relatórios da IA de Bolso (modo Banco de dados): pergunta -> tabelas candidatas
   -> uma chamada ao modelo pedindo JSON -> consulta no toolbox -> visual e frase de molde.
   O modelo nunca vê o resultado: todo número na tela vem do banco. Sem DOM. */
(function(root){
"use strict";
const isNode = typeof module !== 'undefined' && module.exports;
const Chart = isNode ? require('./chart.js') : root.CD2Chart;
const Cat = isNode ? require('./catalog.js') : root.CD2Catalog;
const Tb = isNode ? require('./toolbox.js') : root.CD2Toolbox;

const VISUALS = ['barra', 'linha', 'pizza', 'tabela', 'numero'];
const MENSAGENS = {
  sem_tabelas:'Não achei tabelas sobre isso neste banco.',
  sql:'Não consegui montar essa consulta.',
  timeout:'A consulta demorou demais e foi cancelada para proteger o banco.',
  timeout_front:'A consulta passou de 15 segundos e o harness parou de esperar. Ela pode continuar rodando no servidor.',
  vazio:'A consulta não trouxe nenhuma linha.',
  modelo:'O modelo não respondeu.'
};
// Textos de timeout por motor (PostgreSQL, MariaDB/MySQL, driver Go, Oracle) e o do próprio front.
const TIMEOUT_RE = /statement timeout|max_statement_time|maximum statement execution time|context deadline exceeded|query timeout|invalid connection|ORA-01013|tempo esgotado/i;
const MEDIA_RE = /(^|[^a-z])(media|medio|avg|average|ticket|percent|percentual|pct|taxa|margem|rate|ratio|indice|preco|price|unitario|saldo|estoque)([^a-z]|$)/;
const semAcento = s => String(s).normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase();
const MESES = ['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez'];
const TEMPO_NOME_RE = /^(mes|month|ano|year|dia|day|data|date|periodo|semana|week|trimestre)/i;
const TEMPO_VALOR_RE = /^\d{4}-\d{2}(-\d{2})?([ T].*)?$|^\d{1,2}\/\d{4}$|^\d{1,2}\/\d{1,2}\/\d{4}$|^(jan|fev|mar|abr|mai|jun|jul|ago|set|out|nov|dez)[a-z]*[\/ -]?\d{2,4}$/i;

function parseRows(text){
  const s = String(text == null ? '' : text);
  const t = s.trim();
  if(t.startsWith('ERRO: ')) return { ok:false, error:t.slice(6) || 'erro sem mensagem' };
  if(!t || t === 'null') return { ok:true, rows:[] };
  const obj = v => v && typeof v === 'object' && !Array.isArray(v);
  const bad = { ok:false, error:'Resposta inesperada do banco: ' + t.slice(0, 200) };
  try{
    const j = JSON.parse(t);
    if(Array.isArray(j)) return { ok:true, rows:j.filter(obj) };
    return obj(j) ? { ok:true, rows:[j] } : bad;
  }catch(e){}
  const rows = [];
  for(const line of t.split(/\r?\n/)){
    if(!line.trim()) continue;
    try{ const j = JSON.parse(line); if(!obj(j)) return bad; rows.push(j); }catch(e){ return bad; }
  }
  return { ok:true, rows };
}
function parseModelJson(text){
  let s = String(text == null ? '' : text);
  const k = s.lastIndexOf('</think>'); if(k >= 0) s = s.slice(k + 8);
  let o = null;
  try{ o = JSON.parse(s.trim()); }catch(e){
    const a = s.indexOf('{'), b = s.lastIndexOf('}');
    if(a >= 0 && b > a){ try{ o = JSON.parse(s.slice(a, b + 1)); }catch(e2){ o = null; } }
  }
  if(!o || typeof o !== 'object' || typeof o.visual !== 'string' || typeof o.sql !== 'string' || !o.sql.trim() || !VISUALS.includes(semAcento(o.visual))) return null;
  const str = v => typeof v === 'string' ? v.trim() : '';
  return { sql:o.sql.trim(), visual:semAcento(o.visual), x:str(o.x), y:str(o.y), titulo:str(o.titulo) };
}
function isTimeout(msg){ return TIMEOUT_RE.test(String(msg == null ? '' : msg)); }

function findCol(colunas, name){ if(!name) return null; const n = name.toLowerCase(); return colunas.find(c => c.toLowerCase() === n) || null; }
function isNumCol(linhas, c){
  let any = false;
  for(const r of linhas){ const v = r[c]; if(v == null || v === '') continue; if(Chart.num(v) == null) return false; any = true; }
  return any;
}
function isTimeCol(linhas, c){
  const vs = linhas.map(r => r[c]).filter(v => v != null && v !== '');
  if(!vs.length) return false;
  if(vs.every(v => TEMPO_VALOR_RE.test(String(v)))) return true;
  return TEMPO_NOME_RE.test(c) && vs.every(v => Chart.num(v) != null);
}
// Chave ordenável de um rótulo de tempo (ISO, dd/mm/aaaa, mm/aaaa, 'jan/2026', número) ou null.
function timeKey(v){
  const t = String(v).trim();
  if(/^\d{4}-\d{2}/.test(t)) return t;
  let m = t.match(/^(\d{1,2})\/(\d{1,2})\/(\d{4})$/);
  if(m) return m[3] + '-' + m[2].padStart(2, '0') + '-' + m[1].padStart(2, '0');
  m = t.match(/^(\d{1,2})\/(\d{4})$/);
  if(m) return m[2] + '-' + m[1].padStart(2, '0');
  m = semAcento(t).match(/^([a-z]{3,})[\/ -](\d{2}|\d{4})$/);
  if(m){
    const i = MESES.indexOf(m[1].slice(0, 3));
    if(i >= 0) return (m[2].length === 2 ? '20' + m[2] : m[2]) + '-' + String(i + 1).padStart(2, '0');
  }
  return null;
}
function byTime(linhas, x){
  const out = linhas.slice();
  if(out.every(r => Chart.num(r[x]) != null)) return out.sort((a, b) => Chart.num(a[x]) - Chart.num(b[x]));
  const ks = out.map(r => timeKey(r[x]));
  if(ks.every(k => k != null)){
    const idx = out.map((r, i) => i).sort((i, j) => ks[i].localeCompare(ks[j]) || i - j);
    return idx.map(i => out[i]);
  }
  return out;
}
// Coluna com cara de chave: alguma palavra do nome começa com id, cod, cd, nr ou nro, ou termina em id.
function isKeyName(c){
  return semAcento(String(c).replace(/([a-z0-9])([A-Z])/g, '$1 $2')).split(/[^a-z0-9]+/).filter(Boolean)
    .some(t => /^(id|cod|cd|nr|nro)/.test(t) || t.endsWith('id'));
}
const isMeasure = c => !TEMPO_NOME_RE.test(c) && !isKeyName(c);
// Uma coluna numérica para y: só quando a escolha é única (medidas antes de tempo e chave).
function onlyNum(numCols, x){
  const cands = numCols.filter(c => c !== x), pref = cands.filter(isMeasure), pool = pref.length ? pref : cands;
  return pool.length === 1 ? pool[0] : null;
}
// O pedido do modelo é ajustado pelo dado que voltou (regras da seção 3, passo 5, da spec).
function pickVisual(req, colunas, linhas){
  const tabela = () => ({ visual:'tabela', x:null, y:null, linhas, aviso:null });
  const numCols = colunas.filter(c => isNumCol(linhas, c));
  const txtCols = colunas.filter(c => !numCols.includes(c) && linhas.some(r => r[c] != null && r[c] !== ''));
  let x = findCol(colunas, req.x), y = findCol(colunas, req.y);
  if(!x){ const t = txtCols.filter(c => c !== y); if(t.length === 1) x = t[0]; }
  if(!y) y = onlyNum(numCols, x);
  if(linhas.length === 1 && colunas.length === 1 && numCols.length === 1) return { visual:'numero', x:null, y:numCols[0], linhas, aviso:null };
  let v = req.visual;
  if(v === 'numero'){
    if(linhas.length !== 1 || !numCols.length) return tabela();
    const yn = (y && numCols.includes(y)) ? y : (numCols.find(isMeasure) || numCols[0]);
    // Um rótulo de texto junto: barra de uma barra, para a resposta mostrar de quem é o número.
    if(txtCols.length === 1) return { visual:'barra', x:txtCols[0], y:yn, linhas, aviso:null };
    return { visual:'numero', x:null, y:yn, linhas, aviso:null };
  }
  if(v === 'tabela' || !x || !y || !numCols.includes(y)) return tabela();
  if(v === 'pizza' && (linhas.length > 6 || linhas.some(r => (Chart.num(r[y]) || 0) < 0))) v = 'barra';
  const tempo = isTimeCol(linhas, x);
  if(v === 'linha' && !tempo) v = 'barra';
  if(v === 'barra' && tempo && linhas.length > 20) v = 'linha';
  let out = (v === 'linha' || (v === 'barra' && tempo)) ? byTime(linhas, x)
    : linhas.slice().sort((a, b) => (Chart.num(b[y]) || 0) - (Chart.num(a[y]) || 0));
  let aviso = null;
  if(v === 'barra' && out.length > 20){ aviso = 'Mostrando os 20 maiores de ' + out.length + '.'; out = byValue(out, y).slice(0, 20); }
  return { visual:v, x, y, linhas:out, aviso };
}
function byValue(linhas, y){ return linhas.slice().sort((a, b) => (Chart.num(b[y]) || 0) - (Chart.num(a[y]) || 0)); }
// Frase de molde: montada daqui, nunca pelo modelo.
function frase(r){
  const L = r.linhas || [], T = r.todas || L;
  if(r.visual === 'numero') return 'Resultado: ' + Chart.fmt(L[0] && L[0][r.y]) + '.';
  if(r.visual === 'tabela') return T.length === 1 ? '1 linha.' : T.length + ' linhas.';
  const vals = L.map(l => Chart.num(l[r.y]) || 0), labs = Chart.labelsOf(L.map(l => l[r.x]));
  let iMax = 0; vals.forEach((v, i) => { if(v > vals[iMax]) iMax = i; });
  if(r.visual === 'linha') return 'De ' + labs[0] + ' a ' + labs[labs.length - 1] + '. Maior valor em ' + labs[iMax] + ': ' + Chart.fmt(vals[iMax]) + '.';
  const n = T.length + (T.length === 1 ? ' categoria.' : ' categorias.');
  const maior = 'Maior: ' + labs[iMax] + ', ' + Chart.fmt(vals[iMax]) + '. ';
  if(MEDIA_RE.test(semAcento(r.y))) return maior + n;
  const tot = T.reduce((a, l) => a + (Chart.num(l[r.y]) || 0), 0);
  return maior + 'Total: ' + Chart.fmt(tot) + ' em ' + n;
}

const SCHEMA = { type:'object', properties:{ sql:{ type:'string' }, visual:{ type:'string', enum:VISUALS }, x:{ type:'string' }, y:{ type:'string' }, titulo:{ type:'string' } }, required:['sql', 'visual', 'x', 'y', 'titulo'] };
const DIALETO = {
  postgres:  { nome:'PostgreSQL', mes:c => "to_char(" + c + ", 'YYYY-MM')", top:(s, o) => s + ' ' + o + ' LIMIT 10' },
  mariadb:   { nome:'MariaDB/MySQL (crases em nomes reservados)', mes:c => "DATE_FORMAT(" + c + ", '%Y-%m')", top:(s, o) => s + ' ' + o + ' LIMIT 10' },
  sqlserver: { nome:'SQL Server (TOP em vez de LIMIT; sem WITH; use subconsulta)', mes:c => "FORMAT(" + c + ", 'yyyy-MM')", top:(s, o) => s.replace(/^SELECT /, 'SELECT TOP (10) ') + ' ' + o },
  oracle:    { nome:'Oracle (sem LIMIT; nomes sem aspas)', mes:c => "TO_CHAR(" + c + ", 'YYYY-MM')", top:(s, o) => s + ' ' + o + ' FETCH FIRST 10 ROWS ONLY' },
  firebird:  { nome:'Firebird (ROWS em vez de LIMIT; sem WITH; nomes como o banco devolve)', mes:c => "EXTRACT(YEAR FROM " + c + ") || '-' || LPAD(EXTRACT(MONTH FROM " + c + "), 2, '0')", top:(s, o) => s + ' ' + o + ' ROWS 10' }
};
function exemplos(motor){
  const d = DIALETO[motor] || DIALETO.postgres, m = d.mes('data_venda');
  return [
    'Pergunta: quantos pedidos existem? -> ' + JSON.stringify({ sql:'SELECT COUNT(*) AS pedidos FROM pedido', visual:'numero', x:'', y:'pedidos', titulo:'Total de pedidos' }),
    'Pergunta: total vendido por mês -> ' + JSON.stringify({ sql:'SELECT ' + m + ' AS mes, SUM(valor) AS total FROM venda GROUP BY ' + m + ' ORDER BY mes', visual:'linha', x:'mes', y:'total', titulo:'Vendas por mês' }),
    'Pergunta: 10 clientes que mais compraram -> ' + JSON.stringify({ sql:d.top('SELECT c.nome AS cliente, SUM(p.valor) AS total FROM pedido p JOIN cliente c ON c.id = p.cliente_id GROUP BY c.nome', 'ORDER BY total DESC'), visual:'barra', x:'cliente', y:'total', titulo:'Clientes que mais compraram' })
  ].join('\n');
}
// Prefixo de schema para os nomes de tabela, só quando o schema configurado não é o padrão da conta:
// PostgreSQL public, SQL Server dbo, Oracle o próprio usuário; MariaDB nunca (schema é a base).
function schemaPrefix(motor, schema, usuario){
  const s = String(schema == null ? '' : schema).trim();
  if(!s || motor === 'mariadb' || motor === 'firebird') return '';
  const padrao = { postgres:'public', sqlserver:'dbo', oracle:String(usuario == null ? '' : usuario).trim() }[motor] || 'public';
  return s.toLowerCase() === padrao.toLowerCase() ? '' : s + '.';
}
// Nome e tipo vindos do banco: sem quebra de linha nem controle, e no máximo 80 caracteres.
const limpo = v => String(v == null ? '' : v).replace(/[\u0000-\u001f\u007f\s]+/g, ' ').trim().slice(0, 80);
function buildMessages(o){
  const d = DIALETO[o.motor] || DIALETO.postgres, pre = limpo(schemaPrefix(o.motor, o.schema, o.usuario));
  // View não tem contagem de linhas: "cerca de 0 linhas" faria o modelo achar que está vazia.
  const tam = t => t.type === 'VIEW' ? (t.rows ? 'view, cerca de ' + t.rows + ' linhas' : 'view') : 'cerca de ' + t.rows + ' linhas';
  // Papel e colunas úteis do perfil das tabelas; indefinida não acrescenta nada.
  const papel = t => {
    const p = t.perfil;
    if(!p || !p.papel || p.papel === 'indefinida') return '';
    // Colunas úteis só fazem sentido em movimento e itens; num cadastro, uma data de nascimento induziria o modelo.
    if(p.papel !== 'movimento' && p.papel !== 'itens') return '; ' + p.papel;
    const c = p.colunas || {};
    return '; ' + p.papel + (c.data ? '; data: ' + limpo(c.data) : '') + (c.valor ? '; valor: ' + limpo(c.valor) : '') + (c.quantidade ? '; quantidade: ' + limpo(c.quantidade) : '');
  };
  // Descrição feita pelo modelo a partir da amostra: só texto, sem quebra de linha, até 160 caracteres.
  const desc = t => t.perfil && t.perfil.descricao
    ? 'descrição: ' + String(t.perfil.descricao).replace(/[\u0000-\u001f\u007f\s]+/g, ' ').trim().slice(0, 160) + ' | colunas: ' : '';
  const tabs = o.tabelas.map(t => '- ' + pre + limpo(t.name) + ' (' + tam(t) + papel(t) + '): ' + desc(t)
    + t.columns.map(c => limpo(c.name) + ' ' + limpo(c.type)).join(', ')).join('\n');
  const lig = Cat.links(o.tabelas);
  const sys = [
    'Você transforma uma pergunta em uma consulta SQL de leitura e diz como mostrar o resultado.',
    'Banco: ' + d.nome + (o.versao ? '. Versão: ' + o.versao : '') + '.',
    'Tabelas disponíveis (use só estas tabelas e colunas, com os nomes exatamente como estão' + (pre ? ', com o schema na frente' : '') + '):',
    tabs,
    lig.length ? 'Como as tabelas se ligam (use estes JOIN):\n' + lig.map(l => '- ' + pre + limpo(l.a) + '.' + limpo(l.col) + ' = ' + pre + limpo(l.b) + '.' + limpo(l.colB || l.col)).join('\n') : '',
    'Regras:',
    (o.motor === 'sqlserver' || o.motor === 'firebird') ? '- Um único SELECT, sem WITH e sem ponto e vírgula. Use subconsulta no lugar de WITH.' : '- Um único SELECT (ou WITH), sem ponto e vírgula.',
    '- Agregue no banco com GROUP BY e devolva no máximo 50 linhas.',
    '- Ao agrupar por uma entidade (loja, produto, departamento, cliente), mostre o nome ou a descrição com JOIN na tabela de cadastro, não o código.',
    "- Para filtrar por nome ou descrição, use comparação parcial sem diferenciar caixa (UPPER(coluna) LIKE '%TEXTO%').",
    '- Depois de JOIN com tabela de itens, não some colunas do cabeçalho (evita contar em dobro).',
    '- Dê a cada coluna do resultado um alias curto em português, sem espaço nem acento (ex.: total, mes, loja).',
    '- visual: numero (um valor só), barra (comparar categorias), linha (evolução no tempo), pizza (até 6 partes de um todo), tabela (lista).',
    '- x é o alias da coluna de categoria ou de tempo; y é o alias da coluna numérica. Em numero e tabela, x pode ficar vazio.',
    '- titulo: curto, em português, sem números.',
    'Responda só com um objeto JSON no formato {"sql": "...", "visual": "...", "x": "...", "y": "...", "titulo": "..."}.',
    'Exemplos:',
    exemplos(o.motor)
  ].filter(Boolean).join('\n');
  let user = 'Pergunta: ' + o.pergunta;
  if(o.anterior) user += '\n\nA tentativa anterior falhou.\nSQL anterior: ' + (o.anterior.sql || '(nenhum)') + '\nErro: ' + String(o.anterior.erro == null ? '' : o.anterior.erro).slice(0, 300) + '\nCorrija e responda de novo só com o JSON.';
  return [{ role:'system', content:sys }, { role:'user', content:user }];
}

// A consulta precisa ler ao menos uma tabela listada (nome inteiro, schema na frente vale). Sem isso,
// um SELECT 1500 AS total mostraria um número inventado pelo modelo. Literais '...' não contam.
function readsListedTable(sql, tabelas){
  const s = String(sql).replace(/'(?:[^']|'')*'/g, ' ');
  const esc = n => n.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  return tabelas.some(t => new RegExp('(^|[^A-Za-z0-9_$#])' + esc(t.name) + '([^A-Za-z0-9_$#]|$)', 'i').test(s));
}
// Depois de JOIN do cabeçalho com os itens, o valor do cabeçalho se repete em cada item: SUM ou AVG dele
// conta em dobro. O prompt já pede para não fazer isso, mas o modelo pequeno insiste; aqui a consulta
// é recusada antes de ir ao banco e o motivo volta para o modelo. Devolve o motivo, ou '' se está ok.
function somaDoCabecalho(sql, tabelas){
  const s = String(sql).replace(/'(?:[^']|'')*'/g, ' ');
  const esc = n => n.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const usa = n => new RegExp('(^|[^A-Za-z0-9_$#])' + esc(n) + '([^A-Za-z0-9_$#]|$)', 'i').test(s);
  const cols = n => (tabelas.find(t => t.name === n) || { columns:[] }).columns;
  const tem = (n, c) => cols(n).some(x => String(x.name).toLowerCase() === c.toLowerCase());
  for(const p of Cat.itemPairs(tabelas)){
    if(!usa(p.cab) || !usa(p.itens)) continue;
    for(const m of s.matchAll(/\b(?:SUM|AVG)\s*\(\s*(?:DISTINCT\s+)?(?:"?[\w$#]+"?\.)?"?([\w$#]+)"?\s*\)/gi)){
      const c = m[1];
      if(!tem(p.cab, c) || tem(p.itens, c)) continue;
      const valores = cols(p.itens).filter(x => x.kind === 'numero' && String(x.name).toLowerCase() !== p.col.toLowerCase()).map(x => x.name);
      return 'A consulta junta ' + p.cab + ' com os itens (' + p.itens + ') e soma ' + p.cab + '.' + c
        + ', que se repete em cada item e conta em dobro. Use uma coluna de valor de ' + p.itens
        + (valores.length ? ' (' + valores.join(', ') + ')' : '') + ' ou não junte os itens.';
    }
  }
  return '';
}
// Título do modelo com número que não está na pergunta não vale: o número viria do modelo.
function safeTitle(titulo, pergunta){
  const q = new Set(String(pergunta).match(/\d+/g) || []);
  return titulo && (String(titulo).match(/\d+/g) || []).every(n => q.has(n)) ? titulo : pergunta;
}
async function ask(o){
  const tabelas = Cat.rankTables(o.catalog, o.pergunta);
  if(!tabelas.length) return { ok:false, motivo:'sem_tabelas' };
  let anterior = null, falha = 'modelo', ultimoSql = '';
  for(let tentativa = 0; tentativa < 2; tentativa++){
    let texto;
    try{ texto = await o.callModel(buildMessages({ pergunta:o.pergunta, tabelas, motor:o.motor, versao:o.versao, schema:o.schema, usuario:o.usuario, anterior }), o.signal); }
    catch(e){ if(e && e.name === 'AbortError') throw e; return { ok:false, motivo:'modelo' }; }
    checkAbort(o.signal);
    const req = parseModelJson(texto);
    if(!req){ falha = 'modelo'; anterior = { sql:'', erro:'A resposta não era o objeto JSON pedido, com sql e um visual da lista.' }; continue; }
    const g = Tb.guardSql(req.sql);
    if(!g.ok){ falha = 'sql'; ultimoSql = req.sql; anterior = { sql:req.sql, erro:g.error }; continue; }
    if(!readsListedTable(g.sql, tabelas)){ falha = 'sql'; ultimoSql = g.sql; anterior = { sql:g.sql, erro:'A consulta precisa ler uma das tabelas listadas.' }; continue; }
    const dobra = somaDoCabecalho(g.sql, tabelas);
    if(dobra){ falha = 'sql'; ultimoSql = g.sql; anterior = { sql:g.sql, erro:dobra }; continue; }
    const sql = Tb.prepareSql(g.sql, o.motor);
    ultimoSql = sql;
    const r = await o.runSql(sql, o.signal);
    checkAbort(o.signal);
    if(!r.ok){
      // SQL Server e Oracle não têm limite de tempo no servidor: só o front parou de esperar.
      if(r.timeout) return { ok:false, motivo:(r.front && (o.motor === 'sqlserver' || o.motor === 'oracle' || o.motor === 'firebird')) ? 'timeout_front' : 'timeout', sql };
      falha = 'sql'; anterior = { sql, erro:r.error }; continue;
    }
    if(!r.rows.length || r.rows.every(l => Object.values(l).every(v => v == null || v === ''))) return { ok:false, motivo:'vazio', sql };
    const colunas = Object.keys(r.rows[0]);
    const v = pickVisual(req, colunas, r.rows);
    const out = { ok:true, visual:v.visual, x:v.x, y:v.y, titulo:safeTitle(req.titulo, o.pergunta),
      frase:frase({ visual:v.visual, x:v.x, y:v.y, linhas:v.linhas, todas:r.rows }), colunas, linhas:v.linhas, todas:r.rows, sql };
    if(v.aviso) out.aviso = v.aviso;
    return out;
  }
  return falha === 'sql' ? { ok:false, motivo:'sql', sql:ultimoSql } : { ok:false, motivo:'modelo' };
}

function modelCaller(o){
  const schema = o.schema || SCHEMA, nome = o.schemaName || 'consulta', maxTokens = o.maxTokens || 1024;
  return async function(messages, signal){
    const body = { model:'cd2', messages, stream:false, temperature:0, max_tokens:maxTokens };
    if(o.useSchema) body.response_format = { type:'json_schema', json_schema:{ name:nome, strict:true, schema } };
    const r = await o.fetch(o.url, { method:'POST', headers:{ 'Content-Type':'application/json' }, body:JSON.stringify(body), signal });
    if(!r.ok) throw new Error('Modelo respondeu HTTP ' + r.status);
    const j = await r.json();
    const m = j && j.choices && j.choices[0] && j.choices[0].message;
    return (m && m.content) || '';
  };
}
function abortError(){ const e = new Error('Interrompido'); e.name = 'AbortError'; return e; }
function checkAbort(signal){ if(signal && signal.aborted) throw abortError(); }
// Prazo do front: vale para todo motor, inclusive os que não cortam a consulta no servidor.
// No prazo ou no Parar, o request ao toolbox é abortado (fetch com signal).
function toolRunner(client, o){
  const ms = (o && o.timeoutMs) || 15000;
  return async function(sql, signal){
    checkAbort(signal);
    const ctl = new AbortController();
    let timer, onAbort;
    const prazo = new Promise(res => { timer = setTimeout(() => { res({ ok:false, timeout:true, error:'tempo esgotado', front:true }); ctl.abort(); }, ms); });
    const parou = new Promise((res, rej) => { onAbort = () => { rej(abortError()); ctl.abort(); }; if(signal) signal.addEventListener('abort', onAbort, { once:true }); });
    try{
      const r = await Promise.race([
        Promise.resolve().then(() => client.call('db_read_only_query', { sql }, ctl.signal)).then(parseRows, e => ({ ok:false, error:String((e && e.message) || e) })),
        prazo, parou
      ]);
      if(!r.ok && !r.timeout && isTimeout(r.error)) r.timeout = true;
      return r;
    }finally{ clearTimeout(timer); if(signal) signal.removeEventListener('abort', onAbort); }
  };
}

const api = { VISUALS, MENSAGENS, SCHEMA, parseRows, parseModelJson, isTimeout, pickVisual, frase, buildMessages, somaDoCabecalho, ask, modelCaller, toolRunner };
if(isNode) module.exports = api; else root.CD2Ask = api;
})(typeof window !== 'undefined' ? window : globalThis);
