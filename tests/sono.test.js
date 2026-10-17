'use strict';
// Testes da vigia do sono (web/sono.js). Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const S = require('../web/sono.js');

// Relógio de mentira: guarda o que foi agendado e deixa o teste disparar um passo por vez.
function relogio(){
  const r = { fila: [], limpos: 0 };
  r.setTimeout = (f, ms) => { r.fila.push({ f, ms }); return r.fila.length; };
  r.clearTimeout = () => { r.limpos++; r.fila = []; };
  r.passo = async () => { const x = r.fila.shift(); if(x) await x.f(); };
  return r;
}
// fetch de mentira que segue um roteiro: true responde 200, false dá erro de rede.
function roteiro(...passos){
  const chamadas = [];
  const f = async (url, opts) => { chamadas.push([url, opts]); if(!passos.shift()) throw new Error('offline'); return { ok: true }; };
  f.chamadas = chamadas; return f;
}
function montar(...passos){
  const t = relogio(), fetch = roteiro(...passos), ev = { dormir: 0, acordar: 0 };
  const v = S.vigiar({ fetch, aoDormir: () => ev.dormir++, aoAcordar: () => ev.acordar++, timers: t });
  return { t, fetch, ev, v };
}

test('uma falha não acusa sono', async () => {
  const { t, ev } = montar(false);
  await t.passo();
  assert.equal(ev.dormir, 0);
});

test('duas falhas seguidas acusam sono uma vez', async () => {
  const { t, ev, fetch } = montar(false, false, false);
  await t.passo(); await t.passo(); await t.passo();
  assert.equal(ev.dormir, 1);
  assert.deepEqual(fetch.chamadas[0], ['/health', { cache: 'no-store' }]);
});

test('resposta com erro HTTP também conta como falha', async () => {
  const t = relogio(); let n = 0;
  S.vigiar({ fetch: async () => ({ ok: false }), aoDormir: () => n++, timers: t });
  await t.passo(); await t.passo();
  assert.equal(n, 1);
});

test('um sucesso no meio zera a contagem', async () => {
  const { t, ev } = montar(false, true, false);
  await t.passo(); await t.passo(); await t.passo();
  assert.equal(ev.dormir, 0);
});

test('volta a responder e acorda', async () => {
  const { t, ev } = montar(false, false, true, true);
  await t.passo(); await t.passo(); await t.passo(); await t.passo();
  assert.equal(ev.dormir, 1);
  assert.equal(ev.acordar, 1);
});

test('acordar só dispara se tinha dormido', async () => {
  const { t, ev } = montar(true, true);
  await t.passo(); await t.passo();
  assert.equal(ev.acordar, 0);
});

test('usa o intervalo pedido', async () => {
  const t = relogio();
  S.vigiar({ fetch: roteiro(true), intervalo: 5000, timers: t });
  assert.equal(t.fila[0].ms, 5000);
});

test('parar para de vigiar', async () => {
  const { t, v, fetch } = montar(true, true);
  await t.passo();
  v.parar();
  assert.equal(t.fila.length, 0);
  assert.equal(fetch.chamadas.length, 1);
});

test('parar durante um fetch em voo não reagenda nem dispara aviso', async () => {
  const t = relogio(); let n = 0, solta;
  const v = S.vigiar({ fetch: () => new Promise((_, rej) => { solta = rej; }), falhas: 1, aoDormir: () => n++, timers: t });
  const p = t.passo();
  v.parar(); solta(new Error('x')); await p;
  assert.equal(n, 0);
  assert.equal(t.fila.length, 0);
});
