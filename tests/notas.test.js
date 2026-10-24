'use strict';
// Testes das notas da TI (web/notas.js). Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const N = require('../web/notas.js');

test('aplicar põe nota na tabela e no campo e ignora o que sumiu do banco', () => {
  const cat = { tables:[{ name:'vendas', tokens:[], columns:[{ name:'status', tokens:[] }] }] };
  N.aplicar(cat, { versao:1, consultas:[], tabelas:{ vendas:{ nota:'Tabela oficial', campos:{ status:'9 = cancelado', antiga:'x' } }, apagada:{ nota:'y', campos:{} } } });
  assert.equal(cat.tables[0].nota, 'Tabela oficial');
  assert.equal(cat.tables[0].columns[0].nota, '9 = cancelado');
  assert.ok(cat.tables[0].notaTokens.includes('oficial'));
  assert.equal(cat.tables.length, 1);
});

test('aplicar remove notas antigas antes de aplicar', () => {
  const cat = { tables:[{ name:'vendas', nota:'velha', notaTokens:['velha'], columns:[{ name:'status', nota:'velha' }] }] };
  N.aplicar(cat, N.vazio());
  assert.equal(cat.tables[0].nota, undefined);
  assert.equal(cat.tables[0].notaTokens, undefined);
  assert.equal(cat.tables[0].columns[0].nota, undefined);
});

test('vazio e contar', () => {
  assert.deepEqual(N.vazio(), { versao:1, tabelas:{}, consultas:[] });
  assert.deepEqual(N.contar(N.vazio()), { tabelasComNota:0 });
  const d = { versao:1, consultas:[], tabelas:{ a:{ nota:'x', campos:{} }, b:{ nota:'', campos:{ c:'y' } }, c:{ nota:' ', campos:{} } } };
  assert.equal(N.contar(d).tabelasComNota, 1);
});

test('limpar corta em 300 e tira quebra de linha', () => {
  assert.equal(N.limpar('a\nb', 300), 'a b');
  assert.equal(N.limpar('a\u0000\t  b', 300), 'a b');
  assert.equal(N.limpar('a'.repeat(400), 300).length, 300);
  assert.equal(N.limpar(null, 300), '');
});

function tabelasParaCortar(){
  const col = (n) => ({ name:'c' + n, type:'X', kind:'texto', nota:'n'.repeat(120) });
  const mk = (name, score) => ({ name, score, nota:'t'.repeat(100), perfil:{ papel:'movimento', descricao:'d'.repeat(80) },
    columns:Array.from({ length:15 }, (_, i) => col(i)) });
  return [mk('a', 10), mk('b', 5)];
}
const total = ts => ts.reduce((s, t) => s + (t.nota || '').length + (t.perfil && t.nota ? (t.perfil.descricao || '').length : 0)
  + t.columns.reduce((x, c) => x + (c.nota || '').length, 0), 0);

test('cortar: tira notas de campo da última tabela primeiro, nunca a nota da tabela', () => {
  const ts = tabelasParaCortar();
  const r = N.cortar(ts, 2000);
  assert.equal(r[0].nota.length, 100);
  assert.equal(r[1].nota.length, 100);
  assert.ok(total(r) <= 2000);
  const n0 = r[0].columns.filter(c => c.nota).length, n1 = r[1].columns.filter(c => c.nota).length;
  assert.ok(n1 < n0 || n1 === 0);
  if(n1 > 0) assert.equal(n0, 15);
  // não muta a entrada
  assert.equal(ts[1].columns.filter(c => c.nota).length, 15);
  assert.equal(ts[0].perfil.descricao.length, 80);
});

test('cortar: depois das notas de campo, tira a descrição da IA de tabela com nota', () => {
  const ts = [
    { name:'a', score:9, nota:'x'.repeat(300), perfil:{ descricao:'d'.repeat(200) }, columns:[{ name:'c', nota:'y'.repeat(120) }] },
    { name:'b', score:5, nota:'x'.repeat(300), perfil:{ descricao:'e'.repeat(200) }, columns:[{ name:'c', nota:'y'.repeat(120) }] },
    { name:'c', score:1, perfil:{ descricao:'f'.repeat(200) }, columns:[{ name:'c' }] },
  ];
  // soma: 600 + 240 + 400 = 1240; limite 800 exige tirar campos (240) e uma descrição
  const r = N.cortar(ts, 800);
  assert.equal(r[0].columns[0].nota, undefined);
  assert.equal(r[1].columns[0].nota, undefined);
  assert.equal(r[1].perfil.descricao, undefined);
  assert.equal(r[0].perfil.descricao.length, 200);
  assert.equal(r[0].nota.length, 300);
  assert.equal(r[2].perfil.descricao.length, 200);
  assert.ok(total(r) <= 800);
});

