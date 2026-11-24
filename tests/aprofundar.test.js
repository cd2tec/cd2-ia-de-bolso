'use strict';
// Aprofundamento condicional (web/aprofundar.js e o resumo com aprofundamento em web/resumo.js), CIDB-16.
// Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const A = require('../web/aprofundar.js');
const R = require('../web/resumo.js');
const C = require('../web/condicao.js');

const lista = [
  { id:'fat', nome:'Faturamento nos últimos 7 dias' },
  { id:'loja', nome:'Faturamento por loja' },
  { id:'dep', nome:'Faturamento por departamento' },
  { id:'forn', nome:'Faturamento por fornecedor' },
  { id:'outra', nome:'Ruptura' }
];
const cond = { coluna:'faturamento', agregacao:'soma', operador:'caiu', valor:10, base:'anterior' };
const consulta = (aps) => ({ id:'fat', nome:'Faturamento nos últimos 7 dias', agenda:{ freq:'minutos', a_cada:5, condicao:cond, aprofundamentos:aps } });
const res = { ok:true, colunas:['dia', 'faturamento'], todas:[{ dia:'2026-11-21', faturamento:80 }, { dia:'2026-11-22', faturamento:20 }] };
const jul = C.julgar(cond, res, [[{ dia:'2026-11-14', faturamento:200 }]]);

test('julgamento do caso de teste dispara', () => { assert.equal(jul.disparou, true); assert.equal(jul.aviso, true); });

test('validar: de 0 a 3, só com condição, consultas que existem e não a própria (CT03)', () => {
  assert.deepEqual(A.validar([], true, lista, 'fat'), { ok:true });
  assert.deepEqual(A.validar(null, false, lista, 'fat'), { ok:true });
  assert.deepEqual(A.validar([], false, lista, 'fat'), { ok:true }, 'sem vínculo, sem condição: tudo bem');
  assert.deepEqual(A.validar(['loja', 'dep', 'forn'], true, lista, 'fat'), { ok:true });
  assert.deepEqual(A.validar(['loja', 'dep', 'forn', 'outra'], true, lista, 'fat'), { ok:false, erro:A.ERRO_LIMITE });
  assert.equal(A.validar(['loja'], false, lista, 'fat').ok, false, 'sem condição');
  assert.match(A.validar(['loja'], false, lista, 'fat').erro, /Só quando/);
  assert.match(A.validar(['fat'], true, lista, 'fat').erro, /própria/);
  assert.match(A.validar(['sumiu'], true, lista, 'fat').erro, /não existe/);
  assert.equal(A.validar([3], true, lista, 'fat').ok, false);
});

test('ids e opcoes: sem repetição, no máximo 3, só as que existem, nomes únicos (CT08)', () => {
  assert.deepEqual(A.ids(consulta(['loja', 'loja', 'dep', 'forn', 'outra'])), ['loja', 'dep', 'forn']);
  assert.deepEqual(A.ids({ agenda:{} }), []);
  assert.deepEqual(A.ids(null), []);
  const ops = A.opcoes(consulta(['loja', 'sumiu', 'dep']), lista);
  assert.deepEqual(ops, [{ id:'loja', nome:'Faturamento por loja' }, { id:'dep', nome:'Faturamento por departamento' }]);
  const rep = A.opcoes(consulta(['a', 'b']), [{ id:'a', nome:'X' }, { id:'b', nome:'X' }]);
  assert.deepEqual(rep.map(o => o.nome), ['X', 'X (2)']);
  assert.deepEqual(A.opcoes(consulta(['fat', 'loja']), lista).map(o => o.id), ['loja'], 'a própria fica de fora');
});

test('pedido: enum com os nomes das vinculadas; só quando disparou e vai avisar (CT02, CT07)', () => {
  const ops = A.opcoes(consulta(['loja', 'dep', 'forn']), lista);
  const p = A.pedido(consulta(['loja', 'dep', 'forn']), jul, ops);
  assert.deepEqual(p.schema, { type:'object', properties:{ escolha:{ type:'string', enum:['Faturamento por loja', 'Faturamento por departamento', 'Faturamento por fornecedor'] } }, required:['escolha'] });
  const u = p.mensagens[1].content;
  assert.match(u, /^Alerta: A soma de faturamento caiu 50%/);
  assert.match(u, /Base de comparação: execução anterior = 200\n/);
  assert.match(u, /- Faturamento por loja\n- Faturamento por departamento\n- Faturamento por fornecedor$/);
  assert.equal(A.pedido(consulta([]), jul, []), null, 'sem vínculo: fluxo da História 3');
  assert.equal(A.pedido(consulta(['loja']), Object.assign({}, jul, { disparou:false, aviso:false }), ops), null, 'não disparou');
  assert.equal(A.pedido(consulta(['loja']), Object.assign({}, jul, { disparou:false, aviso:true }), ops), null, 'aviso de erro');
  assert.equal(A.pedido(consulta(['loja']), null, ops), null);
});

