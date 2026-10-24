/* Mapa do banco para o roteiro de relatórios da IA de Bolso: monta o catálogo a partir
   do db_catalog, escolhe as tabelas que combinam com a pergunta e sugere perguntas tiradas
   do próprio esquema. Lógica pura, sem modelo e sem DOM. */
(function(root){
"use strict";
const MAX_COLS_TABLE = 60;
// Prefixos comuns de coluna em ERP, inclusive colados no nome (DTAVENCIMENTO, SEQPESSOA, USUALTERACAO).
// Separados só quando o resto tem 3 letras ou mais; dta e dth antes de dt para não sobrar 'avencimento'.
const PREFIXES = ['vlr', 'qtd', 'dta', 'dth', 'dt', 'cod', 'cd', 'tot', 'nro', 'nr', 'ds', 'nm', 'seq', 'usu'];
// Palavras cruas e os stems delas: 'mais' vira 'mal' no stem, então a crua precisa estar aqui.
const STOP_RAW = ['de','da','do','das','dos','por','e','o','a','os','as','em','no','na','nos','nas','qual','quais','quanto',
  'quantos','quanta','quantas','mostre','mostra','me','um','uma','com','para','pelo','pela','cada','todo','todos','the','of','by',
  'per','what','how','many','show','list','liste','foram','tem','temos','ha','sao','que','mais','menos','maior','menor','meu','minha'];
const STOP = new Set(STOP_RAW.concat(STOP_RAW.map(stem)));
const WHOLE = new Set(['codigo','total','totais','numero','nome','descricao','usuario','usuarios','sequencia']);
const SYN = [
  ['venda','sale','pedido','order','cupom','vend','vendido','vendida','faturamento','fatura','receita','ticket'],
  ['produto','product','item','sku','mercadoria','prod','artigo'],
  ['cliente','customer','client','cli','consumidor'],
  ['data','date','dt','dia','mes','ano','periodo','day','month','year','time','semana',
    'janeiro','fevereiro','marco','abril','maio','junho','julho','agosto','setembro','outubro','novembro','dezembro'],
  ['valor','vlr','total','tot','amount','faturamento','receita','custo','ticket'],
  ['preco','price','unitario','unit'],
  ['quantidade','qtd','qty','quant'],
  ['loja','store','filial','branch','unidade'],
  ['departamento','depto','dept','department','secao','categoria','category'],
  ['estado','uf','state']
];
const NOISE = new Set(['log','tmp','temp','aux','bkp','backup','audit','auditoria','old','hist']);

function stem(t){
  if(t.length <= 3) return t;
  if(t.endsWith('ns')) return t.slice(0, -2) + 'm';
  if(t.endsWith('oes') || t.endsWith('aes')) return t.slice(0, -3) + 'ao';
  if(t.endsWith('eses')) return t.slice(0, -2);
  if(t.endsWith('ais')) return t.slice(0, -3) + 'al';
  if(t.endsWith('eis')) return t.slice(0, -3) + 'el';
  return t.endsWith('s') ? t.slice(0, -1) : t;
}
function tokens(s, semStop){
  const base = String(s == null ? '' : s).replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase().split(/[^a-z0-9]+/).filter(Boolean);
  const out = [];
  for(let t of base){
    if(semStop && (STOP.has(t) || STOP.has(stem(t)))) continue;
    if(WHOLE.has(t)){ out.push(stem(t)); continue; }
    let p;
    while((p = PREFIXES.find(x => t.length >= x.length + 3 && t.startsWith(x)))){ out.push(p); t = t.slice(p.length); }
    if(t.length >= 6 && t.endsWith('id')){ out.push(stem(t.slice(0, -2)), 'id'); continue; }
    out.push(stem(t));
  }
  return out;
}
const SYN_INDEX = new Map();
for(const g of SYN){ const set = g.map(stem); for(const w of set){ const cur = SYN_INDEX.get(w) || new Set(); set.forEach(x => cur.add(x)); SYN_INDEX.set(w, cur); } }
function syn(q){ return SYN_INDEX.get(q) || new Set([q]); }
// Também casa quando a palavra da pergunta começa com um sinônimo de 4+ letras ('vendemos' e 'vend').
function match(q, x){
  if(q === x || syn(q).has(x)) return true;
  if(q.length >= 4 && x.length >= 4 && (x.startsWith(q) || q.startsWith(x))) return true;
  for(const w of syn(x)) if(w.length >= 4 && q.startsWith(w)) return true;
  return false;
}
function questionTokens(p){ return [...new Set(tokens(p, true).filter(t => t.length >= 2 && !STOP.has(t)))]; }

function kindOf(type){
  const t = String(type || '').toLowerCase();
  if(/interval/.test(t)) return 'outro';
  if(/date|time/.test(t)) return 'data';
  if(/^(bit|bool)/.test(t)) return 'outro';
  if(/char|text|clob|string|enum/.test(t)) return 'texto';
  if(/int|num|dec|float|double|real|money|serial/.test(t)) return 'numero';
  return 'outro';
}
function lowerKeys(r){ const o = {}; for(const k in r) o[k.toLowerCase()] = r[k]; return o; }

function buildCatalog(rows){
  const by = new Map();
  for(const raw of rows || []){
    if(!raw || typeof raw !== 'object') continue;
    const r = lowerKeys(raw);
    if(r.table_name == null || r.column_name == null) continue;
    const name = String(r.table_name);
    let t = by.get(name);
    if(!t){
      t = { name, type:String(r.table_type).toUpperCase() === 'VIEW' ? 'VIEW' : 'TABLE', rows:Math.max(0, Number(r.num_rows) || 0),
        // Planilha e DBF não informam linhas (num_rows nulo): a tela e o prompt não mostram "cerca de 0 linhas".
        semContagem:r.num_rows == null || r.num_rows === '', tokens:tokens(name), columns:[] };
      by.set(name, t);
    }
    if(t.columns.length >= MAX_COLS_TABLE) continue;
    t.columns.push({ name:String(r.column_name), type:String(r.data_type == null ? '' : r.data_type), kind:kindOf(r.data_type), tokens:tokens(r.column_name) });
  }
  return { tables:[...by.values()] };
}

function pickColumns(cols, hit, n){
  const rank = c => hit.has(c.name) ? 0 : c.kind === 'data' ? 1 : c.kind === 'numero' ? 2 : 3;
  // Coluna com nota da TI entra sempre, mesmo além de n: a TI escreveu por um motivo.
  const ord = cols.map((c, i) => ({ c, i })).sort((a, b) => rank(a.c) - rank(b.c) || a.i - b.i);
  const pick = ord.slice(0, n), usados = new Set(pick);
  for(const x of ord) if(x.c.nota && !usados.has(x)) pick.push(x);
  return pick.sort((a, b) => a.i - b.i).map(({ c }) => { const o = { name:c.name, type:c.type, kind:c.kind }; if(c.nota) o.nota = c.nota; return o; });
}
// Tipo de pergunta, pelas palavras cruas (as de parada contam aqui: "quanto", "quantos").
// Medida: valor, total, faturamento, contagem de movimento, evolução no tempo. Entidade: contar ou
// listar cadastro ("quantos clientes", "lista de lojas"). Medida tem precedência.
const MEDIDA_RAW = ['total','totais','faturamento','faturado','fatura','receita','valor','valores','quanto','quanta','soma','somar','media',
  'ticket','vendeu','vendemos','vendido','vendidos','vendida','vendidas','venda','vendas','pedido','pedidos','cupom','cupons','nota','notas',
  'quantidade','revenue','sales','sale','orders','order','amount','sum','average','avg'];
const TEMPO_RAW = ['mes','dia','ano','semana','trimestre','month','day','year','week'];
const ENTIDADE_RAW = ['quantos','quantas','lista','liste','listar','quais','cadastro','cadastrados','cadastradas','temos','existem','many','list'];
const ITEM_RAW = ['produto','produtos','item','itens','departamento','departamentos','categoria','categorias','mercadoria','mercadorias',
  'sku','secao','marca','product','products','category','categories','department'];
const MEDIDA_Q = new Set(MEDIDA_RAW.map(stem));
function palavras(p){
  return String(p == null ? '' : p).normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase().split(/[^a-z0-9]+/).filter(Boolean);
}
function tipoPergunta(p){
  const w = palavras(p);
  const noTempo = w.some((x, i) => TEMPO_RAW.includes(x) && (w[i - 1] === 'por' || w[i - 1] === 'per'));
  if(noTempo || w.some(x => MEDIDA_RAW.includes(x))) return 'medida';
  if(w.some(x => ENTIDADE_RAW.includes(x))) return 'entidade';
  return 'neutra';
}
// Ajuste da nota pelo papel. Cadastro só desce em pergunta de medida quando nenhuma palavra de
// entidade da pergunta bate com o nome dele: em "faturamento por loja", a tabela loja continua
// candidata (para o JOIN do nome); um cadastro de preço que só bate em "venda" desce.
function ajustePapel(papel, tipo, nomeEntidade, citaItem){
  if(tipo === 'medida'){
    if(papel === 'movimento') return 4;
    if(papel === 'itens') return citaItem ? 4 : 2;
    if(papel === 'cadastro') return nomeEntidade ? 0 : -3;
    if(papel === 'parametro') return -6;
  }else if(tipo === 'entidade'){
    if(papel === 'cadastro') return 3;
    if(papel === 'parametro') return -6;
  }else if(papel === 'parametro') return -3;
  return 0;
}
// Nota: +3 por palavra no nome da tabela, +1 por palavra em coluna (até 5), +1,5 por palavra na
// descrição do modelo (até 3), +1 com data, +1 com número, desempate pelo volume (view sem contagem
// fica neutra, como uma tabela de umas mil linhas: não ganha nem perde de saída), mais o ajuste do
// papel. Só entra quem tem ao menos uma palavra em comum com a pergunta, e tabela vazia nunca entra.
function rankTables(catalog, pergunta, opts){
  opts = opts || {};
  const max = opts.max || 5, maxCols = opts.maxCols || 25, qs = questionTokens(pergunta), scored = [];
  const tipo = tipoPergunta(pergunta), citaItem = palavras(pergunta).some(x => ITEM_RAW.includes(x));
  for(const t of (catalog && catalog.tables) || []){
    const papel = t.perfil && t.perfil.papel;
    if(papel === 'vazia') continue;
    // A nota da TI conta como descrição: mesmas palavras, mesmo peso (1,5, até 3 palavras).
    const desc = ((t.perfil && t.perfil.descricaoTokens) || []).concat(t.notaTokens || []);
    let tableHits = 0, nomeEntidade = 0, colHits = 0, descHits = 0; const hitCols = new Set();
    for(const q of qs){
      if(t.tokens.some(x => match(q, x))){ tableHits++; if(!MEDIDA_Q.has(q)) nomeEntidade++; }
      let hit = false;
      for(const c of t.columns) if(c.tokens.some(x => match(q, x))){ hitCols.add(c.name); hit = true; }
      if(hit) colHits++;
      if(desc.some(x => match(q, x))) descHits++;
    }
    if(!tableHits && !colHits && !descHits) continue;
    const hasData = t.columns.some(c => c.kind === 'data'), hasNum = t.columns.some(c => c.kind === 'numero');
    const volume = t.type === 'VIEW' && !t.rows ? 0.03 : Math.log10(t.rows + 1) / 100;
    let score = 3 * tableHits + Math.min(5, colHits) + 1.5 * Math.min(3, descHits) + (hasData ? 1 : 0) + (hasNum ? 1 : 0) + volume;
    if(t.tokens.some(x => NOISE.has(x))) score /= 2;
    score += ajustePapel(papel, tipo, nomeEntidade, citaItem);
    scored.push({ t, score, hitCols });
  }
  scored.sort((a, b) => b.score - a.score || a.t.name.localeCompare(b.t.name));
  return scored.slice(0, max).map(({ t, score, hitCols }) => ({ name:t.name, type:t.type, rows:t.rows, semContagem:!!t.semContagem, score,
    columns:pickColumns(t.columns, hitCols, maxCols), perfil:t.perfil || null, nota:t.nota || '' }));
}

function humanize(name){
  const raw = String(name == null ? '' : name);
  const t = raw.replace(/([a-z0-9])([A-Z])/g, '$1 $2').normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase()
    .split(/[^a-z0-9]+/).filter(x => x && x !== 'id');
  return t.length ? t.join(' ') : raw.toLowerCase();
}
// Chave: palavra que começa com id, cod, cd, nr ou nro, ou que termina em id.
function isKey(c){ return c.tokens.some(t => /^(id|cod|cd|nr|nro|seq)/.test(t) || t.endsWith('id')); }
// Ligações entre as tabelas candidatas. Dois jeitos de ligar, os dois comuns em ERP:
// 1. a mesma coluna-chave nas duas tabelas (cd_prod e cd_prod, nr_cupom e nr_cupom). Muito ERP
//    antigo não declara chave estrangeira, então o nome é a pista. id ou codigo sozinho existe em
//    quase toda tabela e ligaria coisas sem relação;
// 2. <entidade>_id, id_<entidade> ou entidadeId apontando para a tabela da entidade, no singular ou
//    no plural, que tem a coluna id (customer_id -> customers.id, id_cliente -> clientes.id).
const GENERICA = new Set(['id', 'cod', 'codigo', 'seq']);
const NOME_ITEM = new Set(['item', 'line', 'linha', 'detalhe']);
const colTokens = c => c.tokens || tokens(c.name);
const isLinkKey = c => { const n = String(c.name).toLowerCase(); return !GENERICA.has(n) && isKey({ tokens:colTokens(c) }); };
const raiz = w => String(w).toLowerCase().replace(/ies$/, 'y').replace(/s$/, '');
const ultimaPalavra = n => { const p = String(n).replace(/([a-z0-9])([A-Z])/g, '$1_$2').toLowerCase().split(/[^a-z0-9]+/).filter(Boolean); return p[p.length - 1] || ''; };
function entidadeRef(nome){
  const n = String(nome), m = n.match(/^(.+?)_id$/i) || n.match(/^id_(.+)$/i) || n.match(/^(.*[a-z])Id$/);
  return m ? raiz(ultimaPalavra(m[1])) : '';
}
function links(tabs){
  const out = [], visto = new Set();
  const add = (a, col, b, colB) => { const k = a + '.' + col + '=' + b + '.' + colB; if(!visto.has(k)){ visto.add(k); out.push({ a, b, col, colB }); } };
  for(let i = 0; i < tabs.length; i++) for(let j = i + 1; j < tabs.length; j++)
    for(const c of tabs[i].columns) if(isLinkKey(c) && tabs[j].columns.some(d => String(d.name).toLowerCase() === String(c.name).toLowerCase()))
      add(tabs[i].name, c.name, tabs[j].name, c.name);
  for(const a of tabs) for(const c of a.columns){
    const e = entidadeRef(c.name);
    if(!e) continue;
    const cands = tabs.filter(b => b !== a && raiz(ultimaPalavra(b.name)) === e && b.columns.some(d => String(d.name).toLowerCase() === 'id'));
    // order_id com sales_orders e purchase_orders: ambíguo, só liga à tabela que se chama order(s).
    const alvo = cands.length > 1 ? cands.filter(b => raiz(String(b.name).toLowerCase().replace(/[^a-z0-9]/g, '')) === e) : cands;
    for(const b of alvo) add(a.name, c.name, b.name, b.columns.find(d => String(d.name).toLowerCase() === 'id').name);
  }
  return out;
}
// Cabeçalho e itens: a tabela de itens tem item, line, linha ou detalhe no nome e se liga ao
// cabeçalho, e o cabeçalho tem todas as palavras dela no nome (mov_venda e mov_venda_item) ou é
// apontado por uma referência <entidade>_id dos itens e divide com ela uma palavra do nome
// (order_lines.order_id -> sales_orders.id; order_lines.product_id -> products não é cabeçalho).
function itemPairs(tabs){
  const out = [];
  for(const t of tabs){
    const tt = tokens(t.name);
    if(!tt.some(x => NOME_ITEM.has(x))) continue;
    for(const h of tabs){
      const ht = tokens(h.name);
      if(h === t || !ht.length || ht.some(x => NOME_ITEM.has(x))) continue;
      const contido = ht.every(x => tt.includes(x)), comum = ht.some(x => tt.includes(x));
      const l = links([h, t]).find(x => contido || (comum && x.a === t.name && x.b === h.name && x.colB !== x.col));
      if(l) out.push({ cab:h.name, itens:t.name, col:l.a === t.name ? l.col : l.colB });
    }
  }
  return out;
}
const NAO_MEDIDA = new Set(['seq', 'ano', 'cep', 'mes', 'dia']);
const papelDe = t => t.perfil && t.perfil.papel;
// Com perfil, o fato sai do movimento (depois dos itens), view inclusive, antes do volume; cadastro
// (o de preço costuma ser a maior tabela), parâmetro e vazia nunca são o fato.
function escolheFato(todas, tables, ok){
  if(!todas.some(t => t.perfil)) return tables.find(ok);
  const vivas = todas.filter(t => !t.tokens.some(x => NOISE.has(x))).sort((a, b) => b.rows - a.rows || a.name.localeCompare(b.name));
  for(const p of ['movimento', 'itens']){ const f = vivas.find(t => papelDe(t) === p && ok(t)); if(f) return f; }
  return tables.find(t => !['cadastro', 'parametro', 'vazia'].includes(papelDe(t)) && ok(t));
}
function suggest(catalog){
  const todas = (catalog && catalog.tables) || [];
  const tables = todas.filter(t => t.type === 'TABLE' && !t.tokens.some(x => NOISE.has(x)))
    .slice().sort((a, b) => b.rows - a.rows || a.name.localeCompare(b.name));
  const out = [], used = new Set();
  const add = (s, t) => { if(out.length < 4 && !out.includes(s)){ out.push(s); used.add(t.name); } };
  const val = t => t.columns.find(c => c.kind === 'numero' && !isKey(c) && !c.tokens.some(x => NAO_MEDIDA.has(x)));
  const txt = t => t.columns.find(c => c.kind === 'texto' && !isKey(c));
  const fato = escolheFato(todas, tables, t => t.columns.some(c => c.kind === 'data') && val(t));
  if(fato) add('Total por mês em ' + humanize(fato.name), fato);
  const top = tables.find(t => t !== fato && val(t) && txt(t));
  if(top) add('Top 10 de ' + humanize(top.name) + ' por ' + humanize(val(top).name), top);
  if(fato && txt(fato)) add('Distribuição de ' + humanize(fato.name) + ' por ' + humanize(txt(fato).name), fato);
  for(const t of tables){ if(out.length >= 4) break; if(!used.has(t.name)) add('Quantos registros tem ' + humanize(t.name), t); }
  return out;
}

const api = { tokens, questionTokens, kindOf, buildCatalog, rankTables, suggest, humanize, links, itemPairs, isKey, NOME_ITEM, tipoPergunta };
if(typeof module !== 'undefined' && module.exports) module.exports = api; else root.CD2Catalog = api;
})(typeof window !== 'undefined' ? window : globalThis);
