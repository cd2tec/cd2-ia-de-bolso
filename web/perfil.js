/* Perfil das tabelas da IA de Bolso: lê as colunas e uma amostra de linhas e diz o papel de
   cada tabela (movimento, itens, cadastro, parametro, vazia ou indefinida) e as colunas úteis.
   Lógica pura e determinística, sem modelo e sem DOM. Só sinais genéricos de ERP (tipo, formato
   dos valores, auditoria, nomes comuns em português e inglês), nunca o nome de um ERP específico. */
(function(root){
"use strict";
const isNode = typeof module !== 'undefined' && module.exports;
const Cat = isNode ? require('./catalog.js') : root.CD2Catalog;

const LINHAS = 10, MAX_VIEWS = 30;
const PAPEIS = ['movimento', 'itens', 'cadastro', 'parametro', 'vazia', 'indefinida'];
// Palavras no formato de Cat.tokens: sem acento, minúsculas, sem plural.
const AUDITORIA = new Set(['cadastro', 'cad', 'inclusao', 'criacao', 'created', 'alteracao', 'alt', 'atualizacao', 'updated', 'modified', 'inserted', 'ultima', 'ult', 'geracao', 'gerado']);
// Datas de atributo da entidade (nascimento, validade...): nunca são a data de um movimento.
const ATRIBUTO = new Set(['nascimento', 'nasc', 'validade', 'admissao', 'demissao', 'expiracao', 'expiration', 'expiry', 'birth', 'birthday', 'vigencia', 'fundacao']);
const VALOR = new Set(['vlr', 'valor', 'vl', 'tot', 'total', 'amount', 'amt', 'preco', 'price', 'prc', 'custo', 'cost', 'receita', 'revenue']);
const UNITARIO = new Set(['preco', 'price', 'prc', 'unit', 'unitario', 'custo', 'cost']);
const TOTAL = new Set(['tot', 'total']);
const QUANTIDADE = new Set(['qtd', 'qtde', 'quant', 'quantidade', 'qty', 'quantity']);
const USUARIO = new Set(['usuario', 'usu', 'user', 'by', 'operador']);
const DESCRICAO = new Set(['ds', 'nm', 'descricao', 'desc', 'nome', 'name', 'description', 'title', 'titulo', 'razao', 'fantasia']);
const CHAVE = new Set(['chave', 'key', 'parametro', 'param', 'setting', 'config', 'cfg', 'opcao', 'option', 'propriedade', 'property']);
const CONTEUDO = new Set(['valor', 'value', 'conteudo', 'content']);
const NOME_PARAMETRO = new Set(['param', 'parametro', 'config', 'cfg', 'setting', 'configuracao']);

const tem = (c, set) => c.tokens.some(t => set.has(t));
// Começo de palavra colada (VLRUNITARIOECF, DTANASCFUND): o separador não parte o resto do nome.
const comeca = (c, prefixos) => c.tokens.some(t => prefixos.some(p => t.startsWith(p)));
const ATRIBUTO_INICIO = ['nasc', 'valid', 'admiss', 'demiss', 'expir', 'vigen', 'fundac', 'birth'];
// Nome e descrição colados no começo de uma palavra composta (NOMERAZAO, DESCCOMPLETA); desconto não é descrição.
const temDescricao = c => tem(c, DESCRICAO) || c.tokens.some(t => /^(nome|desc|razao)/.test(t) && !/^descont/.test(t));
const preenchidos = v => v.filter(x => x != null && x !== '');
function lower(r){ const o = {}; for(const k in r) o[k.toLowerCase()] = r[k]; return o; }

// Sinais de uma coluna. vals é a lista de valores da amostra, ou null sem amostra.
function sinaisColuna(c, vals){
  const distintos = vals ? new Set(preenchidos(vals).map(String)).size : 0;
  const varia = !vals || vals.length < 2 || distintos > 1;
  const chave = Cat.isKey(c);
  const valor = c.kind === 'numero' && !chave && tem(c, VALOR);
  const unitario = valor && (tem(c, UNITARIO) || comeca(c, ['unit'])) && !tem(c, TOTAL);
  return {
    dataMov: c.kind === 'data' && !tem(c, AUDITORIA) && !tem(c, ATRIBUTO) && !comeca(c, ATRIBUTO_INICIO) && varia,
    dataAud: c.kind === 'data' && tem(c, AUDITORIA),
    valorTotal: valor && !unitario,
    unitario,
    quantidade: c.kind === 'numero' && !chave && tem(c, QUANTIDADE),
    chave,
    idRegistro: chave && !!vals && vals.length >= 2 && distintos === vals.length,
    usuario: tem(c, USUARIO),
    descricao: c.kind === 'texto' && temDescricao(c),
    chaveNome: tem(c, CHAVE),
    conteudo: c.kind === 'texto' && tem(c, CONTEUDO)
  };
}

// Papel de uma tabela. A ordem importa: vazia, parâmetro, movimento, cadastro, indefinida.
// O identificador por linha reforça o movimento, mas não é exigido: view agregada não tem.
function analisarTabela(t, linhas){
  const amostra = Array.isArray(linhas);
  const L = amostra ? linhas.slice(0, LINHAS).map(lower) : null;
  if(amostra && L.length === 0) return { papel:'vazia', colunas:{}, amostra:true, chave:null };
  const s = t.columns.map(c => ({ c, x:sinaisColuna(c, L ? L.map(r => r[String(c.name).toLowerCase()]) : null) }));
  const primeiro = k => { const f = s.find(e => e.x[k]); return f ? f.c.name : null; };
  const algum = k => s.some(e => e.x[k]);
  const colunas = {};
  if(primeiro('dataMov')) colunas.data = primeiro('dataMov');
  if(primeiro('valorTotal')) colunas.valor = primeiro('valorTotal');
  if(primeiro('quantidade')) colunas.quantidade = primeiro('quantidade');
  const chave = primeiro('idRegistro') || (L ? null : primeiro('chave'));
  const dataMov = algum('dataMov');
  const nome = Cat.tokens(t.name);
  let papel = 'indefinida';
  if(!dataMov && t.columns.length <= 8 && ((algum('chaveNome') && algum('conteudo')) || nome.some(x => NOME_PARAMETRO.has(x)))) papel = 'parametro';
  else if(dataMov && (algum('valorTotal') || algum('quantidade'))) papel = 'movimento';
  // Aqui a tabela já não é movimento: data que varia sem valor nem quantidade (ativação, última compra) não
  // impede o cadastro.
  else if(!(algum('quantidade') && !algum('descricao')) && (algum('descricao') || (algum('usuario') && algum('dataAud')) || algum('unitario'))) papel = 'cadastro';
  return { papel, colunas, amostra, chave };
}

function resumo(catalog){
  const r = Object.fromEntries(PAPEIS.map(p => [p, 0]));
  for(const t of (catalog && catalog.tables) || []) if(t.perfil) r[t.perfil.papel]++;
  return r;
}

// Perfil de todas as tabelas. Segunda passada: tabela com valor ou quantidade que se liga a uma
// tabela de movimento pela chave do registro dela vira itens (mov_venda_item pelo nr_cupom,
// order_lines pelo order_id -> sales_orders.id). Movimento só vira itens se tiver item no nome; cadastro só se tiver quantidade.
// o.so (Set de nomes, opcional): só essas tabelas ganham perfil novo, as outras mantêm o que têm (o
// entendimento guardado) e servem de movimento para a segunda passada. É o caso do "Atualizar com o banco".
function analisarCatalogo(catalog, amostras, o){
  const tabs = (catalog && catalog.tables) || [], so = o && o.so;
  const nova = t => !so || so.has(t.name);
  for(const t of tabs) if(nova(t)) t.perfil = analisarTabela(t, amostras ? amostras.get(t.name) : null);
  const movs = tabs.filter(t => t.perfil && t.perfil.papel === 'movimento' && t.perfil.chave);
  for(const t of tabs){
    if(!nova(t)) continue;
    const p = t.perfil;
    if(p.papel !== 'indefinida' && p.papel !== 'movimento' && !(p.papel === 'cadastro' && p.colunas.quantidade)) continue;
    if(!p.colunas.valor && !p.colunas.quantidade) continue;
    if(p.papel === 'movimento' && !Cat.tokens(t.name).some(x => Cat.NOME_ITEM.has(x))) continue;
    const liga = movs.some(m => m !== t && Cat.links([m, t]).some(l => (l.a === m.name ? l.col : l.colB) === m.perfil.chave));
    if(liga) p.papel = 'itens';
  }
  return resumo(catalog);
}

// Lê a amostra de cada tabela sem pesar no banco: no máximo `paralelo` tabelas ao mesmo tempo;
// views por último e uma de cada vez; só as `maxViews` primeiras views. Cada amostra corre contra
// o `prazo`: no SQL Server, no Oracle e no MariaDB o banco segue executando depois que o front
// desiste, então a primeira view lenta encerra as views e 3 tabelas lentas encerram as tabelas.
const PRAZO = 15000, MAX_ESTOUROS = 3, ESTOUROU = {};
async function amostrar(catalog, sample, o){
  o = o || {};
  const tabs = (catalog && catalog.tables) || [], out = new Map(), vivo = o.vivo || (() => true);
  const maxViews = o.maxViews == null ? MAX_VIEWS : o.maxViews, prazo = o.prazo == null ? PRAZO : o.prazo;
  const tabelas = tabs.filter(t => t.type !== 'VIEW'), views = tabs.filter(t => t.type === 'VIEW');
  const total = tabs.length; let feitos = 0, estourosTabela = 0, viewLenta = false;
  const conta = () => { feitos++; if(o.onProgresso) o.onProgresso(feitos, total); };
  const comPrazo = nome => new Promise(res => {
    const timer = setTimeout(() => res(ESTOUROU), prazo);
    const fim = r => { clearTimeout(timer); res(r); };
    Promise.resolve().then(() => sample(nome)).then(fim, () => fim(null));
  });
  const um = async (t, view) => {
    const r = await comPrazo(t.name);
    if(r === ESTOUROU){ if(view) viewLenta = true; else estourosTabela++; }
    if(!vivo()) return;
    out.set(t.name, Array.isArray(r) ? r.slice(0, LINHAS) : null); conta();
  };
  // Pulada conta no progresso, para a faixa fechar no total.
  const pula = t => { out.set(t.name, null); conta(); };
  let i = 0;
  const trabalhador = async () => {
    while(i < tabelas.length && vivo()){ const t = tabelas[i++]; if(estourosTabela >= MAX_ESTOUROS) pula(t); else await um(t, false); }
  };
  await Promise.all(Array.from({ length:Math.max(1, Math.min(o.paralelo || 2, tabelas.length)) }, trabalhador));
  for(const [k, v] of views.entries()){
    if(!vivo()) break;
    if(k < maxViews && !viewLenta) await um(v, true); else pula(v);
  }
  return out;
}

const api = { PAPEIS, LINHAS, MAX_VIEWS, PRAZO, analisarTabela, analisarCatalogo, resumo, amostrar };
if(isNode) module.exports = api; else root.CD2Perfil = api;
})(typeof window !== 'undefined' ? window : globalThis);