test('conferir: só um nome da lista; fora do enum ou fora do formato recusa (CT04)', () => {
  const ops = A.opcoes(consulta(['loja', 'dep']), lista);
  assert.deepEqual(A.conferir('{"escolha":"Faturamento por loja"}', ops), { ok:true, id:'loja', nome:'Faturamento por loja' });
  assert.deepEqual(A.conferir('<think>x</think>{"escolha":"Faturamento por departamento"}', ops), { ok:true, id:'dep', nome:'Faturamento por departamento' });
  const fora = A.conferir('{"escolha":"Faturamento por fornecedor"}', ops);
  assert.equal(fora.ok, false); assert.equal(fora.motivo, 'fora'); assert.match(fora.detalhe, /não é uma das consultas/);
  assert.equal(A.conferir('loja', ops).motivo, 'formato');
  assert.equal(A.conferir('{"escolha":2}', ops).motivo, 'formato');
});

test('novaTentativa e limite de 2', () => {
  assert.equal(A.MAX_TENTATIVAS, 2);
  const m = [{ role:'system', content:'s' }, { role:'user', content:'u' }];
  const n = A.novaTentativa(m, '{"escolha":"x"}', { ok:false, motivo:'fora', detalhe:'"x" não é uma das consultas da lista' });
  assert.equal(n.length, 4); assert.equal(m.length, 2);
  assert.match(n[3].content, /^A resposta anterior foi recusada: "x" não é uma das consultas da lista\./);
});

test('textoAprofundamento: o que foi aprofundado ou por que não', () => {
  assert.equal(A.textoAprofundamento({ id:'loja', nome:'Faturamento por loja', tentativas:1 }), 'Aprofundou em: Faturamento por loja');
  assert.equal(A.textoAprofundamento({ falha:'fora', tentativas:2 }), 'Sem aprofundamento: o modelo escolheu uma consulta fora da lista (2 tentativas).');
  assert.equal(A.textoAprofundamento({ falha:'consulta', nome:'Faturamento por loja', tentativas:1 }), 'Sem aprofundamento: a consulta escolhida não rodou ("Faturamento por loja").');
  assert.equal(A.textoAprofundamento({ falha:'prazo', tentativas:1 }), 'Sem aprofundamento: o modelo demorou mais que o prazo para escolher.');
  assert.equal(A.textoAprofundamento(null), '');
});

// ---- resumo com o aprofundamento (web/resumo.js) ----
const ap = { resultado:{ ok:true, colunas:['loja', 'faturamento'], todas:[{ loja:'Loja 01 - Centro', faturamento:15 }, { loja:'Loja 04 - Valinhos', faturamento:5 }] } };
const resp = frases => JSON.stringify({ linhas:frases });

test('resumo: sem aprofundamento, o pedido é igual ao da História 3 (CT02, CT10)', () => {
  const c = consulta([]);
  assert.deepEqual(R.pedido(c, res, jul, null), R.pedido(c, res, jul));
  assert.deepEqual(R.pedido(c, res, jul, { resultado:{ ok:false, erro:'x' } }), R.pedido(c, res, jul), 'aprofundamento que falhou não entra');
  assert.doesNotMatch(R.mensagens(c, res, jul)[1].content, /Aprofundamento/);
});

test('resumo: inclui o resultado do aprofundamento, sem o nome da consulta (CT01)', () => {
  const u = R.mensagens(consulta(['loja']), res, jul, ap)[1].content;
  assert.match(u, /Aprofundamento \(resultado de outra consulta, para explicar o alerta; colunas: loja, faturamento\):\nloja: Loja 01 - Centro; faturamento: 15\nloja: Loja 04 - Valinhos; faturamento: 5\nUse também o Aprofundamento/);
  assert.doesNotMatch(u, /Faturamento por loja/);
  // O resultado principal continua antes.
  assert.ok(u.indexOf('Resultado (') < u.indexOf('Aprofundamento ('));
});

test('resumo: números do aprofundamento valem na conferência; fora dele, não', () => {
  const c = consulta(['loja']);
  const frase = ['O faturamento caiu 50% (200 para 100).', 'A Loja 04 - Valinhos faturou só 5.'];
  assert.equal(R.conferir(resp(frase), c, res, jul, ap).ok, true);
  assert.equal(R.conferir(resp(frase), c, res, jul).ok, false, 'sem o aprofundamento, o 5 não está nos dados');
  assert.equal(R.conferir(resp(['A Loja 04 faturou 7.']), c, res, jul, ap).ok, false);
});

test('resumo: aprofundamento cortado em 10 linhas, sem linhas diz "nenhuma linha"', () => {
  const muitas = Array.from({ length:14 }, (_, i) => ({ loja:'L' + i, faturamento:i }));
  const u = R.mensagens(consulta(['loja']), res, jul, { resultado:{ ok:true, colunas:['loja', 'faturamento'], todas:muitas } })[1].content;
  assert.match(u, /Aprofundamento \(resultado de outra consulta, para explicar o alerta; as 10 primeiras de 14 linhas;/);
  assert.equal(u.split('\n').filter(l => /^loja: L\d+;/.test(l)).length, 10);
  const vazio = R.mensagens(consulta(['loja']), res, jul, { resultado:{ ok:true, colunas:['loja'], todas:[] } })[1].content;
  assert.match(vazio, /\(nenhuma linha\)/);
});
