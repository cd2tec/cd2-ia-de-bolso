/* Consultas salvas da IA de Bolso: o SQL que o usuário aprovou volta a rodar direto no banco, sem o
   modelo, com parâmetros opcionais. Aqui só a lógica pura, sem DOM: achar os literais do SQL, virar
   um trecho em parâmetro e montar o SQL final com os valores digitados.
   Segurança: o valor digitado nunca fecha a string. Texto troca ' por '' e recusa barra invertida
   (MySQL/MariaDB tratam \' como aspa escapada), ponto e vírgula, marcas de comentário e caracteres de controle
   (a guarda do toolbox tira comentários sem olhar aspas e recusa ; em qualquer lugar). Número só
   dígitos (com decimal opcional), data só AAAA-MM-DD que exista no calendário. % e _ de um LIKE NÃO são
   escapados: quem digita % quer o curinga, e escapar exigiria ESCAPE, que muda de motor para motor.
   Data e hora ('AAAA-MM-DD HH:MM:SS'): o literal inteiro vira parâmetro datahora (aceita também o AAAA-MM-DDTHH:MM do
   datetime-local e normaliza). Consulta antiga com tipo data sobre esse literal cobre só os 10 primeiros caracteres.
   Cada parâmetro é conferido de novo contra o SQL na hora de montar (vale para o que veio importado). */
