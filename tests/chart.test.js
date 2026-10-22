'use strict';
// Testes dos gráficos (web/chart.js). Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const G = require('../web/chart.js');
const count = (s, sub) => s.split(sub).length - 1;

test('num aceita número e texto numérico, recusa o resto', () => {
  assert.equal(G.num(12), 12); assert.equal(G.num('1234.50'), 1234.5); assert.equal(G.num(' -3 '), -3); assert.equal(G.num('1e3'), 1000);
  for(const v of ['abc', '', null, undefined, NaN, Infinity, '1.2.3', '12,5']) assert.equal(G.num(v), null, String(v));
});

test('fmt em pt-BR', () => {
  assert.equal(G.fmt(1234.5), '1.234,5'); assert.equal(G.fmt('58900'), '58.900'); assert.equal(G.fmt(0.123), '0,12'); assert.equal(G.fmt('Bebidas'), 'Bebidas');
});

test('labelsOf: meses, datas, nulos e o resto', () => {
  assert.deepEqual(G.labelsOf(['2026-01-01', '2026-02-01T00:00:00Z']), ['jan/2026', 'fev/2026']);
  assert.deepEqual(G.labelsOf(['2026-01-15', '2026-02-01']), ['15/01/2026', '01/02/2026']);
  assert.deepEqual(G.labelsOf(['2026-03', '2026-12']), ['mar/2026', 'dez/2026']);
  assert.deepEqual(G.labelsOf([null, 'Bebidas', '']), ['(vazio)', 'Bebidas', '(vazio)']);
  assert.deepEqual(G.labelsOf([3, 'x']), ['3', 'x']);
});

test('render barra: uma barra por linha, rótulos escapados, sem NaN', () => {
  const s = G.render('barra', ['loja', 'total'], [{ loja:'A', total:10 }, { loja:'<b>&"', total:'5.5' }], { x:'loja', y:'total' });
  assert.ok(s.startsWith('<svg')); assert.equal(count(s, '<rect class="bar'), 2);
  assert.ok(s.includes('&lt;b&gt;&amp;&quot;')); assert.ok(!s.includes('<b>&"')); assert.ok(!/NaN/.test(s));
  assert.ok(s.includes('5,5'));
});

test('render barra com nulo e negativo', () => {
  const s = G.render('barra', ['loja', 'total'], [{ loja:null, total:null }, { loja:'B', total:-5 }, { loja:'C', total:0 }], { x:'loja', y:'total' });
  assert.ok(s.includes('(vazio)')); assert.ok(s.includes('bar neg')); assert.ok(!/NaN/.test(s));
});

test('render linha: um ponto por linha, sem NaN com um ponto ou tudo zero', () => {
  const l = [{ mes:'2026-01-01', v:1 }, { mes:'2026-02-01', v:3 }, { mes:'2026-03-01', v:2 }];
  const s = G.render('linha', ['mes', 'v'], l, { x:'mes', y:'v' });
  assert.equal(count(s, '<circle class="pt"'), 3); assert.ok(s.includes('jan/2026')); assert.ok(!/NaN/.test(s));
  assert.ok(!/NaN/.test(G.render('linha', ['mes', 'v'], [{ mes:'2026-01-01', v:5 }], { x:'mes', y:'v' })));
  assert.ok(!/NaN/.test(G.render('linha', ['mes', 'v'], [{ mes:'a', v:0 }, { mes:'b', v:0 }], { x:'mes', y:'v' })));
});

test('render linha: o último rótulo do eixo aparece e o anterior some quando ficaria em cima dele', () => {
  // 61 meses: o passo é 8, o índice 56 cairia a 4 pontos (uns 60 px) do último e ficava sobreposto
  const l = []; for(let i = 0; i < 61; i++){ const y = 2021 + Math.floor((6 + i) / 12), m = (6 + i) % 12 + 1; l.push({ mes:y + '-' + String(m).padStart(2, '0') + '-01', v:i }); }
  const s = G.render('linha', ['mes', 'v'], l, { x:'mes', y:'v' });
  const rotulos = [...s.matchAll(/text-anchor="middle">([^<]+)</g)].map(m => m[1]);
  assert.equal(rotulos[rotulos.length - 1], 'jul/2026'); assert.ok(!rotulos.includes('mar/2026'), rotulos.join('|'));
  assert.ok(rotulos.includes('jul/2021') && rotulos.length >= 8, rotulos.join('|'));
});

