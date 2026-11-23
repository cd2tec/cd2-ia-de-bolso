'use strict';
// Testes do resumo do alerta (web/resumo.js): o pedido ao modelo e a conferência dos números.
// Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const R = require('../web/resumo.js');
const C = require('../web/condicao.js');

// Faturamento por dia, como a consulta salva do erp_pdv devolve.
const linhas = [
  { data:'2026-10-30', faturamento:105032.38 },
  { data:'2026-10-31', faturamento:135979.13 },
  { data:'2026-11-01', faturamento:103797.68 },
  { data:'2026-11-22', faturamento:40112.08 }
];
const soma = linhas.reduce((a, l) => a + l.faturamento, 0); // 384.921,27
const consulta = { id:'c1', nome:'Faturamento nos últimos 30 dias',
  agenda:{ condicao:{ coluna:'faturamento', agregacao:'soma', operador:'maior', valor:1 } } };
const resultado = { ok:true, colunas:['data', 'faturamento'], todas:linhas, linhas };
const jul = C.julgar(consulta.agenda.condicao, resultado, []);
const resp = frases => JSON.stringify({ linhas:frases });

test('pedido só quando a condição disparou e vai avisar', () => {
  assert.equal(jul.disparou, true);
  const p = R.pedido(consulta, resultado, jul);
  assert.ok(p);
  assert.deepEqual(p.schema, R.SCHEMA);
  assert.equal(p.mensagens.length, 2);
  assert.equal(R.pedido(consulta, resultado, null), null, 'sem condição: sem resumo');
  assert.equal(R.pedido(consulta, resultado, Object.assign({}, jul, { disparou:false, aviso:false })), null);
  assert.equal(R.pedido(consulta, resultado, Object.assign({}, jul, { disparou:false, aviso:true })), null, 'aviso de erro: sem resumo');
  assert.equal(R.pedido(consulta, { ok:false, motivo:'sql' }, jul), null);
});

test('mensagens: título, frase do alerta e linhas no formato brasileiro', () => {
  const u = R.mensagens(consulta, resultado, jul)[1].content;
  assert.match(u, /Consulta: Faturamento nos últimos 30 dias/);
  assert.match(u, /Alerta: A soma de faturamento = 384\.921,27, acima de 1\./);
  assert.match(u, /data: 31\/10\/2026; faturamento: 135\.979,13/);
  assert.match(u, /Linhas \(4\):/);
});

test('mensagens: corta em 20 linhas e diz quantas eram', () => {
  const muitas = Array.from({ length:30 }, (_, i) => ({ loja:'Loja ' + (i + 1), total:i }));
  const r = { ok:true, colunas:['loja', 'total'], todas:muitas };
  const u = R.mensagens({ nome:'x' }, r, { texto:'t', disparou:true, aviso:true })[1].content;
  assert.match(u, /Linhas \(20 de 30\):/);
  assert.equal(u.split('\n').filter(l => l.startsWith('loja:')).length, 20);
});

test('conferir: resumo com números e datas dos dados passa', () => {
  const r = R.conferir(resp([
    'A soma de faturamento chegou a 384.921,27, acima do limite de 1.',
    'O melhor dia foi 31/10/2026, com 135.979,13.',
    'O pior dia foi 22/11, com 40.112,08.'
  ]), consulta, resultado, jul);
  assert.deepEqual(r, { ok:true, linhas:[
    'A soma de faturamento chegou a 384.921,27, acima do limite de 1.',
    'O melhor dia foi 31/10/2026, com 135.979,13.',
    'O pior dia foi 22/11, com 40.112,08.'
  ] });
});

test('conferir: arredondamento que o texto mostra vale (135.979 e 384,9 mil)', () => {
  assert.equal(R.conferir(resp(['O melhor dia teve 135.979.']), consulta, resultado, jul).ok, true);
  assert.equal(R.conferir(resp(['O mês somou 384,9 mil.']), consulta, resultado, jul).ok, true);
  assert.equal(R.conferir(resp(['O mês somou 0,38 milhão.']), consulta, resultado, jul).ok, true);
});

