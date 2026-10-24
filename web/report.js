/* Saídas do roteiro de relatórios: CSV para o Excel em pt-BR e o HTML do relatório impresso
   (Imprimir/PDF do navegador), com logo e contato da CD2 no rodapé. Nada vai para o pen drive. */
(function(root){
"use strict";
function esc(v){ return String(v == null ? '' : v).replace(/[&<>"']/g, c => ({ '&':'&amp;', '<':'&lt;', '>':'&gt;', '"':'&quot;', "'":'&#39;' }[c])); }
// Decimal com vírgula para o Excel em português. Só valor inteiramente numérico escapa da neutralização;
// texto que pode virar fórmula (inclusive após espaço, tabulação ou CR) ganha apóstrofo.
function cell(v){
  if(v == null) return '';
  let s;
  if(typeof v === 'number') s = String(v).replace('.', ',');
  else if(/^\s*-?\d+([.,]\d+)?\s*$/.test(String(v))) s = String(v).trim().replace('.', ',');
  else {
    s = String(v);
    if(/^[\t\r]/.test(s) || /^\s*[=+\-@]/.test(s)) s = "'" + s;
  }
  return /[;"\r\n]/.test(s) ? '"' + s.replace(/"/g, '""') + '"' : s;
}
function toCsv(colunas, linhas){
  const out = [colunas.map(cell).join(';')];
  for(const r of linhas) out.push(colunas.map(c => cell(r[c])).join(';'));
  return '\uFEFF' + out.join('\r\n') + '\r\n';
}
function fileName(d){
  const p = n => String(n).padStart(2, '0');
  return 'cd2-relatorio-' + d.getFullYear() + p(d.getMonth() + 1) + p(d.getDate()) + '-' + p(d.getHours()) + p(d.getMinutes()) + '.csv';
}
function printHtml(o){
  return '<div class="pr">'
    + '<header class="pr-head">' + (o.logoSvg || '') + '<span>Relatório gerado pela IA de Bolso</span></header>'
    + '<p class="pr-q">Pergunta: ' + esc(o.pergunta) + '</p>'
    + '<h1>' + esc(o.titulo) + '</h1><p>' + esc(o.frase) + '</p>'
    + (o.aviso ? '<p>' + esc(o.aviso) + '</p>' : '')
    + '<div class="pr-viz">' + (o.visualHtml || '') + '</div>'
    + '<h2>Dados</h2>' + (o.tabelaHtml || '')
    + '<h2>Consulta usada</h2><pre>' + esc(o.sql) + '</pre>'
    + '<p class="pr-meta">' + esc(o.quando) + ' · ' + esc(o.motor) + (o.base ? ' · base ' + esc(o.base) : '') + '</p>'
    + '<footer class="pr-foot">' + (o.logoSvg || '') + '<span>CD2 Tech' + (o.contato ? ' · ' + esc(o.contato) : '') + '</span></footer>'
    + '</div>';
}

// O que o harness entende do banco: as tabelas agrupadas pelo papel do perfil, com colunas úteis e a
// descrição do modelo e as notas da TI. As vazias só pelo nome, no fim. Sem as ligações entre tabelas.
const GRUPOS = [['movimento', 'Movimento'], ['itens', 'Itens de movimento'], ['cadastro', 'Cadastro'], ['parametro', 'Parâmetro'], ['indefinida', 'Indefinida']];
function linhaTabela(t){
  const p = t.perfil, c = (p.papel === 'movimento' || p.papel === 'itens') ? (p.colunas || {}) : {};
  const uteis = [c.data ? 'data: ' + c.data : '', c.valor ? 'valor: ' + c.valor : '', c.quantidade ? 'quantidade: ' + c.quantidade : ''].filter(Boolean).join('; ');
  const linhas = n => 'cerca de ' + Number(n).toLocaleString('pt-BR') + (Number(n) === 1 ? ' linha' : ' linhas');
  const campos = (t.columns || []).filter(x => x.nota).map(x => esc(x.name) + ': ' + esc(x.nota)).join('; ');
  const tam = t.type === 'VIEW' ? (t.rows ? 'view, ' + linhas(t.rows) : 'view') : t.semContagem ? '' : linhas(t.rows || 0);
  return '<li><b>' + esc(t.name) + '</b>' + (tam ? ' <span class="pr-meta">(' + esc(tam) + ')</span>' : '')
    + (uteis ? '<br><span class="pr-meta">' + esc(uteis) + '</span>' : '')
    + (p.descricao ? '<br>' + esc(p.descricao) : '')
    + (t.nota ? '<br><span class="pr-nota">Comentário: ' + esc(t.nota) + '</span>' : '')
    + (campos ? '<br><span class="pr-meta">Notas dos campos: ' + campos + '</span>' : '') + '</li>';
}
function perfilHtml(o){
  const tabs = o.tabelas || [], com = tabs.filter(t => t.perfil && t.perfil.papel);
  const cab = '<header class="pr-head">' + (o.logoSvg || '') + '<span>O que a IA de Bolso entende deste banco</span></header>'
    + '<h1>' + esc(o.base || 'Banco de dados') + '</h1>'
    + '<p class="pr-meta">' + esc(o.motor) + (o.schema ? ' · schema ' + esc(o.schema) : '') + ' · ' + esc(o.quando) + ' · ' + tabs.length + ' tabelas</p>';
  const rodape = '<footer class="pr-foot">' + (o.logoSvg || '') + '<span>CD2 Tech' + (o.contato ? ' · ' + esc(o.contato) : '') + '</span></footer>';
  if(!com.length) return '<div class="pr">' + cab + '<p>A análise das tabelas ainda não terminou. Tente de novo em alguns segundos.</p>' + rodape + '</div>';
  const pr = o.progresso;
  const aviso = pr && pr.feitos < pr.total ? '<p>O modelo ainda está descrevendo as tabelas: ' + pr.feitos + ' de ' + pr.total + ' tabelas descritas até agora.</p>' : '';
  const conta = papel => com.filter(t => t.perfil.papel === papel).length;
  const resumo = '<ul class="pr-resumo">' + GRUPOS.concat([['vazia', 'Vazias']]).map(([p, rotulo]) => '<li>' + rotulo + ': ' + conta(p) + '</li>').join('') + '</ul>';
  const grupos = GRUPOS.map(([p, rotulo]) => {
    const lista = com.filter(t => t.perfil.papel === p);
    return lista.length ? '<h2>' + rotulo + ' (' + lista.length + ')</h2><ul class="pr-tabs">' + lista.map(linhaTabela).join('') + '</ul>' : '';
  }).join('');
  const vazias = com.filter(t => t.perfil.papel === 'vazia');
  const fim = vazias.length ? '<h2>Vazias (' + vazias.length + ')</h2><p>' + vazias.map(t => esc(t.name)).join(', ') + '</p>' : '';
  return '<div class="pr">' + cab + aviso + resumo + grupos + fim + rodape + '</div>';
}

const api = { toCsv, fileName, printHtml, perfilHtml };
if(typeof module !== 'undefined' && module.exports) module.exports = api; else root.CD2Report = api;
})(typeof window !== 'undefined' ? window : globalThis);
