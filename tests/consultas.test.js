'use strict';
// Testes das consultas salvas (web/consultas.js): achar literais, virar parâmetro e montar o SQL. Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const C = require('../web/consultas.js');

const SQL = "SELECT l.nome AS loja, SUM(v.total) AS total FROM vendas_2023 v JOIN lojas l ON l.cod = v.cod_loja "
  + "WHERE v.dt_mov >= '2026-10-01' AND v.dt_mov < '2026-11-01' AND v.status <> 9 AND UPPER(l.cidade) LIKE '%CURITIBA%' -- 77\nGROUP BY l.nome";
const pos = (s, trecho) => { const i = s.indexOf(trecho); assert.ok(i >= 0, trecho); return [i, i + trecho.length]; };

test('acharLiterais: datas marcadas, número e texto do LIKE sem os %', () => {
  assert.deepEqual(C.acharLiterais(SQL).map(l => [l.valor, l.tipo, l.marcado]),
    [['2026-10-01','data',true], ['2026-11-01','data',true], ['9','numero',false], ['CURITIBA','texto',false]]);
  const cur = C.acharLiterais(SQL)[3];
  assert.equal(SQL.slice(cur.inicio, cur.fim), 'CURITIBA');
  const d = C.acharLiterais(SQL)[0];
  assert.equal(SQL.slice(d.inicio, d.fim), '2026-10-01');
});

test('acharLiterais ignora número em identificador, aspas duplas, colchetes e comentário', () => {
  const s = 'SELECT "Loja 3", [Loja 3], `x1`, t1.col, v.c2 FROM vendas_2023 t1 /* 5 e \'oi\' */ WHERE t1.n = 42 -- 77 \'x\'\n AND t1.k = \'A\'\'B\'';
  assert.deepEqual(C.acharLiterais(s).map(l => [l.valor, l.tipo]), [['42','numero'], ["A'B",'texto']]);
});

test('acharLiterais: data com hora fica marcada e só a parte da data vira parâmetro', () => {
  const s = "SELECT a FROM t WHERE d < '2026-11-01 00:00:00'";
  const [l] = C.acharLiterais(s);
  assert.equal(l.tipo, 'data'); assert.equal(l.valor, '2026-11-01'); assert.equal(l.marcado, true);
  assert.equal(s.slice(l.inicio, l.fim), '2026-11-01');
});

test('acharLiterais para no primeiro literal com barra invertida (motor ambíguo)', () => {
  const s = "SELECT 1 FROM t WHERE a = 'X' AND b = 'a\\' OR 1=1 AND c = '7'";
  assert.deepEqual(C.acharLiterais(s).map(l => l.valor), ['1', 'X']);
});

test('paramDeTrecho aceita trecho dentro de aspas e recusa pedaço de identificador', () => {
  const [a, b] = pos(SQL, 'CURITIBA');
  assert.deepEqual(C.paramDeTrecho(SQL, a, b), { ok:true, param:{ valor:'CURITIBA', tipo:'texto', inicio:a, fim:b } });
  const [da, db] = pos(SQL, "'2026-10-01'");
  assert.deepEqual(C.paramDeTrecho(SQL, da, db), { ok:true, param:{ valor:'2026-10-01', tipo:'data', inicio:da + 1, fim:db - 1 } });
  const [na, nb] = pos(SQL, '9 AND');
  assert.deepEqual(C.paramDeTrecho(SQL, na, na + 1), { ok:true, param:{ valor:'9', tipo:'numero', inicio:na, fim:na + 1 } });
  const erro = { ok:false, erro:'Selecione um valor dentro de aspas ou um número inteiro.' };
  const [ia] = pos(SQL, '2023');
  assert.deepEqual(C.paramDeTrecho(SQL, ia, ia + 4), erro);
  const [la, lb] = pos(SQL, 'lojas');
  assert.deepEqual(C.paramDeTrecho(SQL, la, lb), erro);
  const [ca] = pos(SQL, '77');
  assert.deepEqual(C.paramDeTrecho(SQL, ca, ca + 2), erro);
  assert.deepEqual(C.paramDeTrecho(SQL, a - 2, b), erro);
  assert.deepEqual(C.paramDeTrecho(SQL, 'x', 3), erro);
});

test("paramDeTrecho recusa trecho que corta o '' de um texto escapado", () => {
  const s = "SELECT 1 FROM t WHERE n = 'D''AVILA'";
  const i = s.indexOf("D''");
  assert.equal(C.paramDeTrecho(s, i, i + 2).ok, false);
  assert.equal(C.paramDeTrecho(s, i + 2, i + 8).ok, false);
  assert.deepEqual(C.paramDeTrecho(s, i, i + 8).param.valor, "D'AVILA");
});

test('paramDeValor acha CURITIBA e recusa valor ausente', () => {
  const [a, b] = pos(SQL, 'CURITIBA');
  assert.deepEqual(C.paramDeValor(SQL, 'CURITIBA'), { ok:true, param:{ valor:'CURITIBA', tipo:'texto', inicio:a, fim:b } });
  const [d] = pos(SQL, '2026-11-01');
  assert.deepEqual(C.paramDeValor(SQL, '2026-11-01'), { ok:true, param:{ valor:'2026-11-01', tipo:'data', inicio:d, fim:d + 10 } });
  assert.deepEqual(C.paramDeValor(SQL, 'LONDRINA'), { ok:false, erro:'Esse valor não aparece na consulta.' });
  assert.deepEqual(C.paramDeValor(SQL, 'lojas'), { ok:false, erro:'Esse valor não aparece na consulta.' });
});

