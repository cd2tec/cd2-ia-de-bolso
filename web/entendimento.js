/* Entendimento do banco da IA de Bolso: o perfil e a descrição de cada tabela, guardados pelo cd2-ctl
   como um documento por banco para não refazer tudo a cada abertura. Aqui só a lógica pura, sem DOM:
   capturar do catálogo, aplicar de volta, comparar com o banco atual, limpar as notas do que sumiu,
   mesclar e reconhecer se o banco parece de um ERP de varejo. Nenhuma linha de amostra entra no documento. */
(function(root){
"use strict";
const isNode = typeof module !== 'undefined' && module.exports;
const Cat = isNode ? require('./catalog.js') : root.CD2Catalog;

const TEXTO_VAREJO = 'Este banco não parece ser de um ERP de varejo. A IA de Bolso foi feita para funcionar melhor com ERPs de varejo: ela vai tentar ajudar mesmo assim, mas pode errar mais.';
// Papéis que deveriam ter descrição da IA; parâmetro e vazia não passam pelo modelo.
const PEDEM_DESCRICAO = new Set(['movimento', 'itens', 'indefinida', 'cadastro']);

const tem = (o, k) => Object.prototype.hasOwnProperty.call(o, k);
// Atribui sem passar pelo setter de __proto__ (há bancos com tabela chamada assim).
function pôr(o, k, v){ Object.defineProperty(o, k, { value:v, enumerable:true, writable:true, configurable:true }); }
const objeto = v => v !== null && typeof v === 'object' && !Array.isArray(v);
const baixo = s => String(s).toLowerCase();
const tabelasDe = c => (c && Array.isArray(c.tables)) ? c.tables : [];
const camposDe = t => (t && Array.isArray(t.columns)) ? t.columns.map(c => String(c.name)) : [];
// Mapa nome-em-minúsculas -> [nome original, valor] das chaves próprias de um objeto.
function porNome(o){
  const m = new Map();
  if(objeto(o)) for(const k of Object.keys(o)) if(!m.has(baixo(k))) m.set(baixo(k), [k, o[k]]);
  return m;
}

// Só os campos do perfil que vão para o disco: nada de tokens nem de linhas.
function perfilParaDoc(p){
  const e = { papel:p.papel, colunas:objeto(p.colunas) ? { ...p.colunas } : {}, chave:p.chave == null ? null : p.chave, amostra:!!p.amostra };
  if(typeof p.descricao === 'string' && p.descricao) e.descricao = p.descricao;
  return e;
}

// Tira do perfil (já em forma de documento) as colunas úteis e a chave que não estão mais entre os
// campos atuais da tabela (sem caixa); as que ficam passam a ter a grafia atual. Sem isso o prompt
// mandaria "data: DT_VENDA" numa tabela que não tem mais essa coluna. Devolve true se tirou alguma.
function podar(p, campos){
  const atual = new Map(campos.map(c => [baixo(c), c]));
  let perdeu = false;
  const colunas = {};
  for(const k of Object.keys(p.colunas)){
    const v = p.colunas[k];
    const c = (typeof v === 'string') ? atual.get(baixo(v)) : undefined;
    if(c === undefined){ perdeu = true; continue; }
    pôr(colunas, k, c);
  }
  p.colunas = colunas;
  if(p.chave != null){
    const c = atual.get(baixo(p.chave));
    if(c === undefined){ p.chave = null; perdeu = true; } else p.chave = c;
  }
  return perdeu;
}

function capturar(catalog, agora){
  const tabelas = {};
  for(const t of tabelasDe(catalog)){
    const e = (t.perfil && typeof t.perfil.papel === 'string') ? perfilParaDoc(t.perfil) : {};
    e.campos = camposDe(t);
    pôr(tabelas, t.name, e);
  }
  return { versao:1, atualizado:(agora || new Date()).toISOString(), completo:true, tabelas };
}

// Documento utilizável: versão 1, completo e com cada tabela bem formada.
function valido(doc){
  if(!objeto(doc) || doc.versao !== 1 || doc.completo !== true || !objeto(doc.tabelas)) return false;
  for(const k of Object.keys(doc.tabelas)){
    const e = doc.tabelas[k];
    if(!objeto(e) || !Array.isArray(e.campos)) return false;
    if(e.papel !== undefined && (typeof e.papel !== 'string' || !objeto(e.colunas))) return false;
    if(e.descricao !== undefined && typeof e.descricao !== 'string') return false;
  }
  return true;
}

// Repõe o perfil guardado. Tabela do banco fora do documento, ou no documento sem perfil (nova, vinda
// de mesclar), vai em desconhecidas para ser entendida. Coluna útil ou chave que não existe mais na
// tabela sai do perfil, e a tabela vai em perderam.
function aplicar(catalog, doc){
  if(!valido(doc)) return { ok:false, desconhecidas:[], perderam:[] };
  const m = porNome(doc.tabelas);
  const desconhecidas = [], perderam = [];
  for(const t of tabelasDe(catalog)){
    const achou = m.get(baixo(t.name));
    const e = achou && achou[1];
    if(!e || typeof e.papel !== 'string'){ desconhecidas.push(t.name); continue; }
    const p = perfilParaDoc(e);
    if(podar(p, camposDe(t))) perderam.push(t.name);
    if(p.descricao) p.descricaoTokens = Cat.tokens(p.descricao, true);
    t.perfil = p;
  }
  return { ok:true, desconhecidas, perderam };
}

// Lê a tabela como palavra inteira, sem caixa e fora de literais (mesma ideia de readsListedTable do ask.js).
function leTabela(sql, nome){
  const s = String(sql == null ? '' : sql).replace(/'(?:[^']|'')*'/g, ' ');
  const esc = String(nome).replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  return new RegExp('(^|[^A-Za-z0-9_$#])' + esc + '([^A-Za-z0-9_$#]|$)', 'i').test(s);
}
const temDescricao = p => typeof p.descricao === 'string' && !!p.descricao.trim();
const textoNota = v => (typeof v === 'string' ? v.trim() : '');

function comparar(catalog, doc, notas){
  const docTabs = porNome(doc && objeto(doc.tabelas) ? doc.tabelas : {});
  const cat = new Map(tabelasDe(catalog).map(t => [baixo(t.name), t]));
  const novasTabelas = [], novosCampos = [], semDescricao = [], perfilPerdido = [];
  const sumiram = { tabelas:[], campos:[] };
  for(const t of tabelasDe(catalog)){
    const achou = docTabs.get(baixo(t.name));
    if(!achou){ novasTabelas.push(t.name); continue; }
    const e = objeto(achou[1]) ? achou[1] : {};
    const antes = Array.isArray(e.campos) ? e.campos.map(String) : [];
    const agora = camposDe(t);
    const setAntes = new Set(antes.map(baixo)), setAgora = new Set(agora.map(baixo));
    for(const c of agora) if(!setAntes.has(baixo(c))) novosCampos.push({ tabela:t.name, campo:c });
    for(const c of antes) if(!setAgora.has(baixo(c))) sumiram.campos.push({ tabela:t.name, campo:c });
    // Entrada sem perfil (tabela nova de uma atualização interrompida) também precisa ser entendida.
    if((typeof e.papel !== 'string' || PEDEM_DESCRICAO.has(e.papel)) && !temDescricao(e)) semDescricao.push(t.name);
    // Perfil guardado que aponta para coluna útil ou chave que sumiu: precisa ser refeito (sem o modelo).
    if(typeof e.papel === 'string' && objeto(e.colunas) && podar(perfilParaDoc(e), agora)) perfilPerdido.push(t.name);
  }
  for(const [k, [nome]] of docTabs) if(!cat.has(k)) sumiram.tabelas.push(nome);

  // Notas que vão sair junto com o que sumiu.
  const notasApagadas = [];
  const notasTabs = porNome(notas && notas.tabelas);
  const sumidas = new Set(sumiram.tabelas.map(baixo));
  const camposSumidos = new Map();
  for(const { tabela, campo } of sumiram.campos){
    const k = baixo(tabela);
    if(!camposSumidos.has(k)) camposSumidos.set(k, new Set());
    camposSumidos.get(k).add(baixo(campo));
  }
  for(const [k, [nome, n]] of notasTabs){
    if(!objeto(n)) continue;
    const campos = objeto(n.campos) ? n.campos : {};
    if(sumidas.has(k)){
      if(textoNota(n.nota)) notasApagadas.push({ tipo:'tabela', onde:nome, nota:textoNota(n.nota) });
      for(const c of Object.keys(campos)) if(textoNota(campos[c])) notasApagadas.push({ tipo:'campo', onde:nome + '.' + c, nota:textoNota(campos[c]) });
    } else if(camposSumidos.has(k)){
      const set = camposSumidos.get(k);
      for(const c of Object.keys(campos)) if(set.has(baixo(c)) && textoNota(campos[c])) notasApagadas.push({ tipo:'campo', onde:nome + '.' + c, nota:textoNota(campos[c]) });
    }
  }
  const consultas = (notas && Array.isArray(notas.consultas)) ? notas.consultas : [];
  const consultasAfetadas = consultas.filter(q => q && sumiram.tabelas.some(n => leTabela(q.sql, n))).map(q => q.nome);

  const nadaMudou = !novasTabelas.length && !novosCampos.length && !sumiram.tabelas.length && !sumiram.campos.length && !semDescricao.length && !perfilPerdido.length;
  return { novasTabelas, novosCampos, sumiram, notasApagadas, semDescricao, perfilPerdido, consultasAfetadas, nadaMudou };
}

// Cópia das notas sem as das tabelas e campos que sumiram. Consultas ficam todas.
function limparNotas(notas, diff){
  const s = (diff && diff.sumiram) || {};
  const sumidas = new Set((s.tabelas || []).map(baixo));
  const camposSumidos = new Map();
  for(const { tabela, campo } of s.campos || []){
    const k = baixo(tabela);
    if(!camposSumidos.has(k)) camposSumidos.set(k, new Set());
    camposSumidos.get(k).add(baixo(campo));
  }
  const out = { ...(objeto(notas) ? notas : {}) };
  const tabelas = {};
  const orig = (objeto(notas) && objeto(notas.tabelas)) ? notas.tabelas : {};
  for(const k of Object.keys(orig)){
    if(sumidas.has(baixo(k))) continue;
    const n = orig[k];
    if(!objeto(n)){ pôr(tabelas, k, n); continue; }
    const tira = camposSumidos.get(baixo(k));
    const campos = {};
    const oc = objeto(n.campos) ? n.campos : {};
    for(const c of Object.keys(oc)) if(!(tira && tira.has(baixo(c)))) pôr(campos, c, oc[c]);
    pôr(tabelas, k, { ...n, campos });
  }
  out.tabelas = tabelas;
  out.consultas = (objeto(notas) && Array.isArray(notas.consultas)) ? notas.consultas.map(q => objeto(q) ? { ...q } : q) : [];
  return out;
}

// Documento novo: sem o que sumiu, com as tabelas novas sem perfil e os campos atuais de todas.
// Colunas úteis e chave que não existem mais saem do perfil guardado.
function mesclar(doc, catalog, diff, agora){
  const docTabs = porNome(doc && objeto(doc.tabelas) ? doc.tabelas : {});
  const sumidas = new Set(((diff && diff.sumiram && diff.sumiram.tabelas) || []).map(baixo));
  const tabelas = {};
  for(const t of tabelasDe(catalog)){
    const k = baixo(t.name);
    if(sumidas.has(k)) continue;
    const achou = docTabs.get(k);
    const e = (achou && objeto(achou[1]) && typeof achou[1].papel === 'string') ? perfilParaDoc(achou[1]) : {};
    if(typeof e.papel === 'string') podar(e, camposDe(t));
    e.campos = camposDe(t);
    pôr(tabelas, t.name, e);
  }
  const novo = { versao:1, atualizado:(agora || new Date()).toISOString(), completo:!!(doc && doc.completo), tabelas };
  if(doc && objeto(doc.varejo)) novo.varejo = { ...doc.varejo };
  return novo;
}

// Grupos de sinais de varejo, já na forma que Cat.tokens devolve (sem acento, minúsculo, com stem;
// 'fornecedores' vira 'fornecedore' e 'operadores' vira 'operadore').
const GRUPOS = [
  ['produto', 'product', 'item', 'ean', 'gtin', 'sku', 'mercadoria', 'prod', 'plu'],
  ['venda', 'vendido', 'vendida', 'sale', 'cupom', 'coupon', 'pdv', 'nfc', 'nfce', 'receipt', 'pedido', 'order'],
  ['estoque', 'stock', 'inventory', 'inventario'],
  ['loja', 'filial', 'store', 'branch', 'shop'],
  ['cliente', 'customer', 'client', 'consumidor', 'fornecedor', 'fornecedore', 'supplier', 'vendor'],
  ['preco', 'price', 'custo', 'cost'],
  ['caixa', 'operador', 'operadore', 'finalizadora', 'pagamento', 'payment', 'cashier', 'tender', 'checkout']
].map(g => new Set(g));
const VENDA = 1;
// Pedaços de 4+ letras procurados DENTRO do token, para nomes colados (TBVENDAITEM, PRECOVENDA, CODPROD
// vira cod+prod, que já bate inteiro). Só aumenta os grupos achados, ou seja, só empurra para "parece". Nada genérico demais aqui: 'prod' pegaria produção e
// produtividade, 'finaliz' pegaria qualquer status finalizado.
const PEDACOS = [
  ['produt'],
  ['vend', 'cupom', 'pedid', 'order'],
  ['estoq'],
  ['loja', 'filial'],
  ['client', 'fornec'],
  ['preco'],
  ['caixa', 'pagament']
];
// "nota fiscal" só vale junto, no mesmo texto: "nota" sozinha é ambígua.
function sinaisDe(texto){
  const tk = new Set(Cat.tokens(texto, true));
  if(tk.has('nota') && tk.has('fiscal')) tk.add('nfc');
  return tk;
}
function bate(tk, i){
  for(const x of tk){
    if(GRUPOS[i].has(x)) return true;
    if(PEDACOS[i].some(p => x.includes(p))) return true;
  }
  return false;
}

// Conservador de propósito: avisar um varejo de verdade que ele não é varejo é o pior erro.
// Só diz que não parece com no máximo 1 grupo e nenhuma tabela de movimento (ou itens) com sinal de venda.
// Sem descrição da IA em metade ou mais das tabelas que deviam ter, não há evidência (nomes em código
// só se revelam pela descrição): não avisa, e devolve semEvidencia:true para quem chamou não guardar
// isso como veredito final (um Atualizar que descreva as tabelas verifica de novo).
function pareceVarejo(catalog){
  const tabs = tabelasDe(catalog);
  if(!tabs.length) return { parece:true, grupos:0, semEvidencia:true };
  const achados = new Set();
  let movVenda = false, pedem = 0, semDesc = 0;
  for(const t of tabs){
    const p = t.perfil || {};
    if(PEDEM_DESCRICAO.has(p.papel)){ pedem++; if(!temDescricao(p)) semDesc++; }
    const textos = [t.name, ...camposDe(t), typeof p.descricao === 'string' ? p.descricao : ''];
    const tk = new Set();
    for(const s of textos) for(const x of sinaisDe(s)) tk.add(x);
    GRUPOS.forEach((g, i) => { if(bate(tk, i)) achados.add(i); });
    if((p.papel === 'movimento' || p.papel === 'itens') && bate(tk, VENDA)) movVenda = true;
  }
  const grupos = achados.size;
  if(pedem > 0 && semDesc * 2 >= pedem) return { parece:true, grupos, semEvidencia:true };
  return { parece:!(grupos <= 1 && !movVenda), grupos, semEvidencia:false };
}

const api = { TEXTO_VAREJO, capturar, aplicar, comparar, limparNotas, mesclar, pareceVarejo };
if(isNode) module.exports = api; else root.CD2Entendimento = api;
})(typeof window !== 'undefined' ? window : globalThis);
