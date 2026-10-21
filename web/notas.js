/* Notas da TI da IA de Bolso: texto livre por tabela e por coluna, guardado pelo cd2-ctl como um
   documento por banco. Aqui só a lógica pura, sem DOM: aplicar as notas no catálogo, cortar para
   caber no prompt, exportar/ler arquivo e montar o plano de importação (junta, sem apagar nada). */
(function(root){
"use strict";
const isNode = typeof module !== 'undefined' && module.exports;
const Cat = isNode ? require('./catalog.js') : root.CD2Catalog;

const LIMITE = { tabela:300, campo:120, prompt:2000 };
const FORMATO = 'ia-de-bolso-notas';
const ERRO_ARQUIVO = 'Este arquivo não é uma exportação da IA de Bolso.';

const vazio = () => ({ versao:1, tabelas:{}, consultas:[] });
function limpar(texto, max){
  const s = String(texto == null ? '' : texto).replace(/[\u0000-\u001f\u007f]+/g, ' ').replace(/\s+/g, ' ').trim();
  return s.slice(0, max);
}
const txt = v => (typeof v === 'string' ? v.trim() : '');
const tabelasDe = d => (d && d.tabelas && typeof d.tabelas === 'object') ? d.tabelas : {};
const consultasDe = d => (d && Array.isArray(d.consultas)) ? d.consultas : [];

function aplicar(catalog, dados){
  const tabelas = tabelasDe(dados);
  for(const t of catalog.tables){
    delete t.nota; delete t.notaTokens;
    for(const c of t.columns) delete c.nota;
    const n = Object.prototype.hasOwnProperty.call(tabelas, t.name) ? tabelas[t.name] : null;
    if(!n) continue;
    const nota = txt(n.nota);
    if(nota){ t.nota = nota; t.notaTokens = Cat.tokens(nota, true); }
    const campos = n.campos || {};
    for(const c of t.columns){
      const v = Object.prototype.hasOwnProperty.call(campos, c.name) ? txt(campos[c.name]) : '';
      if(v) c.nota = v;
    }
  }
  return catalog;
}

function contar(dados){
  return { tabelasComNota: Object.values(tabelasDe(dados)).filter(n => n && txt(n.nota)).length };
}

// Soma o que a TI escreveu: nota da tabela, notas de campo e descrição da IA das tabelas com nota.
function tamanho(ts){
  let s = 0;
  for(const t of ts){
    if(t.nota){ s += t.nota.length; if(t.perfil && t.perfil.descricao) s += t.perfil.descricao.length; }
    for(const c of t.columns || []) if(c.nota) s += c.nota.length;
  }
  return s;
}
// Corte, da tabela menos relevante para a mais: 1) notas de campo (da última coluna para a primeira),
// 2) descrição da IA de tabela com nota. A nota da tabela nunca sai. Não muta a entrada.
function cortar(tabelas, max){
  const out = tabelas.map(t => ({ ...t, columns:(t.columns || []).map(c => ({ ...c })), perfil:t.perfil ? { ...t.perfil } : t.perfil }));
  let soma = tamanho(out);
  for(let i = out.length - 1; i >= 0 && soma > max; i--){
    const cols = out[i].columns;
    for(let j = cols.length - 1; j >= 0 && soma > max; j--){
      if(cols[j].nota){ soma -= cols[j].nota.length; delete cols[j].nota; }
    }
  }
  for(let i = out.length - 1; i >= 0 && soma > max; i--){
    const t = out[i];
    if(t.nota && t.perfil && t.perfil.descricao){ soma -= t.perfil.descricao.length; delete t.perfil.descricao; }
  }
  return out;
}

function enxuto(dados){
  const tabelas = {};
  for(const [nome, n] of Object.entries(tabelasDe(dados))){
    if(!n) continue;
    const nota = txt(n.nota), campos = {};
    for(const [c, v] of Object.entries(n.campos || {})) if(txt(v)) campos[c] = txt(v);
    if(nota || Object.keys(campos).length) tabelas[nome] = { nota, campos };
  }
  return { versao:1, tabelas, consultas:consultasDe(dados).slice() };
}

function exportar(dados, banco, agora){
  const d = agora || new Date();
  const base = String((banco && banco.base) || 'banco').replace(/[^A-Za-z0-9._-]+/g, '_');
  const conteudo = JSON.stringify({ formato:FORMATO, versao:1, exportado:d.toISOString(), banco, dados:enxuto(dados) }, null, 2);
  return { nome:`ia-de-bolso-notas-${base}-${d.toISOString().slice(0, 10)}.json`, conteudo };
}

function lerArquivo(texto){
  let j;
  try { j = JSON.parse(texto); } catch(e){ return { ok:false, erro:ERRO_ARQUIVO }; }
  if(!j || j.formato !== FORMATO || !j.dados || typeof j.dados !== 'object') return { ok:false, erro:ERRO_ARQUIVO };
  return { ok:true, banco:j.banco || null, dados:enxuto(j.dados) };
}

function mesmoBanco(a, b){
  if(!a || !b) return false;
  const h = x => String(x.host == null ? '' : x.host).toLowerCase();
  const s = x => String(x == null ? '' : x);
  return s(a.motor) === s(b.motor) && h(a) === h(b) && s(a.porta) === s(b.porta) && s(a.base) === s(b.base) && s(a.schema) === s(b.schema);
}

const chaveConsulta = c => String(c.nome == null ? '' : c.nome).trim().toLowerCase();

// Junta o que chega com o que existe. Só há conflito quando os dois lados têm texto e ele difere;
// nesse caso o resultado fica com o que chega. Não muta as entradas.
function planoImportacao(atual, chegando){
  const res = enxuto(atual);
  const tabelas = {};
  for(const [k, v] of Object.entries(res.tabelas)) tabelas[k] = { nota:v.nota, campos:{ ...v.campos } };
  const conflitos = [];
  const novas = { notas:0, consultas:0 };
  const cheg = enxuto(chegando);
  for(const [nome, n] of Object.entries(cheg.tabelas)){
    const cur = tabelas[nome] || (tabelas[nome] = { nota:'', campos:{} });
    if(n.nota){
      if(!cur.nota) { cur.nota = n.nota; novas.notas++; }
      else if(cur.nota !== n.nota){ conflitos.push({ tipo:'tabela', onde:nome, antes:cur.nota, depois:n.nota }); cur.nota = n.nota; }
    }
    for(const [c, v] of Object.entries(n.campos)){
      if(!cur.campos[c]){ cur.campos[c] = v; novas.notas++; }
      else if(cur.campos[c] !== v){ conflitos.push({ tipo:'campo', onde:`${nome}.${c}`, antes:cur.campos[c], depois:v }); cur.campos[c] = v; }
    }
  }
  const consultas = res.consultas.map(c => ({ ...c }));
  for(const c of cheg.consultas){
    const i = consultas.findIndex(x => chaveConsulta(x) === chaveConsulta(c));
    if(i < 0){
      let id = c.id;
      if(id == null || consultas.some(x => x.id === id)) id = `${id == null ? 'q' : id}-${consultas.length + 1}`;
      consultas.push({ ...c, id });
      novas.consultas++;
    } else if(consultas[i].sql !== c.sql){
      conflitos.push({ tipo:'consulta', onde:String(consultas[i].nome), antes:consultas[i].sql, depois:c.sql });
      consultas[i] = { ...c, id:consultas[i].id };
    }
  }
  return { novas, conflitos, resultado:{ versao:1, tabelas, consultas } };
}

const api = { LIMITE, vazio, limpar, aplicar, contar, cortar, exportar, lerArquivo, mesmoBanco, planoImportacao };
if(isNode) module.exports = api; else root.CD2Notas = api;
})(typeof window !== 'undefined' ? window : globalThis);