test('conferir: número inventado descarta o resumo inteiro', () => {
  const r = R.conferir(resp(['A soma foi 384.921,27.', 'Isso é 12% acima do mês passado.']), consulta, resultado, jul);
  assert.equal(r.ok, false); assert.equal(r.motivo, 'numero'); assert.match(r.detalhe, /12/);
  assert.equal(R.conferir(resp(['O melhor dia teve 135.980.']), consulta, resultado, jul).ok, false, 'arredondou errado');
  assert.equal(R.conferir(resp(['A soma foi 386 mil.']), consulta, resultado, jul).ok, false, 'arredondou demais');
  assert.equal(R.conferir(resp(['Média diária de 96.230,32.']), consulta, resultado, jul).ok, false, 'calculou a média');
});

test('conferir: data ou horário que não está nos dados descarta', () => {
  const r = R.conferir(resp(['O melhor dia foi 15/10/2026.']), consulta, resultado, jul);
  assert.equal(r.ok, false); assert.equal(r.motivo, 'numero'); assert.match(r.detalhe, /15\/10\/2026/);
  assert.equal(R.conferir(resp(['O melhor dia foi 31/10/2025.']), consulta, resultado, jul).ok, false, 'ano errado');
  assert.equal(R.conferir(resp(['Rodou às 13:00.']), consulta, resultado, jul).ok, false);
});

test('conferir: números do título, de nomes e da regra podem ser citados', () => {
  const c = { nome:'Produto com mais saída nos últimos 7 dias',
    agenda:{ condicao:{ coluna:'quantidade', agregacao:'primeira', operador:'menor', valor:2500 } } };
  const r = { ok:true, colunas:['produto', 'quantidade'], todas:[{ produto:'Cerveja Pilsen Do Campo 350ml', quantidade:2053 }] };
  const j = C.julgar(c.agenda.condicao, r, []);
  const ok = R.conferir(resp(['Nos últimos 7 dias, a Cerveja Pilsen Do Campo 350ml saiu 2.053 vezes, abaixo de 2.500.']), c, r, j);
  assert.equal(ok.ok, true, JSON.stringify(ok));
});

test('conferir: variação e base vêm do julgamento', () => {
  const c = { nome:'Vendas', agenda:{ condicao:{ coluna:'total', agregacao:'primeira', operador:'caiu', valor:10, base:'anterior' } } };
  const r = { ok:true, colunas:['total'], todas:[{ total:80 }] };
  const j = C.julgar(c.agenda.condicao, r, [[{ total:100 }]]);
  assert.equal(j.disparou, true);
  assert.equal(R.conferir(resp(['O total caiu 20% (de 100 para 80), mais que o limite de 10%.']), c, r, j).ok, true);
  assert.equal(R.conferir(resp(['O total caiu 25%.']), c, r, j).ok, false);
});

test('conferir: formato errado', () => {
  const f = t => R.conferir(t, consulta, resultado, jul);
  assert.equal(f('não é json').motivo, 'formato');
  assert.equal(f('{"texto":"x"}').motivo, 'formato');
  assert.equal(f(resp([])).motivo, 'formato');
  assert.equal(f(resp(['a', 'b', 'c', 'd'])).motivo, 'formato');
  assert.equal(f(resp(['a'.repeat(R.MAX_FRASE + 1)])).motivo, 'formato');
  assert.equal(f(resp(['ok', 3])).motivo, 'formato');
  assert.equal(f('<think>pensando</think>' + resp(['Sem números aqui.'])).ok, true);
  assert.equal(f('Claro! ' + resp(['Sem números aqui.']) + ' Pronto.').ok, true);
});

test('celula: data, número e texto', () => {
  assert.equal(R.celula('2026-10-31'), '31/10/2026');
  assert.equal(R.celula('2026-10-31T00:00:00Z'), '31/10/2026');
  assert.equal(R.celula(135979.13), '135.979,13');
  assert.equal(R.celula('135979.13'), '135.979,13');
  assert.equal(R.celula('7891000100103'), '7891000100103', 'código em texto fica como está');
  assert.equal(R.celula(null), '');
});

test('textoAviso: uma frase por linha', () => {
  assert.equal(R.textoAviso(['a', 'b']), 'a\nb');
  assert.equal(R.textoAviso(null), '');
});

test('soma de conferência bate com o julgamento', () => {
  assert.ok(Math.abs(jul.valor - soma) < 1e-6);
});
