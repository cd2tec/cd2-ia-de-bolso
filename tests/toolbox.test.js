'use strict';
// Testes da lógica pura do modo "MCP Nativo para Banco de dados". Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const T = require('../web/toolbox.js');

test('urlFor: porta do harness + 1000, sempre 127.0.0.1', () => {
  assert.equal(T.urlFor({ protocol:'http:', hostname:'localhost', port:'8080' }), 'http://127.0.0.1:9080/mcp');
  assert.equal(T.urlFor({ protocol:'http:', hostname:'127.0.0.1', port:'8093' }), 'http://127.0.0.1:9093/mcp');
  assert.equal(T.urlFor({ protocol:'http:', hostname:'127.0.0.1', port:'' }), 'http://127.0.0.1:1080/mcp');
});

test('engineLabel: rótulos e fallback', () => {
  assert.equal(T.engineLabel('postgres'), 'PostgreSQL');
  assert.equal(T.engineLabel('mariadb'), 'MariaDB / MySQL');
  assert.equal(T.engineLabel('xpto'), 'Banco de dados');
  assert.equal(T.ENGINES.oracle.port, 1521);
});

test('guardSql aceita SELECT e WITH, com espaço, comentário e caixa baixa', () => {
  assert.deepEqual(T.guardSql('select 1'), { ok:true, sql:'select 1' });
  assert.deepEqual(T.guardSql('  WITH x AS (SELECT 1 a) SELECT a FROM x'), { ok:true, sql:'WITH x AS (SELECT 1 a) SELECT a FROM x' });
  assert.deepEqual(T.guardSql('-- total\nSELECT count(*) FROM produto'), { ok:true, sql:'SELECT count(*) FROM produto' });
  assert.deepEqual(T.guardSql('/* a */ SELECT 1'), { ok:true, sql:'SELECT 1' });
});

test('guardSql tira um ponto e vírgula final, recusa no meio', () => {
  assert.deepEqual(T.guardSql('SELECT 1;'), { ok:true, sql:'SELECT 1' });
  assert.deepEqual(T.guardSql('SELECT 1; \n'), { ok:true, sql:'SELECT 1' });
  const r = T.guardSql('select 1) q; delete from produto; select (1');
  assert.equal(r.ok, false); assert.match(r.error, /ponto e vírgula/);
});

test('guardSql recusa DML, DDL e vazio com mensagem para o modelo', () => {
  for(const s of ['DELETE FROM produto', 'update produto set x=1', 'DROP TABLE produto', 'EXEC xp_cmdshell', '', '   ', 'SELECT 1 INTO novo FROM t']){
    const r = T.guardSql(s); assert.equal(r.ok, false, s); assert.match(r.error, /só SELECT/i);
  }
  assert.equal(T.guardSql('SELECT update_date FROM t').ok, true);
  assert.equal(T.guardSql(undefined).ok, false);
});

test('guardSql recusa lote T-SQL sem ponto e vírgula e funções com efeito colateral', () => {
  const casos = [
    ['select 1 a) q shutdown with nowait select * from (select 1 b', 'SHUTDOWN'],
    ['SELECT 1 SHUTDOWN', 'SHUTDOWN'],
    ['select 1 a) q waitfor delay \'00:00:10\' select * from (select 1 b', 'WAITFOR'],
    ['select 1 a) q declare @x int set @x = 1 select * from (select 1 b', 'DECLARE'],
    ['select * from openrowset(\'SQLNCLI\', \'x\', \'select 1\')', 'OPENROWSET'],
    ['select pg_terminate_backend(123)', 'PG_TERMINATE_BACKEND'],
    ['SELECT set_config(\'default_transaction_read_only\', \'off\', false)', 'SET_CONFIG'],
    ['select pg_sleep(10)', 'PG_SLEEP'],
    ['select pg_sleep_for(\'10 seconds\')', 'PG_SLEEP_FOR'],
    ['select * from dblink_exec(\'x\', \'delete\')', 'DBLINK_EXEC'],
    ['select nextval(\'s\')', 'NEXTVAL']
  ];
  for(const [s, palavra] of casos){
    const r = T.guardSql(s); assert.equal(r.ok, false, s); assert.match(r.error, new RegExp('palavra ' + palavra), s);
  }
});

