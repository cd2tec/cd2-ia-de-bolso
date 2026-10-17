/* A cena da janelinha em pixel art: o pen drive, o robô andando com o bloco e o cérebro enchendo.
   Só monta SVG em texto; quem anima é o tela.js (passo a passo) e o estilo.css. Os quadros são cópia
   de core/robo.txt e o teste confere que continuam iguais. */
(function(root){
"use strict";

const QUADROS = {
  pen:['..gggg..', '..gkkg..', '.bbbbbb.', '.bbbbbb.', '.bwwwwb.', '.bbbbbb.', '.bbbbbb.', '.bbbbbb.'],
  roboA:['..ggg..', '.ggggg.', '.gcgcg.', '.ggggg.', '..bbb..', '.bbbbb.', 'b.bbb.b', '..bbb..', '.g...g.', '.g...g.'],
  roboB:['..ggg..', '.ggggg.', '.gcgcg.', '.ggggg.', '..bbb..', '.bbbbb.', 'b.bbb.b', '..bbb..', '..g.g..', '..g.g..'],
  bloco:['yyy', 'yyy'],
  cerebro:['..pppppp..', '.piiiiiip.', 'piiiiiiiip', 'piiiiiiiip', 'piiiiiiiip', 'piiiiiiiip', '.piiiiiip.', '..pppppp..'],
  sono:['..........', '..........', '..........', '..kk..kk..'],
  acordado:['..........', '..........', '..ww..ww..', '..wk..wk..', '..k....k..', '...kkkk...']
};
// Rosto de quem espia: fora do QUADROS porque o robo.txt o chama de [espia] só para o robô do terminal.
const ESPIA = ['..........', '..........', '......ww..', '..kk..wk..', '..........', '....kk....'];

const COR = { g:'#a1a6ae', p:'#a1a6ae', k:'#0d1233', w:'#ffffff', b:'#0000ff', c:'#dbff2a', y:'#dbff2a' };
const CHEIO = '#dbff2a', VAZIO = '#1b2050';
const LINHAS_CEREBRO = 6; // o miolo do cérebro tem 6 linhas de "i"

function pixels(linhas, dx, dy, corI){
  let s = '';
  linhas.forEach((l, y) => [...l].forEach((ch, x) => {
    const cor = ch === 'i' ? corI(y) : COR[ch];
    if(cor) s += '<rect x="' + (x + dx) + '" y="' + (y + dy) + '" width="1" height="1" fill="' + cor + '"/>';
  }));
  return s;
}
const semI = () => CHEIO;

// "cheio" é quantas das 6 linhas do miolo estão acesas, de baixo para cima (0 a 6).
function miolo(cheio){
  const n = Math.max(0, Math.min(LINHAS_CEREBRO, Math.round(cheio)));
  return y => (y >= 1 && y <= LINHAS_CEREBRO && (LINHAS_CEREBRO + 1 - y) <= n) ? CHEIO : VAZIO;
}

function rostoLinhas(rosto){
  return rosto === 'acordado' ? QUADROS.acordado : rosto === 'espia' ? ESPIA : QUADROS.sono;
}

function svgCerebro(rosto, cheio){
  const n = cheio === undefined ? LINHAS_CEREBRO : cheio;
  return '<svg viewBox="0 0 10 8" shape-rendering="crispEdges" aria-hidden="true">'
    + pixels(QUADROS.cerebro, 0, 0, miolo(n))
    + '<g class="rosto ' + (rosto || 'sono') + '">' + pixels(rostoLinhas(rosto), 0, 0, semI) + '</g></svg>';
}

const PASSOS = 10; // quantos passos o robô dá do pen drive até o cérebro

// pct de 0 a 100; passo é um contador que só cresce: troca a perna e avança o robô.
function svgCarregando(pct, passo){
  const p = Math.max(0, Math.min(100, Number(pct) || 0));
  const k = Math.max(0, Math.floor(passo) || 0);
  const perna = k % 2 ? QUADROS.roboB : QUADROS.roboA;
  const x = 9 + Math.round((k % PASSOS) * 11 / PASSOS); // de 9 a 19: o bloco termina antes do cérebro
  const cheio = Math.round(p / 100 * LINHAS_CEREBRO);
  // Uma só cena em coordenadas de pixel: pen (0..8), faixa do robô (8..32), cérebro (32..42).
  return '<svg viewBox="0 0 42 10" shape-rendering="crispEdges" aria-hidden="true">'
    + pixels(QUADROS.pen, 0, 2, semI)
    + '<g class="robo">' + pixels(perna, x, 0, semI) + pixels(QUADROS.bloco, x + 8, 5, semI) + '</g>'
    + '<g transform="translate(32 1)">'
    + pixels(QUADROS.cerebro, 0, 0, miolo(cheio)) + pixels(QUADROS.sono, 0, 0, semI) + '</g></svg>';
}

const api = { QUADROS, ESPIA, svgCarregando, svgCerebro };
if(typeof module !== 'undefined' && module.exports) module.exports = api; else root.CD2Cena = api;
})(typeof window !== 'undefined' ? window : globalThis);
