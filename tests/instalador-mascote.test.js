'use strict';
// Bonecos do instalador (installer/frontend/dist/mascote.js): os pixels têm de ser os mesmos do app,
// para o mascote e o robô não ficarem diferentes entre o instalador e a IA de Bolso. Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs'), path = require('node:path');
const M = require('../installer/frontend/dist/mascote.js');

const app = fs.readFileSync(path.join(__dirname, '..', 'web', 'index.html'), 'utf8');
// Lê "const NOME = {...};" do app e avalia só o literal de dados.
function doApp(nome){
  const m = app.match(new RegExp('const ' + nome + ' = (\\{[\\s\\S]*?\\});'));
  assert.ok(m, nome + ' não encontrado em web/index.html');
  return Function('"use strict"; return (' + m[1] + ');')();
}

test('mascote e robô usam os mesmos pixels e cores do app', () => {
  assert.deepEqual(M.MASCOTE, doApp('MASCOTE'));
  assert.deepEqual(M.MASCOTE_COR, doApp('MASCOTE_COR'));
  assert.deepEqual(M.ROBO, doApp('ROBO'));
  assert.deepEqual(M.ROBO_COR, doApp('ROBO_COR'));
});

test('mascoteSvg: robô atrás do bolso, com lente e ponta da antena marcadas', () => {
  const s = M.mascoteSvg();
  assert.match(s, /^<svg [^>]*aria-hidden="true"/);
  assert.ok(s.indexOf('class="rob"') < s.indexOf('class="bolso"'), 'o bolso vem depois para cobrir o robô');
  assert.equal((s.match(/class="lente"/g) || []).length, 2);
  assert.equal((s.match(/class="ponta"/g) || []).length, 1);
});

test('roboSvg: dois quadros e o bloco', () => {
  const s = M.roboSvg();
  for(const c of ['q1', 'q2', 'bloco']) assert.match(s, new RegExp('class="' + c + '"'));
});

test('piada: nunca repete a última e cai dentro da lista', () => {
  // O sorteio cai primeiro na última (0) e depois na seguinte: tem de pular a repetida.
  const seq = [0, 0.99]; let k = 0;
  assert.equal(M.piada(0, () => seq[k++]), M.PIADAS.length - 1);
  for(let i = 0; i < 50; i++){
    const n = M.piada(3);
    assert.ok(n >= 0 && n < M.PIADAS.length && n !== 3);
  }
});

test('textos das piadas sem travessão e sem emoji', () => {
  for(const p of M.PIADAS){
    assert.doesNotMatch(p, /[–—]/);
    assert.doesNotMatch(p, /\p{Extended_Pictographic}/u);
  }
});