test('guardSql não barra nomes que só contêm as palavras proibidas', () => {
  for(const s of [
    'select offset_x from t',
    'select use_count from t',
    'select * from t order by id offset 10 rows fetch next 5 rows only',
    'select settings from t',
    'select copy_count, kill_ratio, backup_date from t',
    'SELECT asset, reset_at FROM t'
  ]){
    assert.equal(T.guardSql(s).ok, true, s);
  }
});

test('parseStatus lê o JSON do toolbox (array com uma linha) e tolera lixo', () => {
  const txt = '[{"engine":"postgres","db_version":"PostgreSQL 16.3 on x86_64\\nmore","host":"db.local","database":"cd2","user":"leitor","schema":"public","visible_tables":42}]';
  const s = T.parseStatus(txt);
  assert.equal(s.engine, 'postgres'); assert.equal(s.visible_tables, 42);
  assert.equal(T.parseStatus('ERRO: x'), null);
  assert.equal(T.parseStatus('[]'), null);
});

test('statusLines: ordem fixa, versão só na primeira linha, tabelas como texto', () => {
  const lines = T.statusLines({ engine:'sqlserver', db_version:'Microsoft SQL Server 2022\n\tCopyright', host:'h', database:'d', user:'u', schema:'dbo', visible_tables:7 });
  assert.deepEqual(lines, [['Motor','SQL Server'],['Versão','Microsoft SQL Server 2022'],['Host','h'],['Base','d'],['Usuário','u'],['Schema','dbo'],['Tabelas visíveis','7']]);
});

test('TOOL_DESCRIPTIONS cobre as sete ferramentas', () => {
  assert.deepEqual(Object.keys(T.TOOL_DESCRIPTIONS).sort(), ['db_catalog','db_connection_status','db_describe_table','db_list_tables','db_read_only_query','db_sample_table','db_search_columns']);
});

/* ---------- cd2-ctl (controle do banco pela tela) ---------- */
const K = 'a'.repeat(32) + '0123456789abcdef0123456789ABCDEF';

test('ctlUrlFor: porta do harness + 3000, sempre 127.0.0.1, sem barra no fim', () => {
  assert.equal(T.ctlUrlFor({ port:'8080' }), 'http://127.0.0.1:11080');
  assert.equal(T.ctlUrlFor({ hostname:'localhost', port:'8093' }), 'http://127.0.0.1:11093');
  assert.equal(T.ctlUrlFor({ port:'' }), 'http://127.0.0.1:3080');
});

test('trio de portas P, P+1000, P+3000 nunca cai em porta bloqueada pelos navegadores (P de 8080 a 8100)', () => {
  for (let p = 8080; p <= 8100; p++) {
    for (const q of [p, p + 1000, p + 3000]) {
      assert.ok(!T.UNSAFE_PORTS.includes(q), 'porta bloqueada: ' + q + ' (P=' + p + ')');
    }
  }
});

test('readKeyFromHash: só k= com 64 hexadecimais; hash vazio não é chave', () => {
  assert.equal(K.length, 64);
  assert.equal(T.readKeyFromHash('#k=' + K), K);
  assert.equal(T.readKeyFromHash('#x=1&k=' + K), K);
  assert.equal(T.readKeyFromHash('#k=' + K + '&y=2'), K);
  for(const h of ['', '#', '#k=', '#k=' + K.slice(1), '#k=' + K + 'f', '#k=' + K.slice(1) + 'g', '#kk=' + K, '?k=' + K, null, undefined]){
    assert.equal(T.readKeyFromHash(h), null, String(h));
  }
});

