'use strict';
// Testes da condição do agendamento (web/condicao.js): quando uma execução avisa. Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const C = require('../web/condicao.js');

const ok = todas => ({ ok:true, todas });
const limite = (operador, valor, extra) => Object.assign({ coluna:'pct', agregacao:'primeira', operador, valor }, extra);
const variacao = (operador, valor, base, n) => ({ coluna:'total', agregacao:'soma', operador, valor, base, n });

test('sem condição: julgar devolve null (o agendador avisa como antes)', () => {
  assert.equal(C.julgar(null, ok([{ pct:9 }]), []), null);
  assert.equal(C.julgar(undefined, ok([{ pct:9 }]), []), null);
});

test('limite fixo: dispara acima, não dispara abaixo nem no limite', () => {
  const c = limite('maior', 4);
  const sim = C.julgar(c, ok([{ pct:5.2 }]), []);
  assert.equal(sim.disparou, true); assert.equal(sim.aviso, true); assert.equal(sim.valor, 5.2);
  assert.equal(sim.texto, 'pct = 5,2, acima de 4.');
  const nao = C.julgar(c, ok([{ pct:3.1 }]), []);
  assert.equal(nao.disparou, false); assert.equal(nao.aviso, false); assert.equal(nao.avaliada, true);
  assert.equal(nao.texto, 'pct = 3,1, não passou de 4.');
  assert.equal(C.julgar(c, ok([{ pct:4 }]), []).disparou, false);
});

test('limite "menor que"', () => {
  const c = limite('menor', 1000);
  assert.equal(C.julgar(c, ok([{ pct:800 }]), []).texto, 'pct = 800, abaixo de 1.000.');
  assert.equal(C.julgar(c, ok([{ pct:1200 }]), []).disparou, false);
});

test('coluna sem caixa e número vindo como texto do banco', () => {
  const r = C.julgar(limite('maior', 4), ok([{ PCT:'4.5' }]), []);
  assert.equal(r.disparou, true);
});

test('várias linhas: cada agregação', () => {
  const linhas = [{ loja:'A', pct:2 }, { loja:'B', pct:7 }, { loja:'C', pct:3 }];
  const v = ag => C.julgar({ coluna:'pct', agregacao:ag, operador:'maior', valor:0 }, ok(linhas), []).valor;
  assert.equal(v('primeira'), 2);
  assert.equal(v('soma'), 12);
  assert.equal(v('media'), 4);
  assert.equal(v('maximo'), 7);
  assert.equal(v('minimo'), 2);
});

test('várias linhas: valores nulos ficam de fora da agregação', () => {
  const r = C.julgar({ coluna:'pct', agregacao:'soma', operador:'maior', valor:5 }, ok([{ pct:null }, { pct:3 }, { pct:4 }]), []);
  assert.equal(r.valor, 7); assert.equal(r.disparou, true);
});

test('resultado vazio: não confere e não avisa', () => {
  const r = C.julgar(limite('maior', 4), { ok:false, motivo:'vazio' }, []);
  assert.deepEqual([r.avaliada, r.disparou, r.aviso, r.motivo], [false, false, false, 'vazio']);
});

test('erro do banco: não confere, mas avisa (o alerta não pode parar calado)', () => {
  for(const motivo of ['sql', 'timeout', 'timeout_front']){
    const r = C.julgar(limite('maior', 4), { ok:false, motivo }, []);
    assert.deepEqual([r.avaliada, r.disparou, r.aviso, r.motivo], [false, false, true, 'erro']);
  }
});

test('coluna que não veio ou sem número: avisa para a pessoa arrumar a condição', () => {
  const sem = C.julgar(limite('maior', 4), ok([{ outra:1 }]), []);
  assert.deepEqual([sem.avaliada, sem.aviso, sem.motivo], [false, true, 'sem_coluna']);
  const txt = C.julgar(limite('maior', 4), ok([{ pct:'muito' }]), []);
  assert.deepEqual([txt.avaliada, txt.aviso, txt.motivo], [false, true, 'sem_numero']);
});

test('variação contra a execução anterior', () => {
  const c = variacao('caiu', 20, 'anterior');
  const r = C.julgar(c, ok([{ total:70 }]), [[{ total:100 }], [{ total:10 }]]);
  assert.equal(r.disparou, true); assert.equal(r.base, 100); assert.equal(r.variacao, -30); assert.equal(r.n, 1);
  assert.equal(r.texto, 'A soma de total caiu 30% (100 para 70) em relação à execução anterior.');
  const nao = C.julgar(c, ok([{ total:90 }]), [[{ total:100 }]]);
  assert.equal(nao.disparou, false); assert.equal(nao.aviso, false);
  assert.equal(nao.texto, 'A soma de total caiu 10% (100 para 90) em relação à execução anterior, não caiu mais de 20%.');
  // Sentido contrário ao da regra: a frase diz o que a regra pedia, não "dentro do limite".
  const subiu = C.julgar(c, ok([{ total:346.8 }]), [[{ total:100 }]]);
  assert.equal(subiu.aviso, false);
  assert.equal(subiu.texto, 'A soma de total subiu 246,8% (100 para 346,8) em relação à execução anterior, não caiu mais de 20%.');
});