test('cortar: sem estouro não mexe em nada', () => {
  const ts = [{ name:'a', score:1, nota:'x', perfil:{ descricao:'d' }, columns:[{ name:'c', nota:'y' }] }];
  assert.deepEqual(N.cortar(ts, 2000), ts);
});

test('exportar não leva nota vazia e dá nome com a base e a data', () => {
  const dados = { versao:1, consultas:[], tabelas:{ vendas:{ nota:'ok', campos:{ a:'x', b:'' } }, vazia:{ nota:'', campos:{ z:' ' } } } };
  const banco = { motor:'oracle', host:'h', porta:1521, base:'loja01', schema:'S' };
  const r = N.exportar(dados, banco, new Date('2026-10-21T15:00:00Z'));
  assert.equal(r.nome, 'ia-de-bolso-notas-loja01-2026-10-21.json');
  const j = JSON.parse(r.conteudo);
  assert.equal(j.formato, 'ia-de-bolso-notas');
  assert.equal(j.versao, 1);
  assert.deepEqual(j.banco, banco);
  assert.deepEqual(Object.keys(j.dados.tabelas), ['vendas']);
  assert.deepEqual(j.dados.tabelas.vendas.campos, { a:'x' });
  assert.ok(j.exportado.startsWith('2026-10-21'));
});

test('lerArquivo recusa JSON estragado e outro formato', () => {
  const msg = 'Este arquivo não é uma exportação da IA de Bolso.';
  assert.deepEqual(N.lerArquivo('{'), { ok:false, erro:msg });
  assert.deepEqual(N.lerArquivo('{"formato":"x"}'), { ok:false, erro:msg });
});

test('lerArquivo aceita a exportação e devolve banco e dados', () => {
  const banco = { motor:'oracle', host:'h', porta:1, base:'b', schema:'s' };
  const e = N.exportar({ versao:1, consultas:[], tabelas:{ v:{ nota:'n', campos:{} } } }, banco, new Date());
  const r = N.lerArquivo(e.conteudo);
  assert.equal(r.ok, true);
  assert.deepEqual(r.banco, banco);
  assert.equal(r.dados.tabelas.v.nota, 'n');
});

const dd = (tabelas, consultas) => ({ versao:1, tabelas, consultas:consultas || [] });
const q = (id, nome, sql) => ({ id, nome, sql, visual:'tabela', x:'', y:'', params:[], criada:'2026-10-01' });

test('planoImportacao junta e só sobrescreve o que está preenchido dos dois lados', () => {
  const atual = dd({ vendas:{ nota:'A', campos:{} }, lojas:{ nota:'L', campos:{} } }, [q('1', 'Top', 'sql1')]);
  const chegando = dd({ vendas:{ nota:'B', campos:{} }, produtos:{ nota:'P', campos:{} } }, [q('2', 'top ', 'sql2'), q('3', 'Nova', 'sql3')]);
  const p = N.planoImportacao(atual, chegando);
  assert.equal(p.conflitos.length, 2);
  const t = p.conflitos.find(c => c.tipo === 'tabela');
  assert.deepEqual([t.onde, t.antes, t.depois], ['vendas', 'A', 'B']);
  const c = p.conflitos.find(x => x.tipo === 'consulta');
  assert.equal(c.antes, 'sql1'); assert.equal(c.depois, 'sql2');
  assert.deepEqual(p.novas, { notas:1, consultas:1 });
  assert.equal(p.resultado.tabelas.lojas.nota, 'L');
  assert.equal(p.resultado.tabelas.vendas.nota, 'B');
  assert.equal(p.resultado.tabelas.produtos.nota, 'P');
  assert.equal(p.resultado.consultas.length, 2);
  assert.equal(p.resultado.consultas.find(x => x.nome.trim().toLowerCase() === 'top').sql, 'sql2');
  // não muta
  assert.equal(atual.tabelas.vendas.nota, 'A');
});