test('defaultsFor: porta e schema padrão por motor (mesmas regras do cd2-ctl)', () => {
  assert.deepEqual(T.defaultsFor('postgres', 'u', 'b'), { porta:5432, schema:'public' });
  assert.deepEqual(T.defaultsFor('oracle', 'leitor', 'FREEPDB1'), { porta:1521, schema:'LEITOR' });
  assert.deepEqual(T.defaultsFor('mariadb', 'u', 'loja'), { porta:3306, schema:'loja' });
  assert.deepEqual(T.defaultsFor('sqlserver', 'u', 'b'), { porta:1433, schema:'dbo' });
  assert.deepEqual(T.defaultsFor('firebird', 'LEITURA_RO', 'C:\\dados\\erp.fdb'), { porta:3050, schema:'' });
  assert.deepEqual(T.defaultsFor('dbf', '', 'C:\\dados\\loja'), { porta:0, schema:'' });
  assert.deepEqual(T.defaultsFor('oracle', '', ''), { porta:1521, schema:'' });
  assert.deepEqual(T.defaultsFor('xpto', 'u', 'b'), { porta:'', schema:'' });
});

test('configPayload: apara, porta numérica, porta vazia e senha vazia somem', () => {
  assert.deepEqual(
    T.configPayload({ motor:'postgres', host:' db.local ', porta:' 5432 ', base:' cd2', usuario:'leitor ', senha:'', schema:'' }),
    { motor:'postgres', host:'db.local', porta:5432, base:'cd2', usuario:'leitor', schema:'' });
  assert.deepEqual(
    T.configPayload({ motor:'oracle', host:'h', porta:'', base:'s', usuario:'u', senha:' com espaço ', schema:'X' }),
    { motor:'oracle', host:'h', base:'s', usuario:'u', schema:'X', senha:' com espaço ' });
  assert.equal(T.configPayload({ motor:'postgres', porta:'54a' }).porta, '54a');
  assert.equal('senha' in T.configPayload({ motor:'postgres' }), false);
  assert.equal(T.configPayload({ motor:'postgres', senha:"cd2 s3nh@'x" }).senha, "cd2 s3nh@'x");
});

test('ctlStatusLines: campos do /status, sem senha', () => {
  const l = T.ctlStatusLines({ estado:'rodando', motor:'oracle', host:'h', porta:1521, base:'FREEPDB1', usuario:'leitor', schema:'LEITOR', senha_salva:true });
  assert.deepEqual(l, [['Motor','Oracle'],['Host','h'],['Porta','1521'],['Base','FREEPDB1'],['Usuário','leitor'],['Schema','LEITOR']]);
});

test('prepareSql: SQL Server ganha OFFSET 0 ROWS quando tem ORDER BY sem TOP nem OFFSET', () => {
  assert.equal(T.prepareSql('SELECT mes, SUM(v) AS total FROM venda GROUP BY mes ORDER BY mes', 'sqlserver'),
    'SELECT mes, SUM(v) AS total FROM venda GROUP BY mes ORDER BY mes OFFSET 0 ROWS');
  assert.equal(T.prepareSql('SELECT TOP (10) nome FROM p ORDER BY nome', 'sqlserver'), 'SELECT TOP (10) nome FROM p ORDER BY nome');
  assert.equal(T.prepareSql('SELECT nome FROM p ORDER BY nome OFFSET 5 ROWS', 'sqlserver'), 'SELECT nome FROM p ORDER BY nome OFFSET 5 ROWS');
  assert.equal(T.prepareSql('SELECT count(*) AS n FROM p', 'sqlserver'), 'SELECT count(*) AS n FROM p');
});

test('prepareSql: outros motores passam intactos', () => {
  for(const m of ['postgres', 'mariadb', 'oracle', undefined])
    assert.equal(T.prepareSql('SELECT a FROM t ORDER BY a', m), 'SELECT a FROM t ORDER BY a');
});

