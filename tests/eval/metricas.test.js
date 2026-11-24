'use strict';
// Métricas da bateria da História 6 (CIDB-26) com respostas conhecidas: as contas conferem com a
// contagem manual (H6-CT04), o número inventado de propósito é pego e contado (H6-CT05) e a escolha errada
// de propósito conta como erro (H6-CT06). Sem modelo. Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const M = require('./metricas.js');
const C = require('../../web/condicao.js');
const R = require('../../web/resumo.js');
const A = require('../../web/aprofundar.js');
const { controles } = require('./bateria.js');
const resumos = require('./resumo-casos.json');
const aprofs = require('./aprofundamento-casos.json');

const t = (ms, motivo) => ({ ms, ok:!motivo, motivo });

test('H6-CT04: escolha, acerto com aceitáveis, estrito, prazo e falhas, como na contagem manual', () => {
  const itens = [
    { id:'1', esperada:'a', aceitaveis:['a'], escolha:'a', tentativas:[t(3000)] },                 // acerto estrito
    { id:'2', esperada:'a', aceitaveis:['a', 'b'], escolha:'b', tentativas:[t(4000)] },            // acerto (aceitável)
    { id:'3', esperada:'a', aceitaveis:['a'], escolha:'c', tentativas:[t(2000)] },                 // errada
    { id:'4', esperada:'a', aceitaveis:['a'], escolha:'a', tentativas:[t(9000, 'fora'), t(8000)] }, // acerto, 17 s: fora do prazo
    { id:'5', esperada:'a', aceitaveis:['a'], falha:'modelo', tentativas:[{ ms:100, erro:'x' }] },
    { id:'6', esperada:'a', aceitaveis:['a'], escolha:'c', controle:true, tentativas:[t(1)] }      // controle: fora
  ];
  const e = M.escolha(itens);
  assert.equal(e.casos, 5);
  assert.equal(e.acertos, 3);
  assert.equal(e.acertoPct, 60);
  assert.equal(e.estritos, 2);
  assert.equal(e.acertosNoPrazo, 2);
  assert.equal(e.segundaTentativa, 1);
  assert.deepEqual(e.erradas, ['3']);
  assert.deepEqual(e.falhas, { modelo:1 });
  assert.equal(e.respondeu, 4);
  assert.equal(e.latencia.media, Math.round((3000 + 4000 + 2000 + 17000) / 4));
  assert.equal(e.latencia.acimaDoPrazo, 1);
});

test('H6-CT04: resumo, 1ª tentativa sem número inventado, aprovados, prazo e latência', () => {
  const itens = [
    { id:'1', linhas:['x'], tentativas:[t(10000)] },
    { id:'2', linhas:['x'], tentativas:[t(12000, 'numero'), t(11000)] },  // inventou na 1ª, aprovado na 2ª (23 s)
    { id:'3', falha:'numero', tentativas:[t(15000, 'numero'), t(14000, 'numero')] },
    { id:'4', falha:'formato', tentativas:[t(5000, 'formato'), t(5000, 'formato')] },
    { id:'5', linhas:['x'], tentativas:[t(30000)] },                       // aprovado fora do prazo
    { id:'6', falha:'modelo', tentativas:[{ ms:1, erro:'x' }] }
  ];
  const r = M.resumo(itens);
  assert.equal(r.casos, 6);
  assert.equal(r.respondeu, 5);
  assert.equal(r.primeiraLegiveis, 4);          // 5 respondidos, 1 fora do formato
  assert.equal(r.primeiraSemInventado, 2);      // 1 e 5
  assert.equal(r.primeiraSemInventadoPct, 50);
  assert.equal(r.aprovados, 3);
  assert.equal(r.aprovadosNoPrazo, 2);
  assert.equal(r.segundaTentativa, 3);
  assert.deepEqual(r.falhas, { numero:1, formato:1, modelo:1 });
  assert.equal(r.latencia.pior, 30000);
  assert.equal(r.latencia.acimaDoPrazo, 2);     // 29 s e 30 s
});

test('quantil pelo posto mais próximo e latência vazia', () => {
  const xs = Array.from({ length:30 }, (_, i) => i + 1);
  assert.equal(M.quantil(xs, 0.9), 27);
  assert.equal(M.quantil(xs, 0.5), 15);
  assert.equal(M.quantil([7], 0.9), 7);
  assert.equal(M.latencia([], 1).n, 0);
});