test('planoImportacao: campo em conflito e campo vazio de um lado', () => {
  const atual = dd({ v:{ nota:'', campos:{ a:'1', b:'' } } });
  const chegando = dd({ v:{ nota:'N', campos:{ a:'2', b:'3' } } });
  const p = N.planoImportacao(atual, chegando);
  assert.deepEqual(p.conflitos, [{ tipo:'campo', onde:'v.a', antes:'1', depois:'2' }]);
  assert.equal(p.resultado.tabelas.v.nota, 'N');
  assert.equal(p.resultado.tabelas.v.campos.b, '3');
  assert.equal(p.novas.notas, 2);
});

test('planoImportacao: mesmo texto dos dois lados não é conflito', () => {
  const a = dd({ v:{ nota:'A', campos:{ c:'x' } } }, [q('1', 'Top', 'sql')]);
  const p = N.planoImportacao(a, dd({ v:{ nota:'A', campos:{ c:'x' } } }, [q('9', 'Top', 'sql')]));
  assert.deepEqual(p.conflitos, []);
  assert.deepEqual(p.novas, { notas:0, consultas:0 });
  assert.equal(p.resultado.consultas.length, 1);
});

test('planoImportacao: mesma consulta com valor de parâmetro diferente não é conflito', () => {
  const comParam = (id, valor) => ({ ...q(id, 'Top', 'SELECT 1 WHERE a > 10'), params:[{ nome:'Mínimo', tipo:'numero', inicio:20, fim:22, valor }] });
  const p = N.planoImportacao(dd({}, [comParam('1', '10')]), dd({}, [comParam('9', '25')]));
  assert.deepEqual(p.conflitos, []);
  assert.deepEqual(p.novas, { notas:0, consultas:0 });
  assert.equal(p.resultado.consultas.length, 1);
  // parâmetro em outro lugar continua sendo conflito
  const outro = { ...comParam('9', '10') }; outro.params = [{ ...outro.params[0], nome:'Outro' }];
  assert.equal(N.planoImportacao(dd({}, [comParam('1', '10')]), dd({}, [outro])).conflitos.length, 1);
});

test('mesmoBanco ignora caixa do host', () => {
  const a = { motor:'oracle', host:'DB.Local', porta:1521, base:'x', schema:'s' };
  assert.equal(N.mesmoBanco(a, { ...a, host:'db.local' }), true);
  assert.equal(N.mesmoBanco(a, { ...a, base:'y' }), false);
  assert.equal(N.mesmoBanco(a, { ...a, porta:1522 }), false);
  assert.equal(N.mesmoBanco(a, null), false);
});

test('lerArquivo limpa o texto importado: teto e sem quebra de linha', () => {
  const j = { formato:'ia-de-bolso-notas', versao:1, banco:{}, dados:{ versao:1, consultas:[], tabelas:{ v:{ nota:'a'.repeat(50000), campos:{ c:('x\n').repeat(250) } } } } };
  const r = N.lerArquivo(JSON.stringify(j));
  assert.equal(r.dados.tabelas.v.nota.length, 300);
  assert.equal(r.dados.tabelas.v.campos.c.length, 120);
  assert.ok(!/[\n\r]/.test(r.dados.tabelas.v.campos.c));
});

test('arquivo malformado não estoura e perde só as entradas ruins', () => {
  const j = { formato:'ia-de-bolso-notas', versao:1, banco:{}, dados:{ versao:1,
    consultas:[null, 5, { nome:'ok', sql:'s', params:'x' }, { nome:1, sql:'s' }, { nome:'sem sql' }],
    tabelas:{ a:null, b:'texto', c:{ nota:'n', campos:'abc' }, d:{ nota:7, campos:{ k:3 } } } } };
  const r = N.lerArquivo(JSON.stringify(j));
  assert.equal(r.ok, true);
  assert.deepEqual(r.dados.consultas, [{ nome:'ok', sql:'s', params:[] }]);
  assert.deepEqual(Object.keys(r.dados.tabelas), ['c']);
  assert.deepEqual(r.dados.tabelas.c.campos, {});
  assert.doesNotThrow(() => N.planoImportacao(r.dados, r.dados));
  assert.doesNotThrow(() => N.planoImportacao(vazioDados(), { tabelas:{ x:null }, consultas:[null] }));
});
const vazioDados = () => N.vazio();

