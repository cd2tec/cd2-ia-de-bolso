/* Os dois bonecos da IA de Bolso, no mesmo pixel art do app (web/index.html): o mascote, um robozinho
   nerd saindo do bolso, e o robô do terminal (core/robo.txt), que anda carregando um bloco amarelo.
   Só monta SVG em texto; quem anima é o estilo.css. Os pixels são cópia dos do app, e o teste confere
   que continuam iguais. */
(function(root){
"use strict";

// Mascote: g cinza, k armação dos óculos, w lente, y antena, b bolso de jeans, s costura.
const MASCOTE = {
  rob:['.....y.....', '.....g.....', '...ggggg...', '..ggggggg..', '..kwkgkwk..', '..ggggggg..', '..gg...gg..', '...ggggg...'],
  bolso:['bbbbbbbbbbb', 'bsbsbsbsbsb', 'bbbbbbbbbbb', '.bbbbbbbbb.', '..bbbbbbb..']
};
const MASCOTE_COR = { g:'#a1a6ae', k:'#0d1233', w:'#ffffff', y:'#dbff2a', b:'#0000ff', s:'#dbff2a' };

// Robô do terminal em dois quadros (pernas abertas e fechadas) e o bloco que ele carrega.
const ROBO = {
  q1:['..ggg..', '.ggggg.', '.gcgcg.', '.ggggg.', '..bbb..', '.bbbbb.', 'b.bbb.b', '..bbb..', '.g...g.', '.g...g.'],
  q2:['..ggg..', '.ggggg.', '.gcgcg.', '.ggggg.', '..bbb..', '.bbbbb.', 'b.bbb.b', '..bbb..', '..g.g..', '..g.g..'],
  bloco:['yyy', 'yyy']
};
const ROBO_COR = { g:'#a1a6ae', b:'#0000ff', c:'#dbff2a', y:'#dbff2a' };

const PIADAS = [
  'Cabe uma IA inteira aqui. E ainda sobra espaço para as fotos do churrasco.',
  'Pen drive bom é pen drive que volta para o bolso.',
  'Já tentou desligar e ligar de novo? Eu já. Doeu.',
  'Funciona na minha máquina. Literalmente: eu sou a sua máquina.',
  'Rodo com 4 bilhões de parâmetros e zero nuvens.',
  'Não é bug, é feature não documentada.',
  'Meu café? Fica em cache.',
  'Existem 10 tipos de pessoas: as que entendem binário e as que não.'
];

function pixels(linhas, cores, dx, dy, marcar){
  let s = '';
  linhas.forEach((l, y) => [...l].forEach((ch, x) => {
    if(!cores[ch]) return;
    const c = marcar ? marcar(ch, y) : '';
    s += '<rect x="' + (x + dx) + '" y="' + (y + dy) + '" width="1" height="1" fill="' + cores[ch] + '"' + c + '/>';
  }));
  return s;
}

// A lente pisca e a ponta da antena acende e apaga (classes que o estilo anima).
const marcarMascote = (ch, y) => ch === 'w' ? ' class="lente"' : (ch === 'y' && y === 0) ? ' class="ponta"' : '';

// O robô fica atrás do bolso: quando ele afunda, o bolso esconde a parte de baixo.
function mascoteSvg(){
  return '<svg viewBox="0 -2 11 15" shape-rendering="crispEdges" aria-hidden="true">'
    + '<g class="rob">' + pixels(MASCOTE.rob, MASCOTE_COR, 0, 0, marcarMascote) + '</g>'
    + '<g class="bolso">' + pixels(MASCOTE.bolso, MASCOTE_COR, 0, 8) + '</g></svg>';
}

function roboSvg(){
  return '<svg viewBox="0 0 12 10" shape-rendering="crispEdges" aria-hidden="true">'
    + '<g class="q1">' + pixels(ROBO.q1, ROBO_COR, 0, 0) + '</g>'
    + '<g class="q2">' + pixels(ROBO.q2, ROBO_COR, 0, 0) + '</g>'
    + '<g class="bloco">' + pixels(ROBO.bloco, ROBO_COR, 8, 5) + '</g></svg>';
}

// Sorteia uma piada diferente da última.
function piada(ultima, sorteio){
  const r = sorteio || Math.random;
  let i;
  do { i = Math.floor(r() * PIADAS.length); } while(PIADAS.length > 1 && i === ultima);
  return i;
}

const api = { MASCOTE, MASCOTE_COR, ROBO, ROBO_COR, PIADAS, mascoteSvg, roboSvg, piada };
if(typeof module !== 'undefined' && module.exports) module.exports = api; else root.CD2Mascote = api;
})(typeof window !== 'undefined' ? window : globalThis);