test('prepareSql: so enxerga ORDER BY, TOP e OFFSET no nivel externo', () => {
  const ss = s => T.prepareSql(s, 'sqlserver');
  for(const s of [
    'SELECT mes, ROW_NUMBER() OVER (ORDER BY total) AS rn FROM t',
    "SELECT STRING_AGG(x, ',') WITHIN GROUP (ORDER BY x) AS l FROM t",
    "SELECT 'order by' AS s FROM t",
    'SELECT [order by] FROM t',
    'SELECT "order by" FROM t'
  ]) assert.equal(ss(s), s);
  for(const s of [
    'SELECT * FROM (SELECT TOP 5 a, b FROM t ORDER BY a) x ORDER BY b',
    'SELECT a, ROW_NUMBER() OVER (ORDER BY a) rn FROM t ORDER BY a',
    "SELECT 'it''s (' AS s FROM t ORDER BY s"
  ]) assert.equal(ss(s), s + ' OFFSET 0 ROWS');
});

test('placeholdersFor: exemplos genéricos por motor, sem credencial', () => {
  const pg = T.placeholdersFor('postgres', '', '');
  assert.match(pg.host, /^ex\.: /); assert.equal(pg.base, 'ex.: erp'); assert.equal(pg.schema, 'ex.: public');
  assert.equal(pg.senha, 'Senha da conta de leitura');
  assert.equal(T.placeholdersFor('oracle', '', '').base, 'ex.: ORCLPDB1');
  assert.equal(T.placeholdersFor('oracle', '', '').schema, 'ex.: LEITURA_RO');
  assert.equal(T.placeholdersFor('oracle', 'leitor', '').schema, 'ex.: LEITOR');
  assert.equal(T.placeholdersFor('mariadb', '', 'loja').schema, 'ex.: loja');
  assert.equal(T.placeholdersFor('sqlserver', '', '').schema, 'ex.: dbo');
  assert.equal(T.placeholdersFor('firebird', '', '').base, 'ex.: C:\\dados\\erp.fdb');
  assert.equal(T.placeholdersFor('firebird', '', '').schema, 'ex.: nao usado');
  assert.equal(T.placeholdersFor('dbf', '', '').base, 'ex.: C:\\dados\\loja');
  assert.equal(T.placeholdersFor('dbf', '', '').schema, 'ex.: nao usado');
  for(const m of Object.keys(T.ENGINES)){
    const p = T.placeholdersFor(m, '', '');
    for(const k of ['host', 'base', 'usuario', 'schema']) assert.match(p[k], /^ex\.: /, m + '.' + k);
  }
  assert.deepEqual(T.placeholdersFor('xpto', '', ''), { host:'', base:'', usuario:'', senha:'', schema:'' });
});

test('portFor: porta padrão do motor, sem sobrescrever porta digitada', () => {
  assert.equal(T.portFor('postgres', ''), '5432');
  assert.equal(T.portFor('oracle', '5432'), '1521');
  assert.equal(T.portFor('mariadb', ' 1521 '), '3306');
  assert.equal(T.portFor('sqlserver', '3306'), '1433');
  assert.equal(T.portFor('sqlserver', '14330'), '14330');
  assert.equal(T.portFor('xpto', '5432'), '5432');
});

// View não tem contagem de linhas (entra com 0). Ordenar só por linhas e cortar em 300 deixava as
// views de fora em banco com muitas tabelas vazias, e são elas que alguém montou para relatório.
test('db_catalog: nos motores, views vêm antes da ordem por linhas', () => {
  const fs = require('node:fs'), path = require('node:path');
  for(const motor of ['postgres', 'mariadb', 'sqlserver', 'oracle', 'firebird', 'dbf']){
    const y = fs.readFileSync(path.join(__dirname, '..', 'core', 'toolbox', motor + '.yaml'), 'utf8');
    const bloco = y.slice(y.indexOf('name: db_catalog'), y.indexOf('---', y.indexOf('name: db_catalog')));
    const ordens = [...bloco.matchAll(/ORDER BY ([^\n]+)/g)].map(m => m[1]);
    assert.ok(ordens.length >= 1, motor);
    assert.match(ordens[0], /^CASE WHEN .*(VIEW|'v'|'V').* THEN 0 ELSE 1 END, /, motor + ': ' + ordens[0]);
  }
});