test('H6-CT05: resumo com número inventado é recusado pela conferência e contado na métrica', () => {
  const ct = controles(resumos, aprofs).find(c => c.id === 'CT05');
  const res = { ok:true, colunas:ct.colunas, todas:ct.linhas, linhas:ct.linhas };
  const j = C.julgar(ct.consulta.agenda.condicao, res, ct.anteriores);
  const conf = R.conferir(ct.fixa, ct.consulta, res, j);
  assert.equal(conf.ok, false);
  assert.equal(conf.motivo, 'numero');
  const r = M.resumo([{ id:'CT05', falha:'numero', tentativas:[t(1, 'numero')] }, { id:'ok', linhas:['x'], tentativas:[t(1)] }]);
  assert.equal(r.primeiraSemInventado, 1);
  assert.equal(r.aprovados, 1);
});

test('H6-CT06: escolha válida mas errada passa na conferência e conta como erro', () => {
  const ct = controles(resumos, aprofs).find(c => c.id === 'CT06');
  const ops = A.opcoes(ct.consulta, ct.opcoes.map(o => ({ id:o.id, nome:o.nome })));
  const conf = A.conferir(ct.fixaEscolha, ops);
  assert.equal(conf.ok, true, 'o nome existe na lista: a conferência não tem como saber que é a errada');
  assert.ok(!ct.aceitaveis.includes(conf.nome));
  const e = M.escolha([{ id:'CT06', esperada:ct.esperada, aceitaveis:ct.aceitaveis, escolha:conf.nome, tentativas:[t(1)] }]);
  assert.equal(e.acertos, 0);
  assert.deepEqual(e.erradas, ['CT06']);
});

test('frase com maior e menor trocados é apontada; a certa não', () => {
  const todas = [{ d:'21/11', v:117479.24 }, { d:'22/11', v:51612.16 }, { d:'20/11', v:80000 }];
  assert.deepEqual(M.frasesTrocadas(['O dia 22/11/2026 teve o maior faturamento de 51.612,16.'], todas, 'v'),
    ['O dia 22/11/2026 teve o maior faturamento de 51.612,16.']);
  assert.deepEqual(M.frasesTrocadas(['O maior faturamento foi 117.479,24, em 21/11.', 'O menor foi 51.612,16.'], todas, 'v'), []);
  assert.deepEqual(M.frasesTrocadas(['O pior dia teve 117.479'], todas, 'v'), ['O pior dia teve 117.479']);
  assert.deepEqual(M.frasesTrocadas(['qualquer'], [{ v:1 }], 'v'), []);
});

test('duas rodadas: diferenças caso a caso (H6-CT02)', () => {
  const a = [{ etapa:'escolha', id:'A01', escolha:'x' }, { etapa:'resumo', id:'01', linhas:['a'] }, { etapa:'resumo', id:'02', linhas:['b'] }];
  const b = [{ etapa:'escolha', id:'A01', escolha:'y' }, { etapa:'resumo', id:'01', linhas:['a'] }, { etapa:'resumo', id:'02', falha:'numero' }];
  const c = M.comparar(a, b);
  assert.equal(c.total, 3);
  assert.equal(c.iguais, 1);
  assert.deepEqual(c.difs.map(d => d.id), ['A01', '02']);
});

test('casos: 30 por agente; todo caso de aprofundamento dispara, tem 3 opções e o gabarito está nelas (H6-CT01)', () => {
  assert.equal(resumos.length, 30);
  assert.equal(aprofs.length, 30);
  assert.equal(aprofs.filter(c => c.piloto === 'vendas').length, 15);
  for(const c of aprofs){
    const res = { ok:true, colunas:c.colunas, todas:c.linhas, linhas:c.linhas };
    const j = C.julgar(c.consulta.agenda.condicao, res, c.anteriores);
    assert.ok(j.disparou && j.aviso, c.id + ' não dispara');
    const ops = A.opcoes(c.consulta, c.opcoes.map(o => ({ id:o.id, nome:o.nome })));
    assert.equal(ops.length, 3, c.id);
    assert.ok(A.pedido(c.consulta, j, ops), c.id);
    assert.ok(c.aceitaveis.includes(c.esperada) && c.aceitaveis.every(n => ops.some(o => o.nome === n)), c.id);
    assert.ok(c.porque, c.id);
  }
});
