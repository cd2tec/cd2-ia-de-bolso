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

test('mesmoBanco ignora caixa do host', () => {
  const a = { motor:'oracle', host:'DB.Local', porta:1521, base:'x', schema:'s' };
  assert.equal(N.mesmoBanco(a, { ...a, host:'db.local' }), true);
  assert.equal(N.mesmoBanco(a, { ...a, base:'y' }), false);
  assert.equal(N.mesmoBanco(a, { ...a, porta:1522 }), false);
  assert.equal(N.mesmoBanco(a, null), false);
});