(function(root){
"use strict";
const isNode = typeof module !== 'undefined' && module.exports;

const TIPOS = ['data', 'datahora', 'numero', 'texto'];
const ERRO_TRECHO = 'Selecione um valor dentro de aspas ou um número inteiro.';
const ERRO_VALOR = 'Esse valor não aparece na consulta.';
const DATA_RE = /^\d{4}-\d{2}-\d{2}$/;
const DATAHORA_RE = /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/;
const DATAHORA_ENTRADA_RE = /^(\d{4}-\d{2}-\d{2})[ T](\d{2}:\d{2})(:\d{2})?$/;
const NUM_RE = /^\d+(\.\d+)?$/;
// "1.000" e "10,000" podem ser mil ou um/dez: separador de milhar é recusado, nunca adivinhado.
const MILHAR_RE = /^\d{1,3}(\.\d{3})+$|^\d{1,3}(,\d{3})+$|^\d.*\..*,|^\d.*,.*\./;
const TEXTO_RUIM = /[\u0000-\u001f\u007f\\;]|--|\/\*|\*\//;
const ANTES_NUM = /[A-Za-z0-9_$#.@]/;

// Varredura única com estados: fora, string '…' (com ''), identificador "…"/[…]/`…`, comentários.
// Para (parou = posição) no que não dá para ler igual em todo motor: barra invertida dentro de aspas,
// $$ do PostgreSQL, comentário de bloco aninhado, # com aspas na linha (comentário no MySQL) e aspas
// sem fechar. Tudo dali em diante fica fora: nem literal, nem número.
function varrer(sql){
  const s = String(sql), n = s.length, strings = [], numeros = [];
  let i = 0, parou = null;
  while(i < n){
    const c = s[i], d = s[i + 1];
    if(c === '-' && d === '-'){ const e = s.indexOf('\n', i); i = e < 0 ? n : e + 1; continue; }
    if(c === '/' && d === '*'){
      const e = s.indexOf('*/', i + 2), aninhado = s.indexOf('/*', i + 2);
      if(e < 0 || (aninhado >= 0 && aninhado < e)){ parou = i; break; }
      i = e + 2; continue;
    }
    if(c === "'"){
      let j = i + 1, fechou = false, ruim = false; const pares = [];
      while(j < n){
        if(s[j] === '\\'){ ruim = true; break; }
        if(s[j] === "'"){ if(s[j + 1] === "'"){ pares.push(j); j += 2; continue; } fechou = true; break; }
        j++;
      }
      if(ruim || !fechou){ parou = i; break; }
      strings.push({ abre:i, ini:i + 1, fim:j, pares }); i = j + 1; continue;
    }
    if(c === '"' || c === '`' || c === '['){
      const fecha = c === '[' ? ']' : c;
      let j = i + 1, fechou = false, ruim = false;
      while(j < n){
        if(c === '"' && s[j] === '\\'){ ruim = true; break; }
        if(s[j] === fecha){ if(s[j + 1] === fecha){ j += 2; continue; } fechou = true; break; }
        j++;
      }
      if(ruim || !fechou){ parou = i; break; }
      i = j + 1; continue;
    }
    if(c === '$' && (i === 0 || !/[A-Za-z0-9_]/.test(s[i - 1])) && /^\$([A-Za-z_]\w*)?\$/.test(s.slice(i, i + 64))){ parou = i; break; }
    if(c === '#'){ const e = s.indexOf('\n', i); if(/['"`]/.test(s.slice(i, e < 0 ? n : e))){ parou = i; break; } }
    if(c >= '0' && c <= '9' && (i === 0 || !ANTES_NUM.test(s[i - 1]))){
      const tok = /^\d+(\.\d+)?[A-Za-z0-9_$#]*/.exec(s.slice(i))[0];
      if(NUM_RE.test(tok)) numeros.push({ ini:i, fim:i + tok.length });
      i += tok.length; continue;
    }
    i++;
  }
  return { strings, numeros, parou };
}
const comComentario = (v, s) => v.strings.filter(L => /--|\/\*|\*\//.test(s.slice(L.ini, L.fim))).length;
const desescapar = t => t.replace(/''/g, "'");
function dataValida(v){
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(v);
  if(!m) return false;
  const a = +m[1], me = +m[2], di = +m[3];
  const d = new Date(Date.UTC(2000, me - 1, di)); d.setUTCFullYear(a);
  return a >= 1 && d.getUTCFullYear() === a && d.getUTCMonth() === me - 1 && d.getUTCDate() === di;
}

// Aceita 'AAAA-MM-DD HH:MM:SS' e o 'AAAA-MM-DDTHH:MM(:SS)?' do datetime-local; devolve o normalizado ou null.
function dataHoraValida(v){
  const m = DATAHORA_ENTRADA_RE.exec(v);
  if(!m || !dataValida(m[1])) return null;
  const t = m[2] + (m[3] || ':00');
  return /^([01]\d|2[0-3]):[0-5]\d:[0-5]\d$/.test(t) ? m[1] + ' ' + t : null;
}

function acharLiterais(sql){
  const s = String(sql == null ? '' : sql), v = varrer(s), out = [];
  for(const L of v.strings){
    const conteudo = s.slice(L.ini, L.fim);
    if(DATAHORA_RE.test(conteudo)){ out.push({ valor:conteudo, tipo:'datahora', inicio:L.ini, fim:L.fim, marcado:true }); continue; }
    if(DATA_RE.test(conteudo)){ out.push({ valor:conteudo, tipo:'data', inicio:L.ini, fim:L.ini + 10, marcado:true }); continue; }
    let a = L.ini, b = L.fim;
    while(a < b && s[a] === '%') a++;
    while(b > a && s[b - 1] === '%') b--;
    if(a < b) out.push({ valor:desescapar(s.slice(a, b)), tipo:'texto', inicio:a, fim:b, marcado:false });
  }
  for(const N of v.numeros) out.push({ valor:s.slice(N.ini, N.fim), tipo:'numero', inicio:N.ini, fim:N.fim, marcado:false });
  return out.sort((x, y) => x.inicio - y.inicio);
}

// Onde cai o trecho [a, b): número inteiro do varrer, ou dentro do conteúdo de uma string sem cortar um ''.
const cortaPar = (L, a, b) => L.pares.some(p => a === p + 1 || b === p + 1);
const stringDe = (v, a, b) => v.strings.find(L => a >= L.ini && b <= L.fim) || null;
const ehNumero = (v, a, b) => v.numeros.some(N => N.ini === a && N.fim === b);

function paramDeTrecho(sql, inicio, fim){
  const falha = { ok:false, erro:ERRO_TRECHO };
  if(typeof sql !== 'string' || !Number.isInteger(inicio) || !Number.isInteger(fim)) return falha;
  const s = sql, v = varrer(s);
  let a = Math.max(0, inicio), b = Math.min(s.length, fim);
  while(a < b && /\s/.test(s[a])) a++;
  while(b > a && /\s/.test(s[b - 1])) b--;
  if(a >= b) return falha;
  const inteira = v.strings.find(L => a === L.abre && b === L.fim + 1);
  if(inteira){ a = inteira.ini; b = inteira.fim; if(a >= b) return falha; }
  if(ehNumero(v, a, b)) return { ok:true, param:{ valor:s.slice(a, b), tipo:'numero', inicio:a, fim:b } };
  const L = stringDe(v, a, b);
  if(!L || cortaPar(L, a, b)) return falha;
  const t = s.slice(a, b);
  if(DATAHORA_RE.test(t)) return { ok:true, param:{ valor:t, tipo:'datahora', inicio:a, fim:b } };
  if(DATA_RE.test(t)) return { ok:true, param:{ valor:t, tipo:'data', inicio:a, fim:a + 10 } };
  return { ok:true, param:{ valor:desescapar(t), tipo:'texto', inicio:a, fim:b } };
}

function paramDeValor(sql, valor){
  const falha = { ok:false, erro:ERRO_VALOR };
  const alvo = String(valor == null ? '' : valor).trim();
  if(typeof sql !== 'string' || !alvo) return falha;
  // Primeiro um literal inteiro com esse valor; senão, a primeira ocorrência que seja um trecho válido.
  const lit = acharLiterais(sql).find(l => l.valor === alvo);
  if(lit) return { ok:true, param:{ valor:lit.valor, tipo:lit.tipo, inicio:lit.inicio, fim:lit.fim } };
  const formas = [...new Set([alvo, alvo.replace(/'/g, "''")])], achados = [];
  for(const f of formas) for(let i = sql.indexOf(f); i >= 0; i = sql.indexOf(f, i + 1)) achados.push([i, i + f.length]);
  achados.sort((x, y) => x[0] - y[0]);
  for(const [a, b] of achados){
    const r = paramDeTrecho(sql, a, b);
    if(r.ok && r.param.inicio === a && r.param.fim === b && r.param.valor === alvo) return r;
  }
  return falha;
}

function montarSql(consulta, valores){
  if(!consulta || typeof consulta !== 'object' || typeof consulta.sql !== 'string' || !consulta.sql.trim()) return { ok:false, erro:'Consulta sem SQL.' };
  const sql = consulta.sql, v = varrer(sql);
  const params = Array.isArray(consulta.params) ? consulta.params : [];
  const vs = Array.isArray(valores) ? valores : [];
  const itens = [], normal = [];
  for(let i = 0; i < params.length; i++){
    const p = params[i];
    const nome = (p && typeof p.nome === 'string' && p.nome.trim()) || ('Parâmetro ' + (i + 1));
    const invalido = { ok:false, erro:nome + ': parâmetro inválido.' };
    if(!p || typeof p !== 'object' || !TIPOS.includes(p.tipo) || !Number.isInteger(p.inicio) || !Number.isInteger(p.fim)
      || p.inicio < 0 || p.fim > sql.length || p.inicio >= p.fim) return invalido;
    if(p.tipo === 'numero'){ if(!ehNumero(v, p.inicio, p.fim)) return invalido; }
    else { const L = stringDe(v, p.inicio, p.fim); if(!L || cortaPar(L, p.inicio, p.fim)) return invalido; }
    const bruto = vs[i] == null ? p.valor : vs[i];
    const valor = String(bruto == null ? '' : bruto).trim();
    let texto;
    if(p.tipo === 'data'){ if(!dataValida(valor)) return { ok:false, erro:nome + ': data inválida.' }; texto = valor; }
    else if(p.tipo === 'datahora'){ const dh = dataHoraValida(valor); if(!dh) return { ok:false, erro:nome + ': data e hora inválida.' }; texto = dh; normal[i] = dh; }
    else if(p.tipo === 'numero'){
      // O valor salvo (com ponto, ou com vírgula como a janela Rodar mostra) já é o que rodou: não é ambíguo.
      const salvo = String(p.valor == null ? '' : p.valor);
      if(valor !== salvo && valor !== salvo.replace('.', ',') && MILHAR_RE.test(valor)) return { ok:false, erro:nome + ': use vírgula para decimal, sem separador de milhar.' };
      const t = valor.replace(',', '.'); if(!NUM_RE.test(t)) return { ok:false, erro:nome + ': número inválido.' }; texto = t;
      normal[i] = t;
    }
    else { if(TEXTO_RUIM.test(valor)) return { ok:false, erro:nome + ': texto inválido.' }; texto = valor.replace(/'/g, "''"); }
    if(normal[i] == null) normal[i] = valor;
    itens.push({ inicio:p.inicio, fim:p.fim, texto, invalido });
  }
  const ordem = itens.slice().sort((x, y) => x.inicio - y.inicio);
  for(let k = 1; k < ordem.length; k++) if(ordem[k].inicio < ordem[k - 1].fim) return ordem[k].invalido;
  // Do último para o primeiro, para as posições dos anteriores continuarem valendo.
  let out = sql;
  for(let k = ordem.length - 1; k >= 0; k--) out = out.slice(0, ordem[k].inicio) + ordem[k].texto + out.slice(ordem[k].fim);
  // Conferência final: mesma estrutura de literais e nenhum valor encostado formando comentário.
  const w = varrer(out);
  if(w.strings.length !== v.strings.length || w.numeros.length !== v.numeros.length || (w.parou === null) !== (v.parou === null)
    || comComentario(w, out) > comComentario(v, sql)) return { ok:false, erro:'Esses valores quebrariam a consulta.' };
  // valores: o que de fato entrou no SQL (número já com ponto), para o card e para a próxima vez.
  return { ok:true, sql:out, valores:normal };
}

const api = { TIPOS, acharLiterais, paramDeTrecho, paramDeValor, montarSql };
if(isNode) module.exports = api; else root.CD2Consultas = api;
})(typeof window !== 'undefined' ? window : globalThis);
