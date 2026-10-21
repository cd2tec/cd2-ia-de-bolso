'use strict';
// Testes do CSV e do relatório impresso (web/report.js). Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const R = require('../web/report.js');

test('toCsv: BOM, ponto e vírgula, CRLF, aspas e decimal com vírgula', () => {
  const s = R.toCsv(['loja', 'total', 'obs'], [{ loja:'A;B', total:'12.5', obs:'diz "oi"' }, { loja:'C', total:3, obs:null }]);
  assert.ok(s.startsWith('\uFEFF'));
  assert.equal(s.slice(1), 'loja;total;obs\r\n"A;B";12,5;"diz ""oi"""\r\nC;3;\r\n');
});

test('toCsv neutraliza fórmula do Excel e preserva número negativo e código', () => {
  const s = R.toCsv(['a'], [{ a:'=HYPERLINK("x")' }, { a:'+1' }, { a:'@SUM' }, { a:'-texto' }, { a:'-5' }, { a:-2.5 }, { a:'0012345' }]);
  assert.deepEqual(s.slice(1).split('\r\n').slice(1, 8), ['"\'=HYPERLINK(""x"")"', "'+1", "'@SUM", "'-texto", '-5', '-2,5', '0012345']);
});

test('fileName com data e hora', () => {
  assert.equal(R.fileName(new Date(2026, 9, 13, 9, 5)), 'cd2-relatorio-20261013-0905.csv');
});

test('printHtml escapa o texto, mantém o HTML do gráfico e mostra o contato', () => {
  const h = R.printHtml({ pergunta:'<script>x</script>', titulo:'Vendas & cia', frase:'Maior: A.', aviso:'Mostrando 20.', visualHtml:'<svg class="chart"></svg>',
    tabelaHtml:'<div class="tbl"></div>', sql:'SELECT a < b', quando:'13/10/2026 09:05', motor:'PostgreSQL', base:'erp', contato:'cd2.com.br · (11) 0000-0000', logoSvg:'<svg class="logo"></svg>' });
  assert.ok(!h.includes('<script>')); assert.ok(h.includes('&lt;script&gt;')); assert.ok(h.includes('Vendas &amp; cia'));
  assert.ok(h.includes('<svg class="chart"></svg>')); assert.ok(h.includes('SELECT a &lt; b'));
  assert.ok(h.includes('cd2.com.br · (11) 0000-0000')); assert.ok(h.includes('Mostrando 20.'));
  assert.ok(h.includes('PostgreSQL')); assert.ok(h.includes('erp'));
  const sem = R.printHtml({ pergunta:'p', titulo:'t', frase:'f', visualHtml:'', tabelaHtml:'', sql:'s', quando:'q', motor:'m', base:'', contato:'', logoSvg:'' });
  assert.ok(sem.includes('CD2 Tech</footer>') || /CD2 Tech\s*<\/span>\s*<\/footer>/.test(sem));
  assert.ok(!sem.includes('undefined'));
});

test('toCsv nao deixa fórmula escapar por espaço, tabulação, CR ou número seguido de texto', () => {
  const casos = [
    ['-1+1', "'-1+1"],
    ["-1+cmd|' /C calc'!A0", "'-1+cmd|' /C calc'!A0"],
    ['\t=1', "'\t=1"],
    [' =1', "' =1"],
    ['\r=1', "\"'\r=1\""],
  ];
  for (const [entrada, esperado] of casos) {
    const linha = R.toCsv(['a'], [{ a: entrada }]).slice(1).split('\r\n')[1];
    assert.equal(linha, esperado, 'entrada ' + JSON.stringify(entrada));
  }
});

// Relatório "o que o harness entende do banco": tabelas agrupadas pelo papel do perfil.
const TABS = [
  { name:'tab_preco', type:'TABLE', rows:169067, perfil:{ papel:'cadastro', colunas:{} } },
  { name:'view_resumo_venda', type:'VIEW', rows:0, perfil:{ papel:'movimento', colunas:{ data:'dta_movimento', valor:'vlr_total_venda' }, descricao:'Resumo de cada venda <PDV>.' } },
  { name:'aux_tmp', type:'TABLE', rows:5000, perfil:{ papel:'vazia', colunas:{} } },
  { name:'cfg', type:'TABLE', rows:3, perfil:{ papel:'parametro', colunas:{} } },
  { name:'mov_item', type:'TABLE', rows:900, perfil:{ papel:'itens', colunas:{ quantidade:'qtd', valor:'vlr_it' } } }
];
const BASE = { motor:'SQL Server', base:'ZeusManager', schema:'dbo', quando:'16/10/2026 10:00', contato:'cd2.com.br', logoSvg:'<svg id="lg"></svg>' };