test('planoImportacao: tabela com nome de propriedade herdada', () => {
  const p = N.planoImportacao(N.vazio(), dd({ constructor:{ nota:'C', campos:{ toString:'T' } } }));
  assert.equal(p.resultado.tabelas.constructor.nota, 'C');
  assert.equal(p.resultado.tabelas.constructor.campos.toString, 'T');
  assert.equal(p.novas.notas, 2);
  const p2 = N.planoImportacao(p.resultado, dd({ constructor:{ nota:'D', campos:{} } }));
  assert.equal(p2.conflitos.length, 1);
});

test('planoImportacao: mesma consulta com params diferentes é conflito', () => {
  const a = q('1', 'Top', 'sql'), b = { ...q('2', 'Top', 'sql'), params:[{ nome:'d', tipo:'data', valor:'1' }] };
  const p = N.planoImportacao(dd({}, [a]), dd({}, [b]));
  assert.equal(p.conflitos.length, 1);
  assert.equal(p.conflitos[0].tipo, 'consulta');
  assert.deepEqual(p.resultado.consultas[0].params, b.params);
});

test('importar desliga a agenda que vem do arquivo', () => {
  const ag = (ligado, freq) => ({ ligado, freq, params:[{ nome:'d', valor:'1' }] });
  const so = { ...q('1', 'Só aqui', 'a'), agenda:ag(true, 'diaria') };
  const nova = { ...q('2', 'Nova', 'b'), agenda:ag(true, 'semanal') };
  const sem = q('3', 'Sem agenda', 'c');
  const p = N.planoImportacao(dd({}, [so]), dd({}, [nova, sem]));
  const por = n => p.resultado.consultas.find(c => c.nome === n);
  assert.equal(por('Só aqui').agenda.ligado, true);
  assert.equal(por('Nova').agenda.ligado, false);
  assert.equal(por('Nova').agenda.freq, 'semanal');
  assert.equal('agenda' in por('Sem agenda'), false);
  assert.equal(nova.agenda.ligado, true); // não muta a entrada
  // conflito real: fica com o texto do arquivo, mas a agenda daqui se já havia
  const local = { ...q('1', 'Top', 'sql velho'), agenda:ag(true, 'diaria') };
  const arq = { ...q('9', 'Top', 'sql novo'), agenda:ag(true, 'mensal') };
  const c1 = N.planoImportacao(dd({}, [local]), dd({}, [arq])).resultado.consultas[0];
  assert.equal(c1.sql, 'sql novo');
  assert.deepEqual(c1.agenda, local.agenda);
  // conflito real, local sem agenda: leva a do arquivo, desligada
  const c2 = N.planoImportacao(dd({}, [q('1', 'Top', 'sql velho')]), dd({}, [arq])).resultado.consultas[0];
  assert.equal(c2.agenda.ligado, false);
  assert.equal(c2.agenda.freq, 'mensal');
});

test('agenda diferente não é conflito e a daqui fica', () => {
  const local = { ...q('1', 'Top', 'sql'), agenda:{ ligado:true, freq:'diaria', params:[] } };
  const arq = { ...q('9', 'Top', 'sql'), agenda:{ ligado:true, freq:'semanal', params:[] } };
  const p = N.planoImportacao(dd({}, [local]), dd({}, [arq]));
  assert.deepEqual(p.conflitos, []);
  assert.deepEqual(p.resultado.consultas[0].agenda, local.agenda);
  // exportar leva a agenda
  const ex = JSON.parse(N.exportar(dd({}, [local]), { base:'x' }).conteudo);
  assert.deepEqual(ex.dados.consultas[0].agenda, local.agenda);
});

test('cortar não cria perfil em tabela sem perfil', () => {
  const r = N.cortar([{ name:'a', nota:'x', columns:[{ name:'c', nota:'y' }] }], 1);
  assert.ok(!('perfil' in r[0]));
});