test('subiu e variou', () => {
  const ant = [[{ total:100 }]];
  assert.equal(C.julgar(variacao('subiu', 10, 'anterior'), ok([{ total:115 }]), ant).disparou, true);
  assert.equal(C.julgar(variacao('subiu', 10, 'anterior'), ok([{ total:80 }]), ant).disparou, false);
  assert.equal(C.julgar(variacao('variou', 10, 'anterior'), ok([{ total:80 }]), ant).disparou, true);
  assert.equal(C.julgar(variacao('variou', 10, 'anterior'), ok([{ total:105 }]), ant).disparou, false);
  assert.match(C.julgar(variacao('subiu', 10, 'anterior'), ok([{ total:80 }]), ant).texto, /, não subiu mais de 10%\.$/);
  assert.match(C.julgar(variacao('variou', 10, 'anterior'), ok([{ total:105 }]), ant).texto, /, não variou mais de 10%\.$/);
  assert.match(C.julgar(variacao('caiu', 10, 'anterior'), ok([{ total:100 }]), ant).texto, /ficou igual \(100\) .*, não caiu mais de 10%\.$/);
});

test('média das n anteriores: usa só as n mais novas que deram certo', () => {
  const c = variacao('subiu', 50, 'media', 2);
  // A do meio não tem a coluna: pula. Média = (100 + 200) / 2 = 150; 300 é +100%.
  const r = C.julgar(c, ok([{ total:300 }]), [[{ total:100 }], [{ outra:1 }], [{ total:200 }], [{ total:9999 }]]);
  assert.equal(r.base, 150); assert.equal(r.n, 2); assert.equal(r.disparou, true);
  assert.match(r.texto, /em relação à média das 2 anteriores/);
});

test('média com menos execuções do que n usa as que há', () => {
  const r = C.julgar(variacao('subiu', 10, 'media', 7), ok([{ total:200 }]), [[{ total:100 }]]);
  assert.equal(r.n, 1); assert.equal(r.disparou, true);
  assert.match(r.texto, /em relação à única execução anterior/);
});

test('sem execução anterior: não dispara, não avisa e diz por quê', () => {
  const r = C.julgar(variacao('caiu', 20, 'anterior'), ok([{ total:70 }]), []);
  assert.deepEqual([r.avaliada, r.disparou, r.aviso, r.motivo], [false, false, false, 'sem_base']);
  assert.match(r.texto, /Ainda não há execução anterior/);
});

test('base zero: qualquer valor acima conta como subida', () => {
  const r = C.julgar(variacao('subiu', 50, 'anterior'), ok([{ total:5 }]), [[{ total:0 }]]);
  assert.equal(r.disparou, true); assert.equal(r.variacao, null);
  assert.match(r.texto, /subiu de 0 para 5/);
  assert.equal(C.julgar(variacao('variou', 0, 'anterior'), ok([{ total:0 }]), [[{ total:0 }]]).disparou, false);
});

test('validar recusa condição malformada', () => {
  assert.equal(C.validar(null).ok, true);
  assert.equal(C.validar(limite('maior', 4)).ok, true);
  assert.equal(C.validar(limite('maior', NaN)).ok, false);
  assert.equal(C.validar({ coluna:'', agregacao:'soma', operador:'maior', valor:1 }).ok, false);
  assert.equal(C.validar({ coluna:'x', agregacao:'mediana', operador:'maior', valor:1 }).ok, false);
  assert.equal(C.validar(variacao('subiu', -5, 'anterior')).ok, false);
  assert.equal(C.validar(variacao('subiu', 5, 'ontem')).ok, false);
  assert.equal(C.validar(variacao('subiu', 5, 'media', 1)).ok, false);
  assert.equal(C.validar(variacao('subiu', 5, 'media', 31)).ok, false);
  assert.equal(C.validar(variacao('subiu', 5, 'media', 7)).ok, true);
  // Condição inválida guardada: julgar avisa em vez de calar.
  assert.equal(C.julgar({ coluna:'x' }, ok([{ x:1 }]), []).aviso, true);
});

