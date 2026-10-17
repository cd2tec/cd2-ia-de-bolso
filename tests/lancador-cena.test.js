'use strict';
// Cena e piadas da janelinha do lançador (launcher/frontend/dist). Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs'), path = require('node:path');
const dist = path.join(__dirname, '..', 'launcher', 'frontend', 'dist');
const Cena = require(path.join(dist, 'cena.js'));
const Piadas = require(path.join(dist, 'piadas.js'));
const Mascote = require(path.join(dist, 'mascote.js'));

test('mascote.js do lançador é cópia do instalador', () => {
  const a = fs.readFileSync(path.join(dist, 'mascote.js'));
  const b = fs.readFileSync(path.join(__dirname, '..', 'installer', 'frontend', 'dist', 'mascote.js'));
  assert.ok(a.equals(b));
});

// Lê as seções "[nome]" de core/robo.txt (linhas com # são comentário).
function robo(){
  const sec = {}; let atual = null;
  for(const l of fs.readFileSync(path.join(__dirname, '..', 'core', 'robo.txt'), 'utf8').split('\n')){
    if(l.startsWith('#') || !l.trim()) continue;
    const m = l.match(/^\[(\w+)\]$/);
    if(m){ atual = m[1]; sec[atual] = []; } else if(atual) sec[atual].push(l.trim());
  }
  return sec;
}

test('cena usa os pixels de core/robo.txt', () => {
  const r = robo(), esperado = {};
  for(const k of ['pen', 'roboA', 'roboB', 'bloco', 'cerebro', 'sono', 'acordado']) esperado[k] = r[k];
  assert.deepEqual(Cena.QUADROS, esperado);
  assert.deepEqual(Cena.ESPIA, r.espia);
});

const cheios = svg => (svg.match(/fill="#dbff2a"/g) || []).length;
const vazios = svg => (svg.match(/fill="#1b2050"/g) || []).length;

test('svgCerebro: miolo enche conforme o parâmetro e o rosto muda', () => {
  const vazio = Cena.svgCerebro('sono', 0), cheio = Cena.svgCerebro('sono', 6);
  assert.equal(cheios(vazio), 0);
  assert.equal(vazios(cheio), 0);
  assert.ok(vazios(vazio) > 0 && cheios(cheio) > 0);
  assert.match(Cena.svgCerebro('acordado', 6), /class="rosto acordado"/);
  assert.match(Cena.svgCerebro('espia', 6), /class="rosto espia"/);
  assert.match(cheio, /^<svg [^>]*aria-hidden="true"/);
});

test('svgCarregando: progresso enche o cérebro e o passo troca a perna e avança o robô', () => {
  assert.ok(cheios(Cena.svgCarregando(100, 0)) > cheios(Cena.svgCarregando(0, 0)));
  assert.notEqual(Cena.svgCarregando(50, 0), Cena.svgCarregando(50, 1));
  assert.notEqual(Cena.svgCarregando(50, 0), Cena.svgCarregando(50, 2));
  assert.doesNotThrow(() => Cena.svgCarregando(-5, -1));
  assert.doesNotThrow(() => Cena.svgCarregando(900, 1e6));
});

test('piadas: listas exigidas e sem travessão nem emoji', () => {
  assert.ok(Piadas.CHAMADA.length >= 6 && Piadas.DESPEDIDA.length >= 4);
  assert.ok(Piadas.CHAMADA.includes('Tava aqui no cantinho do relógio, contando os segundos. Literalmente.'));
  assert.ok(Piadas.DESPEDIDA.includes('Vou sonhar com planilhas. Até a próxima.'));
  for(const p of [...Piadas.CHAMADA, ...Piadas.DESPEDIDA]){
    assert.doesNotMatch(p, /[–—]/);
    assert.doesNotMatch(p, /\p{Extended_Pictographic}/u);
  }
});

test('proxima não repete', () => {
  const seq = [0, 0.99]; let k = 0;
  assert.equal(Piadas.proxima(Piadas.CHAMADA, 0, () => seq[k++]), Piadas.CHAMADA.length - 1);
  for(let i = 0; i < 50; i++) assert.notEqual(Piadas.proxima(Mascote.PIADAS, 3), 3);
  assert.equal(Piadas.proxima(['só uma'], 0, () => 0), 0);
});