function consulta(){
  const p = C.acharLiterais(SQL);
  return { id:'c1', nome:'Vendas por loja', sql:SQL, visual:'barra', x:'loja', y:'total', criada:'2026-10-21',
    params:[
      { nome:'Data inicial', tipo:'data', inicio:p[0].inicio, fim:p[0].fim, valor:p[0].valor },
      { nome:'Data final', tipo:'data', inicio:p[1].inicio, fim:p[1].fim, valor:p[1].valor },
      { nome:'Status', tipo:'numero', inicio:p[2].inicio, fim:p[2].fim, valor:p[2].valor },
      { nome:'Cidade', tipo:'texto', inicio:p[3].inicio, fim:p[3].fim, valor:p[3].valor }
    ] };
}

test("montarSql escapa aspas: D'ÁVILA vira D''ÁVILA dentro do LIKE", () => {
  const r = C.montarSql(consulta(), ['2026-09-01', '2026-10-01', '3', "D'ÁVILA"]);
  assert.equal(r.ok, true);
  assert.equal(r.sql, SQL.replace("'2026-10-01'", "'2026-09-01'").replace("'2026-11-01'", "'2026-10-01'")
    .replace('<> 9', '<> 3').replace("'%CURITIBA%'", "'%D''ÁVILA%'"));
  assert.deepEqual(C.acharLiterais(r.sql).map(l => l.valor), ['2026-09-01', '2026-10-01', '3', "D'ÁVILA"]);
});

test('montarSql sem valor usa o valor salvo de cada parâmetro', () => {
  assert.deepEqual(C.montarSql(consulta(), []), { ok:true, sql:SQL });
});

test('montarSql recusa data e número inválidos com o nome do parâmetro', () => {
  assert.deepEqual(C.montarSql(consulta(), ['2026-02-30', '2026-10-01', '3', 'X']), { ok:false, erro:'Data inicial: data inválida.' });
  assert.deepEqual(C.montarSql(consulta(), ['01/09/2026', '2026-10-01', '3', 'X']), { ok:false, erro:'Data inicial: data inválida.' });
  assert.deepEqual(C.montarSql(consulta(), ['2026-09-01', '2026-10-01', '3 OR 1=1', 'X']), { ok:false, erro:'Status: número inválido.' });
  assert.deepEqual(C.montarSql(consulta(), ['2026-09-01', '2026-10-01', '-3', 'X']), { ok:false, erro:'Status: número inválido.' });
});

test('montarSql recusa texto com comentário, ponto e vírgula ou barra invertida', () => {
  for(const t of ['a -- b', 'a; DROP', 'x /* y', "a\\' OR 1=1", 'linha\nnova'])
    assert.deepEqual(C.montarSql(consulta(), ['2026-09-01', '2026-10-01', '3', t]), { ok:false, erro:'Cidade: texto inválido.' }, t);
});

test('montarSql recusa texto que encostaria num hífen e formaria --', () => {
  const s = "SELECT 1 FROM t WHERE c = 'A-B'";
  const i = s.indexOf('B');
  const r = C.montarSql({ sql:s, params:[{ nome:'Sufixo', tipo:'texto', inicio:i, fim:i + 1, valor:'B' }] }, ['-x']);
  assert.equal(r.ok, false);
});

test('montarSql com parâmetro malformado devolve ok:false sem lançar', () => {
  const c = consulta();
  const casos = [
    [{ nome:'P', tipo:'data', inicio:'a', fim:'b', valor:'2026-01-01' }],
    [{ nome:'P', tipo:'texto', inicio:-5, fim:2, valor:'x' }],
    [{ nome:'P', tipo:'texto', inicio:0, fim:SQL.length + 10, valor:'x' }],
    [{ nome:'P', tipo:'cor', inicio:c.params[3].inicio, fim:c.params[3].fim, valor:'x' }],
    [{ nome:'P', tipo:'texto', inicio:SQL.indexOf('lojas'), fim:SQL.indexOf('lojas') + 5, valor:'x' }],
    [{ nome:'P', tipo:'numero', inicio:c.params[3].inicio, fim:c.params[3].fim, valor:'1' }],
    [c.params[3], { nome:'Q', tipo:'texto', inicio:c.params[3].inicio + 2, fim:c.params[3].fim, valor:'x' }],
    [null],
    'não é lista'
  ];
  for(const params of casos){
    let r;
    assert.doesNotThrow(() => { r = C.montarSql({ sql:SQL, params }, ['x', 'y']); });
    if(Array.isArray(params)){ assert.equal(r.ok, false, JSON.stringify(params)); assert.match(r.erro, /parâmetro inválido/); }
    else assert.deepEqual(r, { ok:true, sql:SQL });
  }
  assert.deepEqual(C.montarSql({ sql:SQL, params:[{ tipo:'cor', inicio:1, fim:2 }] }, []), { ok:false, erro:'Parâmetro 1: parâmetro inválido.' });
  assert.equal(C.montarSql(null, []).ok, false);
  assert.equal(C.montarSql({ sql:42 }, []).ok, false);
});
