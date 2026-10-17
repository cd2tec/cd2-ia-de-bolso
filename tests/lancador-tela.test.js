'use strict';
// Textos e botões de cada fase da janelinha do lançador. Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const dist = path.join(__dirname, '..', 'launcher', 'frontend', 'dist');
const { render } = require(path.join(dist, 'tela.js'));
const Piadas = require(path.join(dist, 'piadas.js'));
const Mascote = require(path.join(dist, 'mascote.js'));

const BOTOES = ['Abrir uma nova janela da conversa', 'Colocar a IA para dormir'];

test('render Lendo', () => {
  const r = render({ fase: 'Lendo', pct: 62 }, 'normal');
  assert.equal(r.linha, 'carregando o modelo de IA ... 62%');
  assert.equal(r.piadaLista, Mascote.PIADAS);
  assert.deepEqual(r.botoes, []);
});

test('render Subindo mostra o mesmo carregamento', () => {
  assert.equal(render({ fase: 'Subindo', pct: 80 }).linha, 'carregando o modelo de IA ... 80%');
});

test('render Pronta normal', () => {
  const r = render({ fase: 'Pronta' }, 'normal');
  assert.equal(r.titulo, 'Acordei! A conversa abriu no navegador.');
  assert.deepEqual(r.botoes, BOTOES);
  assert.equal(r.linha, '');
});

test('render Pronta chamada', () => {
  const r = render({ fase: 'Pronta' }, 'chamada');
  assert.equal(r.titulo, 'Me chamou?');
  assert.equal(r.piadaLista, Piadas.CHAMADA);
  assert.deepEqual(r.botoes, BOTOES);
});

test('render Pronta com aviso: aviso vira linha e os botões não mudam', () => {
  const r = render({ fase: 'Pronta', aviso: 'O banco não subiu.' }, 'normal');
  assert.equal(r.linha, 'O banco não subiu.');
  assert.deepEqual(r.botoes, BOTOES);
});

test('render Lendo em chamada mostra o carregamento', () => {
  assert.deepEqual(render({ fase: 'Lendo', pct: 10 }, 'chamada'), render({ fase: 'Lendo', pct: 10 }, 'normal'));
});

test('render Dormindo', () => {
  const r = render({ fase: 'Dormindo' }, 'normal');
  assert.equal(r.titulo, 'Colocando a IA para dormir...');
  assert.deepEqual(r.botoes, []);
});

test('render Dormiu', () => {
  const r = render({ fase: 'Dormiu' }, 'normal');
  assert.equal(r.titulo, 'Boa noite! Pode tirar o pen drive.');
  assert.equal(r.piadaLista, Piadas.DESPEDIDA);
  assert.deepEqual(r.botoes, []);
});

test('render Erro', () => {
  const r = render({ fase: 'Erro', erro: 'Não achei o modelo.' }, 'normal');
  assert.equal(r.titulo, 'Não achei o modelo.');
  assert.deepEqual(r.botoes, ['Abrir a pasta dos registros', 'Fechar']);
});

test('aviso toma a linha de carregamento', () => {
  const aviso = 'Aceleração por GPU indisponível, tentando só com CPU...';
  assert.equal(render({ fase: 'Subindo', pct: 5, aviso }, 'normal').linha, aviso);
});