test('tela e daTela: ida e volta, vírgula decimal e "avisar sempre"', () => {
  assert.deepEqual(C.daTela(C.tela(null, 'total')), { ok:true, condicao:null });
  const t = C.tela(null, 'total');
  assert.equal(t.coluna, 'total');
  Object.assign(t, { ligado:true, valor:'4,5' });
  assert.deepEqual(C.daTela(t), { ok:true, condicao:{ coluna:'total', agregacao:'primeira', operador:'maior', valor:4.5 } });
  Object.assign(t, { operador:'caiu', valor:'20', base:'media', n:'7' });
  const c = C.daTela(t).condicao;
  assert.deepEqual(c, { coluna:'total', agregacao:'primeira', operador:'caiu', valor:20, base:'media', n:7 });
  const volta = C.tela(c, 'outra');
  assert.equal(volta.ligado, true); assert.equal(volta.coluna, 'total'); assert.equal(volta.n, '7');
  assert.equal(C.daTela(Object.assign({}, t, { valor:'abc' })).ok, false);
  assert.equal(C.daTela(Object.assign({}, t, { n:'1' })).erro, 'A média usa de 2 a 30 execuções.');
});

test('resumo e textoJulgamento', () => {
  assert.equal(C.resumo(limite('maior', 4)), 'Avisa só quando pct for maior que 4.');
  assert.equal(C.resumo(variacao('caiu', 20, 'media', 7)), 'Avisa só quando a soma de total cair mais de 20% em relação à média das 7 anteriores.');
  assert.equal(C.resumo(null), '');
  assert.equal(C.textoJulgamento({ disparou:true, aviso:true, texto:'x' }), 'Avisou: x');
  assert.equal(C.textoJulgamento({ disparou:false, aviso:false, texto:'x' }), 'Sem aviso: x');
  assert.equal(C.textoJulgamento({ disparou:false, aviso:true, texto:'x' }), 'x');
  assert.equal(C.textoJulgamento(null), '');
});

// ---- colunas da regra (issue #6: a coluna vem em lista, não em campo livre) ----
const R = { ok:true, colunas:['loja', 'valor_total', 'cupons', 'obs'],
  todas:[{ loja:'Loja 01', valor_total:'34204.10', cupons:310, obs:null }, { loja:'Loja 04', valor_total:'12702.50', cupons:120, obs:'' }] };

test('colunasDe: nomes e se é número, sem guardar valores', () => {
  assert.deepEqual(C.colunasDe(R), [
    { nome:'loja', numero:false }, { nome:'valor_total', numero:true }, { nome:'cupons', numero:true }, { nome:'obs', numero:false }]);
  assert.deepEqual(C.colunasDe(null), []);
  assert.deepEqual(C.colunasDe({ ok:false }), []);
});

test('opcoesColuna: só numéricas, y primeiro, a escolhida fica mesmo se sumiu', () => {
  const salva = { y:'valor_total', colunas:C.colunasDe(R) };
  assert.deepEqual(C.opcoesColuna(salva, null, ''), ['valor_total', 'cupons']);
  assert.deepEqual(C.opcoesColuna({ y:'cupons', colunas:C.colunasDe(R) }, null, ''), ['cupons', 'valor_total']);
  assert.deepEqual(C.opcoesColuna(salva, null, 'antiga'), ['valor_total', 'cupons', 'antiga']);
  // consulta antiga, sem colunas guardadas: usa a última execução; sem nada, só o y
  assert.deepEqual(C.opcoesColuna({ y:'valor_total' }, R, ''), ['valor_total', 'cupons']);
  assert.deepEqual(C.opcoesColuna({ y:'total' }, null, ''), ['total']);
  assert.deepEqual(C.opcoesColuna({}, null, ''), []);
});

test('umaLinha: o que a consulta guardou vale; senão, o último resultado', () => {
  assert.equal(C.umaLinha({ umaLinha:true }, R), true);
  assert.equal(C.umaLinha({}, R), false);
  assert.equal(C.umaLinha({}, { todas:[{ total:1 }] }), true);
  assert.equal(C.umaLinha({}, null), false);
});

test('textoUltimo: o número que a regra conferiria no último resultado', () => {
  assert.equal(C.textoUltimo({ coluna:'valor_total', agregacao:'soma' }, R), 'Último resultado: 46.906,6.');
  assert.equal(C.textoUltimo({ coluna:'cupons', agregacao:'primeira' }, R), 'Último resultado: 310.');
  assert.equal(C.textoUltimo({ coluna:'nao_existe', agregacao:'soma' }, R), '');
  assert.equal(C.textoUltimo({ coluna:'cupons', agregacao:'soma' }, null), '');
});

test('rotuloColuna: legível, sem mexer no que já é texto', () => {
  assert.equal(C.rotuloColuna('valor_total'), 'Valor total');
  assert.equal(C.rotuloColuna('Venda (R$)'), 'Venda (R$)');
  assert.equal(C.rotuloColuna(''), '');
});