test('perfilHtml: resumo, grupos na ordem dos papéis, colunas úteis, view, linhas e descrição escapada', () => {
  const h = R.perfilHtml(Object.assign({ tabelas:TABS }, BASE));
  for(const s of ['ZeusManager', 'dbo', 'SQL Server', '16/10/2026 10:00', '<svg id="lg"></svg>', 'cd2.com.br']) assert.ok(h.includes(s), s);
  assert.match(h, /5 tabelas/);
  const ordem = ['Movimento', 'Itens de movimento', 'Cadastro', 'Parâmetro', 'Vazias'].map(t => h.indexOf('<h2>' + t));
  assert.ok(ordem.every((p, i) => p > 0 && (i === 0 || p > ordem[i - 1])), JSON.stringify(ordem));
  assert.match(h, /view_resumo_venda[\s\S]*view[\s\S]*data: dta_movimento; valor: vlr_total_venda/);
  assert.match(h, /Resumo de cada venda &lt;PDV&gt;\./);
  assert.match(h, /cerca de 169\.067 linhas/);
  assert.doesNotMatch(h, /<h2>Indefinida/);
  assert.match(h.slice(h.indexOf('<h2>Vazias')), /aux_tmp/);
});

test('perfilHtml: aviso enquanto o modelo descreve; sem aviso quando terminou; nome escapado', () => {
  assert.match(R.perfilHtml(Object.assign({ tabelas:TABS, progresso:{ feitos:87, total:232 } }, BASE)), /87 de 232 tabelas descritas até agora/);
  assert.doesNotMatch(R.perfilHtml(Object.assign({ tabelas:TABS, progresso:{ feitos:232, total:232 } }, BASE)), /descritas até agora/);
  const h = R.perfilHtml(Object.assign({ tabelas:[{ name:'<script>x', type:'TABLE', rows:1, perfil:{ papel:'cadastro', colunas:{} } }] }, BASE));
  assert.ok(h.includes('&lt;script&gt;x')); assert.ok(!h.includes('<script>x'));
});

test('perfilHtml: sem análise ainda, avisa e não lista grupos', () => {
  const h = R.perfilHtml(Object.assign({ tabelas:[{ name:'a', type:'TABLE', rows:1 }] }, BASE));
  assert.match(h, /análise das tabelas ainda não terminou/);
  assert.doesNotMatch(h, /<h2>Cadastro/);
});

test('perfilHtml e printHtml: nome IA de Bolso, singular de linha, colunas úteis só em movimento e itens', () => {
  const h = R.perfilHtml(Object.assign({ tabelas:[
    { name:'pessoa', type:'TABLE', rows:1, perfil:{ papel:'cadastro', colunas:{ data:'DTANASCFUND', valor:'VLRALUGUEL' } } },
    { name:'titulo', type:'TABLE', rows:2, perfil:{ papel:'movimento', colunas:{ data:'DTAEMISSAO' } } }] }, BASE));
  assert.match(h, /O que a IA de Bolso entende deste banco/);
  assert.match(h, /cerca de 1 linha\)/); assert.match(h, /cerca de 2 linhas/);
  assert.doesNotMatch(h, /DTANASCFUND|VLRALUGUEL/); assert.match(h, /data: DTAEMISSAO/);
  assert.match(R.printHtml({ pergunta:'p', titulo:'t', frase:'f', sql:'s', quando:'q', motor:'m' }), /Relatório gerado pela IA de Bolso/);
});

test('perfilHtml mostra a nota da TI da tabela e dos campos', () => {
  const tabs = [{ name:'pedido', type:'TABLE', rows:10, nota:'Tabela oficial <de vendas>',
    columns:[{ name:'status', nota:'S = cancelado & estornado' }, { name:'id' }],
    perfil:{ papel:'movimento', colunas:{}, descricao:'Pedidos.' } }];
  const h = R.perfilHtml(Object.assign({ tabelas:tabs }, BASE));
  assert.ok(h.includes('Tabela oficial &lt;de vendas&gt;'));
  assert.ok(h.includes('status') && h.includes('S = cancelado &amp; estornado'));
  assert.ok(!h.includes('<de vendas>'));
  assert.ok(!/\bid:/.test(h));
});
