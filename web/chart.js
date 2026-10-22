/* Gráficos do roteiro de relatórios: barra, linha, pizza, número e tabela, em SVG/HTML feito à
   mão (sem biblioteca, funciona offline). Cores por classe CSS (tokens em index.html); a pizza lê os
   tokens --pizza-N (claro ou escuro) e guarda o hex do claro como reserva fora da página. Todo texto que vem do banco é escapado aqui. Sem animação. */
(function(root){
"use strict";
const PALETA = ['#0000ff', '#011e80', '#019eff', '#0333f7', '#a1a6ae', '#dbff2a'];
const corPizza = i => 'var(--pizza-' + (i % PALETA.length + 1) + ',' + PALETA[i % PALETA.length] + ')';
const MESES = ['jan','fev','mar','abr','mai','jun','jul','ago','set','out','nov','dez'];
const W = 640;
const NF = new Intl.NumberFormat('pt-BR', { maximumFractionDigits:2 });

function esc(v){ return String(v == null ? '' : v).replace(/[&<>"']/g, c => ({ '&':'&amp;', '<':'&lt;', '>':'&gt;', '"':'&quot;', "'":'&#39;' }[c])); }
function num(v){
  if(typeof v === 'number') return isFinite(v) ? v : null;
  if(typeof v === 'string' && /^\s*-?\d+(\.\d+)?([eE][+-]?\d+)?\s*$/.test(v)){ const n = Number(v); return isFinite(n) ? n : null; }
  return null;
}
function fmt(v){ const n = num(v); if(n == null) return String(v == null ? '' : v); return NF.format(Math.abs(n) < 0.005 ? 0 : n); }
// Na tabela, só formata o que é claramente número: inteiro em texto pode ser código (CNPJ, EAN).
function fmtCell(v){ return (typeof v === 'number' || (typeof v === 'string' && /^\s*-?\d+\.\d+\s*$/.test(v))) ? fmt(v) : String(v == null ? '' : v); }
function cut(s, n){ s = String(s); return s.length > n ? s.slice(0, n - 1) + '…' : s; }
function labelsOf(vals){
  const empty = v => v == null || v === '';
  const iso = vals.map(v => empty(v) ? null : /^(\d{4})-(\d{2})(?:-(\d{2}))?(?:[T ]\d{2}:\d{2}(?::\d{2}(?:\.\d+)?)?(?:Z|[+-]\d{2}:?\d{2})?)?$/.exec(String(v)));
  const mensal = iso.some(Boolean) && iso.every((m, i) => empty(vals[i]) || (m && (!m[3] || m[3] === '01')));
  return vals.map((v, i) => {
    if(empty(v)) return '(vazio)';
    const m = iso[i];
    if(m && mensal && MESES[Number(m[2]) - 1]) return MESES[Number(m[2]) - 1] + '/' + m[1];
    if(m && m[3]) return m[3] + '/' + m[2] + '/' + m[1];
    return String(v);
  });
}
const svgOpen = (h, label) => '<svg class="chart" viewBox="0 0 ' + W + ' ' + h + '" role="img" aria-label="' + esc(label) + '">';

function barra(linhas, x, y){
  const vals = linhas.map(r => num(r[y]) || 0), max = Math.max(0, ...vals.map(Math.abs)) || 1;
  const labs = labelsOf(linhas.map(r => r[x])), L = 200, R = 90, H = 28, bw = W - L - R;
  let s = svgOpen(linhas.length * H + 8, 'Gráfico de barras: ' + y + ' por ' + x);
  linhas.forEach((r, i) => {
    const v = vals[i], w = Math.round(Math.abs(v) / max * bw), yy = 4 + i * H;
    s += '<g><title>' + esc(labs[i] + ': ' + fmt(v)) + '</title>'
      + '<text class="lbl" x="' + (L - 8) + '" y="' + (yy + 18) + '" text-anchor="end">' + esc(cut(labs[i], 26)) + '</text>'
      + '<rect class="bar' + (v < 0 ? ' neg' : '') + '" x="' + L + '" y="' + (yy + 4) + '" width="' + w + '" height="' + (H - 8) + '" rx="3"/>'
      + '<text class="val" x="' + (L + w + 6) + '" y="' + (yy + 18) + '">' + esc(fmt(v)) + '</text></g>';
  });
  return s + '</svg>';
}
function linha(linhas, x, y){
  const vals = linhas.map(r => num(r[y]) || 0), labs = labelsOf(linhas.map(r => r[x]));
  const L = 70, R = 20, T = 16, B = 40, h = 280, pw = W - L - R, ph = h - T - B, n = linhas.length;
  const mn = Math.min(0, ...vals), mx = Math.max(0, ...vals), span = (mx - mn) || 1;
  const px = i => (L + (n === 1 ? pw / 2 : i * pw / (n - 1))).toFixed(1);
  const py = v => (T + ph - (v - mn) / span * ph).toFixed(1);
  let s = svgOpen(h, 'Gráfico de linha: ' + y + ' por ' + x);
  s += '<line class="axis" x1="' + L + '" y1="' + py(0) + '" x2="' + (W - R) + '" y2="' + py(0) + '"/>';
  s += '<text class="lbl" x="' + (L - 8) + '" y="' + (T + 4) + '" text-anchor="end">' + esc(fmt(mx)) + '</text>';
  s += '<text class="lbl" x="' + (L - 8) + '" y="' + (T + ph + 4) + '" text-anchor="end">' + esc(fmt(mn)) + '</text>';
  s += '<polyline class="line" points="' + vals.map((v, i) => px(i) + ',' + py(v)).join(' ') + '"/>';
  const step = Math.max(1, Math.ceil(n / 8));
  vals.forEach((v, i) => {
    s += '<circle class="pt" cx="' + px(i) + '" cy="' + py(v) + '" r="4"><title>' + esc(labs[i] + ': ' + fmt(v)) + '</title></circle>';
    // o último rótulo sempre aparece; o anterior some quando ficaria a menos de 80 px dele (em cima)
    if(i === n - 1 || (i % step === 0 && (n - 1 - i) * pw / (n - 1) >= 80)) s += '<text class="lbl" x="' + px(i) + '" y="' + (h - 16) + '" text-anchor="middle">' + esc(cut(labs[i], 12)) + '</text>';
  });
  return s + '</svg>';
}
function pizza(linhas, x, y){
  const vals = linhas.map(r => Math.max(0, num(r[y]) || 0)), tot = vals.reduce((a, b) => a + b, 0);
  const labs = labelsOf(linhas.map(r => r[x])), cx = 140, cy = 140, rr = 120;
  let a0 = -Math.PI / 2;
  let s = '<div class="pie"><svg class="chart" viewBox="0 0 280 280" role="img" aria-label="' + esc('Gráfico de pizza: ' + y + ' por ' + x) + '">';
  vals.forEach((v, i) => {
    const frac = tot ? v / tot : 0, a1 = a0 + frac * 2 * Math.PI, cor = corPizza(i);
    const t = '<title>' + esc(labs[i] + ': ' + fmt(v)) + '</title>';
    const p = a => (cx + rr * Math.cos(a)).toFixed(2) + ' ' + (cy + rr * Math.sin(a)).toFixed(2);
    if(frac >= 0.9999) s += '<circle cx="' + cx + '" cy="' + cy + '" r="' + rr + '" style="fill:' + cor + '">' + t + '</circle>';
    else if(frac > 0) s += '<path d="M' + cx + ' ' + cy + ' L' + p(a0) + ' A' + rr + ' ' + rr + ' 0 ' + (frac > 0.5 ? 1 : 0) + ' 1 ' + p(a1) + ' Z" style="fill:' + cor + '">' + t + '</path>';
    a0 = a1;
  });
  s += '</svg><ul class="legend">';
  vals.forEach((v, i) => { s += '<li><i style="background:' + corPizza(i) + '"></i>' + esc(labs[i]) + ' <b>' + esc(fmt(v)) + '</b></li>'; });
  return s + '</ul></div>';
}
function numero(colunas, linhas, y){
  const r = linhas[0] || {}, c = (y && colunas.includes(y)) ? y : (colunas.find(k => num(r[k]) != null) || colunas[0]);
  return '<div class="kpi"><span class="kpi-v">' + esc(fmt(r[c])) + '</span><span class="kpi-l">' + esc(c) + '</span></div>';
}
function tabela(colunas, linhas){
  const head = colunas.map(c => '<th>' + esc(c) + '</th>').join('');
  const body = linhas.map(r => '<tr>' + colunas.map(c => '<td' + (num(r[c]) != null ? ' class="n"' : '') + '>' + esc(fmtCell(r[c])) + '</td>').join('') + '</tr>').join('');
  return '<div class="tbl"><table><thead><tr>' + head + '</tr></thead><tbody>' + body + '</tbody></table></div>';
}
function render(visual, colunas, linhas, opts){
  opts = opts || {};
  if(visual === 'numero') return numero(colunas, linhas, opts.y);
  if(visual === 'barra') return barra(linhas, opts.x, opts.y);
  if(visual === 'linha') return linha(linhas, opts.x, opts.y);
  if(visual === 'pizza') return pizza(linhas, opts.x, opts.y);
  return tabela(colunas, linhas);
}

const api = { num, fmt, labelsOf, render, esc };
if(typeof module !== 'undefined' && module.exports) module.exports = api; else root.CD2Chart = api;
})(typeof window !== 'undefined' ? window : globalThis);