test('render pizza: fatias, legenda, fatia única vira círculo, total zero sem NaN', () => {
  const s = G.render('pizza', ['d', 'v'], [{ d:'A', v:1 }, { d:'B', v:1 }, { d:'C', v:2 }], { x:'d', y:'v' });
  assert.equal(count(s, '<path'), 3); assert.equal(count(s, '<li>'), 3);
  assert.ok(s.includes('fill:var(--pizza-1,#0000ff)'), 'fatia lê o token do tema, com o hex do claro de reserva');
  assert.ok(s.includes('background:var(--pizza-3,#019eff)'), 'legenda usa o mesmo token da fatia');
  assert.ok(G.render('pizza', ['d', 'v'], [{ d:'A', v:9 }], { x:'d', y:'v' }).includes('<circle'));
  assert.ok(!/NaN/.test(G.render('pizza', ['d', 'v'], [{ d:'A', v:0 }, { d:'B', v:0 }], { x:'d', y:'v' })));
});

test('render numero: valor formatado e rótulo escapado', () => {
  const s = G.render('numero', ['<total>'], [{ '<total>':'1234' }], { y:'<total>' });
  assert.ok(s.includes('1.234')); assert.ok(s.includes('&lt;total&gt;')); assert.ok(s.includes('class="kpi"'));
});

test('render tabela: cabeçalho, células escapadas, número alinhado, código sem formatação', () => {
  const s = G.render('tabela', ['nome', 'cnpj', 'valor'], [{ nome:'<x>', cnpj:'12345678000199', valor:'12.5' }, { nome:'y', cnpj:null, valor:3 }]);
  assert.equal(count(s, '<th>'), 3); assert.equal(count(s, '<tr>'), 3);
  assert.ok(s.includes('&lt;x&gt;')); assert.ok(s.includes('12345678000199')); assert.ok(s.includes('12,5')); assert.ok(s.includes('class="n"'));
  assert.ok(G.render('qualquer', ['a'], [{ a:1 }]).includes('<table>'));
});

test('num: overflow de texto numérico vira null, não Infinity', () => {
  assert.equal(G.num('1e999'), null);
});

test('render barra/linha/pizza com overflow no valor: sem NaN nem Infinity', () => {
  const rows = [{ k:'A', v:'1e999' }, { k:'B', v:5 }];
  const barraS = G.render('barra', ['k', 'v'], rows, { x:'k', y:'v' });
  const linhaS = G.render('linha', ['k', 'v'], rows, { x:'k', y:'v' });
  const pizzaS = G.render('pizza', ['k', 'v'], rows, { x:'k', y:'v' });
  for(const s of [barraS, linhaS, pizzaS]) {
    assert.ok(!/NaN/.test(s)); assert.ok(!/Infinity/.test(s));
  }
});

test('labelsOf: só datas inteiras viram data; texto com data no início fica como está', () => {
  assert.deepEqual(G.labelsOf(['2026-01-15 Loja Centro']), ['2026-01-15 Loja Centro']);
  assert.deepEqual(G.labelsOf(['2024-05-12-XYZ']), ['2024-05-12-XYZ']);
  assert.deepEqual(G.labelsOf(['2026-01-01 00:00:00', '2026-02-01T00:00:00.000Z']), ['jan/2026', 'fev/2026']);
  assert.deepEqual(G.labelsOf(['2026-03-05T10:30:00-03:00']), ['05/03/2026']);
});

test('fmt: sem sinal de menos em zero', () => {
  assert.equal(G.fmt(-0), '0');
  assert.equal(G.fmt(-0.001), '0');
});
