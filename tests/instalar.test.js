'use strict';
// Testes da lógica do modal de instalação (web/instalar.js). Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const I = require('../web/instalar.js');
const v = require('../instalacao/testdata/vetor.json');

// Canal falso no formato do BroadcastChannel: o que um lado posta, os outros ouvem (nunca o próprio).
function criarBarramento(){
  const canais = [];
  return function novo(){
    const ouvintes = [];
    const c = {
      postMessage(data){ for(const o of canais) if(o !== c) o._entregar(data); },
      addEventListener(t, f){ if(t === 'message') ouvintes.push(f); },
      removeEventListener(t, f){ const i = ouvintes.indexOf(f); if(i >= 0) ouvintes.splice(i, 1); },
      _entregar(data){ for(const f of ouvintes.slice()) f({ data }); },
      _ouvintes: ouvintes,
    };
    canais.push(c);
    return c;
  };
}

test('urlComprar', () => assert.equal(I.urlComprar('7F3K-Q9WD-2MXA', 8080),
  'https://instalariadebolso.cd2.com.br/comprar?codigo=7F3K-Q9WD-2MXA&porta=8080'));

test('telaInicial', () => {
  assert.equal(I.telaInicial({ origem: 'instalada' }), null);
  assert.equal(I.telaInicial({ origem: 'desconhecida' }), null);
  assert.equal(I.telaInicial(null), null);
  assert.equal(I.telaInicial(undefined), null);
  assert.equal(I.telaInicial({ origem: 'pendrive', temChave: true }), 'reinstalar');
  assert.equal(I.telaInicial({ origem: 'pendrive', temChave: false }), 'compra');
});

test('lerParametros', () => {
  assert.deepEqual(I.lerParametros(''), { abrir: false, chave: null });
  assert.deepEqual(I.lerParametros('?instalar=1'), { abrir: true, chave: null });
  assert.deepEqual(I.lerParametros('instalar=0'), { abrir: false, chave: null });
  assert.deepEqual(I.lerParametros('?chave='), { abrir: false, chave: null });
  const r = I.lerParametros('?instalar=1&chave=' + encodeURIComponent(v.chave));
  assert.deepEqual(r, { abrir: true, chave: v.chave });
});

test('normalizar igual ao Go', () => {
  assert.equal(I.normalizar(v.chave), v.chave);
  const suja = '  ' + v.chave.toLowerCase().replace(/0/g, 'o').replace(/1/g, 'l').replace(/-/g, ' - ') + '\n';
  assert.equal(I.normalizar(suja), v.chave);
  assert.equal(I.normalizar(v.chave.replace(/-/g, '')), v.chave);
  assert.equal(I.normalizar(v.chave.slice(0, -1)), null);
  assert.equal(I.normalizar(v.chave + '0'), null);
  assert.equal(I.normalizar('XYZ-' + v.chave.slice(4)), null);
  assert.equal(I.normalizar(v.chave.slice(0, 10) + 'U' + v.chave.slice(11)), null);
  assert.equal(I.normalizar(v.chave.slice(0, -1) + '1'), null); // bits de enchimento não zerados
  assert.equal(I.normalizar(''), null);
  assert.equal(I.normalizar(null), null);
});

test('voltaSemAbaOriginal', async () => {
  const canal = criarBarramento()();
  const r = await I.entregarChave({ canal, chave: v.chave, esperarMs: 10 });
  assert.equal(r, 'sem-aba');
  assert.equal(canal._ouvintes.length, 0);
});

test('voltaComAbaOriginal', async () => {
  const novo = criarBarramento();
  const aba = novo(), volta = novo();
  let recebida = null;
  const o = I.ouvirChave({ canal: aba, aoReceber: c => { recebida = c; } });
  const r = await I.entregarChave({ canal: volta, chave: v.chave, esperarMs: 500 });
  assert.equal(r, 'entregue');
  assert.equal(recebida, v.chave);
  assert.equal(volta._ouvintes.length, 0);
  o.parar();
  assert.equal(aba._ouvintes.length, 0);
});

test('entregarChave limpa o timer ao receber', async () => {
  const novo = criarBarramento();
  const aba = novo(), volta = novo();
  I.ouvirChave({ canal: aba, aoReceber(){} });
  const ativos = new Set();
  const timers = {
    setTimeout: (f, ms) => { const id = Symbol(); ativos.add(id); return id; },
    clearTimeout: id => ativos.delete(id),
  };
  assert.equal(await I.entregarChave({ canal: volta, chave: v.chave, timers }), 'entregue');
  assert.equal(ativos.size, 0);
});

test('textoProgresso', () => {
  assert.equal(I.textoProgresso({ feitos: 1.4e9, total: 2.4e9 }), 'copiando ... 1,4 de 2,4 GB');
  assert.equal(I.textoProgresso({ feitos: 0, total: 0 }), 'copiando ...');
  assert.equal(I.textoProgresso({}), 'copiando ...');
});

test('ouvirChave ignora mensagem sem chave valida', () => {
  const novo = criarBarramento();
  const aba = novo(), outro = novo();
  let chamadas = 0, acks = 0;
  I.ouvirChave({ canal: aba, aoReceber(){ chamadas++; } });
  outro.addEventListener('message', ev => { if(ev.data.tipo === 'recebi') acks++; });
  outro.postMessage({ tipo: 'chave' });
  outro.postMessage({ tipo: 'chave', chave: 42 });
  outro.postMessage({ tipo: 'chave', chave: 'lixo' });
  assert.equal(chamadas, 0);
  assert.equal(acks, 0);
});

test('ouvirChave entrega a chave normalizada', () => {
  const novo = criarBarramento();
  const aba = novo(), outro = novo();
  let recebida = null;
  I.ouvirChave({ canal: aba, aoReceber: c => { recebida = c; } });
  outro.postMessage({ tipo: 'chave', chave: v.chave.toLowerCase() });
  assert.equal(recebida, v.chave);
});

test('aoReceber que lanca erro nao desfaz o ack', () => {
  const novo = criarBarramento();
  const aba = novo(), outro = novo();
  let acks = 0;
  I.ouvirChave({ canal: aba, aoReceber(){ throw new Error('falhou'); } });
  outro.addEventListener('message', ev => { if(ev.data.tipo === 'recebi') acks++; });
  assert.throws(() => outro.postMessage({ tipo: 'chave', chave: v.chave }), /falhou/);
  assert.equal(acks, 1);
});

test('entregarChave resolve sem-aba se postMessage lanca erro', async () => {
  const ativos = new Set();
  const timers = { setTimeout: () => { const i = Symbol(); ativos.add(i); return i; }, clearTimeout: i => ativos.delete(i) };
  let removidos = 0;
  const canal = { postMessage(){ throw new Error('canal fechado'); }, addEventListener(){}, removeEventListener(){ removidos++; } };
  assert.equal(await I.entregarChave({ canal, chave: 'x', timers }), 'sem-aba');
  assert.equal(ativos.size, 0);
  assert.equal(removidos, 1);
});

test('semParametros tira instalar e chave e mantém o resto', () => {
  assert.equal(I.semParametros('?v=0.14.0&instalar=1&chave=CD2-ABC'), '?v=0.14.0');
  assert.equal(I.semParametros('?instalar=1'), '');
  assert.equal(I.semParametros('chave=X&v=1&outro=a%20b'), '?v=1&outro=a+b');
  assert.equal(I.semParametros(''), '');
});
