'use strict';
// Testes do perfil das tabelas (web/perfil.js). Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const C = require('../web/catalog.js');
const P = require('../web/perfil.js');

const cat = defs => C.buildCatalog(defs.flatMap(([nome, tipo, linhas, cols]) =>
  cols.map(([column_name, data_type]) => ({ table_name:nome, table_type:tipo, num_rows:linhas, column_name, data_type }))));
const gera = (n, f) => Array.from({ length:n }, (_, i) => f(i));
const papeis = c => Object.fromEntries(c.tables.map(t => [t.name, t.perfil.papel]));
const perfil = (c, n) => c.tables.find(t => t.name === n).perfil;

// Estilo 1: ERP em português com prefixos (o banco da avaliação atual).
const PT = () => cat([
  ['mov_venda', 'TABLE', 6000, [['nr_cupom','integer'],['cd_loja','integer'],['cd_cli','integer'],['dt_mov','date'],['vlr_tot','numeric(12,2)']]],
  ['mov_venda_item', 'TABLE', 18000, [['nr_cupom','integer'],['seq','integer'],['cd_prod','integer'],['qtd_it','integer'],['vlr_tot_it','numeric(12,2)']]],
  ['produto', 'TABLE', 120, [['cd_prod','integer'],['ds_prod','varchar(80)'],['cd_depto','integer'],['vlr_custo','numeric(10,2)']]],
  ['loja', 'TABLE', 4, [['cd_loja','integer'],['nm_loja','varchar(40)'],['uf','char(2)']]],
  ['cliente', 'TABLE', 300, [['cd_cli','integer'],['nm_cli','varchar(80)'],['uf','char(2)'],['dt_cad','date']]],
  ['cfg_parametro', 'TABLE', 2, [['chave','varchar(40)'],['valor','text']]],
  ['log_integracao', 'TABLE', 20000, [['id','bigint'],['dt_log','timestamp'],['msg','text']]],
  ['aux_tmp_import', 'TABLE', 0, [['linha','integer'],['conteudo','text']]]
]);
const AM_PT = new Map([
  ['mov_venda', gera(10, i => ({ nr_cupom:100 + i, cd_loja:1 + i % 4, cd_cli:i % 3, dt_mov:'2026-01-0' + (1 + i % 9), vlr_tot:10.5 * (i + 1) }))],
  ['mov_venda_item', gera(10, i => ({ nr_cupom:100 + (i >> 1), seq:1 + i % 2, cd_prod:7 + i, qtd_it:1 + i % 3, vlr_tot_it:3.2 * (i + 1) }))],
  ['produto', gera(10, i => ({ cd_prod:i + 1, ds_prod:'Produto ' + i, cd_depto:1 + i % 3, vlr_custo:2.5 + i }))],
  ['loja', gera(4, i => ({ cd_loja:i + 1, nm_loja:'Loja ' + i, uf:'SP' }))],
  ['cliente', gera(10, i => ({ cd_cli:i + 1, nm_cli:'Cliente ' + i, uf:'RJ', dt_cad:'2025-0' + (1 + i % 9) + '-01' }))],
  ['cfg_parametro', [{ chave:'versao', valor:'1.0' }, { chave:'moeda', valor:'BRL' }]],
  ['log_integracao', gera(10, i => ({ id:i + 1, dt_log:'2026-01-01 10:0' + i, msg:'evento ' + i }))],
  ['aux_tmp_import', []]
]);

test('perfil: ERP em português com prefixos', () => {
  const c = PT(); P.analisarCatalogo(c, AM_PT);
  assert.deepEqual(papeis(c), { mov_venda:'movimento', mov_venda_item:'itens', produto:'cadastro', loja:'cadastro', cliente:'cadastro',
    cfg_parametro:'parametro', log_integracao:'indefinida', aux_tmp_import:'vazia' });
  assert.deepEqual(perfil(c, 'mov_venda').colunas, { data:'dt_mov', valor:'vlr_tot' });
  assert.equal(perfil(c, 'mov_venda').chave, 'nr_cupom');
  assert.deepEqual(perfil(c, 'mov_venda_item').colunas, { valor:'vlr_tot_it', quantidade:'qtd_it' });
});

// Estilo 2: ERP em inglês sem prefixo, com estatística desatualizada e view sem contagem.
test('perfil: ERP em inglês sem prefixo', () => {
  const c = cat([
    ['sales_orders','TABLE',4000,[['id','integer'],['store_id','integer'],['customer_id','integer'],['order_date','date'],['total_amount','numeric(12,2)']]],
    ['order_lines','TABLE',8000,[['order_id','integer'],['line_no','integer'],['product_id','integer'],['quantity','integer'],['line_total','numeric(12,2)']]],
    ['products','TABLE',100,[['id','integer'],['name','varchar(80)'],['category_id','integer'],['created_at','timestamp'],['updated_at','timestamp']]],
    ['price_list','TABLE',400,[['product_id','integer'],['store_id','integer'],['unit_price','numeric(10,2)'],['created_at','timestamp'],['updated_at','timestamp'],['updated_by','varchar(30)']]],
    ['app_settings','TABLE',3,[['setting_key','varchar(40)'],['setting_value','text']]],
    ['audit_log','TABLE',30000,[['id','bigint'],['created_at','timestamp'],['user_name','varchar(30)'],['action','text']]],
    ['tmp_sync_orders','TABLE',1200,[['id','integer'],['payload','text']]],
    ['v_daily_sales','VIEW',0,[['sale_date','date'],['store','varchar(40)'],['total','numeric']]]
  ]);
  P.analisarCatalogo(c, new Map([
    ['sales_orders', gera(10, i => ({ id:i + 1, store_id:1 + i % 4, customer_id:i, order_date:'2026-02-1' + i, total_amount:20 + i }))],
    ['order_lines', gera(10, i => ({ order_id:1 + (i >> 1), line_no:1 + i % 2, product_id:3 + i, quantity:1 + i % 4, line_total:5 + i }))],
    ['products', gera(10, i => ({ id:i + 1, name:'Product ' + i, category_id:1 + i % 5, created_at:'2025-01-0' + (i % 9 + 1), updated_at:'2025-06-0' + (i % 9 + 1) }))],
    ['price_list', gera(10, i => ({ product_id:i + 1, store_id:1, unit_price:3 + i, created_at:'2025-01-01', updated_at:'2026-01-0' + (i % 9 + 1), updated_by:'admin' }))],
    ['app_settings', [{ setting_key:'currency', setting_value:'BRL' }, { setting_key:'version', setting_value:'2.3' }]],
    ['audit_log', gera(10, i => ({ id:i + 1, created_at:'2026-01-01 08:0' + i, user_name:'user' + i, action:'update' }))],
    ['tmp_sync_orders', []],
    ['v_daily_sales', gera(10, i => ({ sale_date:'2026-03-0' + (i % 9 + 1), store:'Downtown', total:100 + i }))]
  ]));
  assert.deepEqual(papeis(c), { sales_orders:'movimento', order_lines:'itens', products:'cadastro', price_list:'cadastro',
    app_settings:'parametro', audit_log:'cadastro', tmp_sync_orders:'vazia', v_daily_sales:'movimento' });
});

// Estilo 3: estruturas com o formato das do ZeusManager, com outros nomes (a regra lê a estrutura).
test('perfil: estruturas do caso real com outros nomes', () => {
  const c = cat([
    ['tb_preco_canal','TABLE',169067,[['cod_loja','int'],['controle','varchar(10)'],['cod_item','varchar(20)'],['cod_canal','int'],['usuario','varchar(20)'],
      ['data_cadastro','datetime'],['data_ultima_alteracao','datetime'],['preco_unitario','numeric'],['percentual_comissao','numeric']]],
    ['vw_vendas_dia','VIEW',0,[['tipo_registro','int'],['dta_movimento','datetime'],['cod_loja','int'],['num_operacao','int'],['hora_venda','int'],
      ['qtde_vendida','numeric'],['vlr_total_venda','numeric']]],
    ['tb_config_valor','TABLE',2576,[['arquivo','varchar(40)'],['chave_registro','int'],['posicao','int'],['cod_loja','int'],['valor','varchar(80)'],
      ['dth_inclusao','datetime'],['dth_alteracao','datetime'],['idt_usuario','varchar(20)']]]
  ]);
  P.analisarCatalogo(c, new Map([
    ['tb_preco_canal', gera(10, i => ({ cod_loja:1, controle:'A', cod_item:'M' + i, cod_canal:i % 3, usuario:'adm', data_cadastro:'2024-05-0' + (i % 9 + 1),
      data_ultima_alteracao:'2026-01-0' + (i % 9 + 1), preco_unitario:4.5 + i, percentual_comissao:0 }))],
    ['vw_vendas_dia', gera(10, i => ({ tipo_registro:1, dta_movimento:'2026-03-0' + (i % 9 + 1), cod_loja:1 + i % 2, num_operacao:500 + i, hora_venda:900 + i,
      qtde_vendida:1 + i, vlr_total_venda:30 + i }))],
    ['tb_config_valor', gera(10, i => ({ arquivo:'PDV', chave_registro:1, posicao:i, cod_loja:1, valor:'S', dth_inclusao:'2023-01-01', dth_alteracao:'2025-0' + (i % 9 + 1) + '-01', idt_usuario:'adm' }))]
  ]));
  assert.deepEqual(papeis(c), { tb_preco_canal:'cadastro', vw_vendas_dia:'movimento', tb_config_valor:'parametro' });
  assert.deepEqual(perfil(c, 'vw_vendas_dia').colunas, { data:'dta_movimento', valor:'vlr_total_venda', quantidade:'qtde_vendida' });
});

test('perfil: sem amostra o papel sai das colunas; empate fica indefinido', () => {
  const c = cat([
    ['sales_orders','TABLE',4000,[['id','integer'],['order_date','date'],['total_amount','numeric']]],
    ['misc','TABLE',10,[['a','integer'],['b','text']]]
  ]);
  const so = P.analisarTabela(c.tables[0], null);
  assert.equal(so.papel, 'movimento'); assert.equal(so.amostra, false);
  assert.equal(P.analisarTabela(c.tables[1], [{ a:1, b:'x' }, { a:2, b:'y' }]).papel, 'indefinida');
});

test('perfil: toda amostra falhando não quebra; itens ainda são reconhecidos pela chave', () => {
  const c = PT();
  const r = P.analisarCatalogo(c, new Map());
  assert.equal(perfil(c, 'mov_venda').papel, 'movimento'); assert.equal(perfil(c, 'mov_venda_item').papel, 'itens');
  assert.equal(perfil(c, 'aux_tmp_import').papel, 'indefinida');
  assert.equal(r.vazia, 0); assert.equal(Object.values(r).reduce((a, b) => a + b, 0), c.tables.length);
});

test('perfil: lê a amostra sem diferenciar caixa dos nomes de coluna', () => {
  const c = cat([['MOV_VENDA','TABLE',10,[['NR_CUPOM','NUMBER'],['DT_MOV','DATE'],['VLR_TOT','NUMBER']]]]);
  const p = P.analisarTabela(c.tables[0], gera(5, i => ({ nr_cupom:i, dt_mov:'2026-01-0' + (i + 1), vlr_tot:i * 2 })));
  assert.equal(p.papel, 'movimento'); assert.equal(p.chave, 'NR_CUPOM');
});

// Correção 1: item só com preço unitário e quantidade; data de atributo não é data de movimento.
test('perfil: item com preço unitário e quantidade, sem total, liga ao pedido e vira itens', () => {
  const c = cat([
    ['pedido','TABLE',500,[['id','integer'],['order_date','date'],['total_amount','numeric(12,2)']]],
    ['pedido_item','TABLE',1500,[['pedido_id','integer'],['produto_id','integer'],['quantity','integer'],['unit_price','numeric(10,2)']]]
  ]);
  P.analisarCatalogo(c, new Map([
    ['pedido', gera(8, i => ({ id:i + 1, order_date:'2026-04-0' + (i + 1), total_amount:50 + i }))],
    ['pedido_item', gera(8, i => ({ pedido_id:1 + (i >> 1), produto_id:10 + i, quantity:1 + i % 3, unit_price:4.5 + i }))]
  ]));
  assert.deepEqual(papeis(c), { pedido:'movimento', pedido_item:'itens' });
});

test('perfil: tabela com quantidade e sem descrição não é cadastro', () => {
  const c = cat([['nota_fiscal_item','TABLE',900,[['numero','integer'],['serie','integer'],['item','integer'],['cd_prod','integer'],['qtd','integer'],['vl_unit','numeric(10,2)']]]]);
  P.analisarCatalogo(c, new Map([['nota_fiscal_item', gera(8, i => ({ numero:1 + (i >> 1), serie:1, item:1 + i % 2, cd_prod:5 + i, qtd:1 + i % 4, vl_unit:2.5 + i }))]]));
  assert.equal(perfil(c, 'nota_fiscal_item').papel, 'indefinida');
});

test('perfil: produto com estoque e preço continua cadastro (tem descrição)', () => {
  const c = cat([['produto','TABLE',200,[['cd_prod','integer'],['ds_prod','varchar(80)'],['qtd_estoque','integer'],['vlr_preco','numeric(10,2)']]]]);
  P.analisarCatalogo(c, new Map([['produto', gera(8, i => ({ cd_prod:i + 1, ds_prod:'Produto ' + i, qtd_estoque:10 + i, vlr_preco:3 + i }))]]));
  assert.equal(perfil(c, 'produto').papel, 'cadastro');
});

test('perfil: data de validade não faz o cadastro virar movimento', () => {
  const c = cat([['produto','TABLE',200,[['id','integer'],['codigo_barras','varchar(20)'],['dt_validade','date'],['custo_total','numeric(10,2)']]]]);
  P.analisarCatalogo(c, new Map([['produto', gera(8, i => ({ id:i + 1, codigo_barras:'789' + i, dt_validade:'2027-01-0' + (i + 1), custo_total:5 + i }))]]));
  assert.notEqual(perfil(c, 'produto').papel, 'movimento');
});

test('perfil: funcionário com nascimento e admissão é cadastro', () => {
  const c = cat([['funcionario','TABLE',60,[['id','integer'],['nome','varchar(80)'],['dt_nascimento','date'],['dt_admissao','date'],['salario','numeric(10,2)']]]]);
  P.analisarCatalogo(c, new Map([['funcionario', gera(8, i => ({ id:i + 1, nome:'Pessoa ' + i, dt_nascimento:'199' + i + '-03-01', dt_admissao:'2020-0' + (i + 1) + '-10', salario:2000 + i }))]]));
  assert.equal(perfil(c, 'funcionario').papel, 'cadastro');
});

test('amostrar: no máximo 2 tabelas ao mesmo tempo, views depois e uma por vez, até 30 views', async () => {
  const c = cat([...gera(5, i => ['t' + i, 'TABLE', 10, [['a','int']]]), ...gera(33, i => ['v' + i, 'VIEW', 0, [['a','int']]])]);
  let ativos = 0, pico = 0, picoViews = 0; const ordem = [], prog = [];
  const sample = async nome => {
    ativos++; pico = Math.max(pico, ativos); if(nome[0] === 'v') picoViews = Math.max(picoViews, ativos); ordem.push(nome);
    await new Promise(r => setTimeout(r, 5)); ativos--; return [{ a:1 }];
  };
  const m = await P.amostrar(c, sample, { onProgresso:(n, t) => prog.push(n + '/' + t) });
  assert.ok(pico <= 2, 'pico ' + pico); assert.equal(picoViews, 1);
  assert.ok(ordem.slice(0, 5).every(n => n[0] === 't'), ordem.join(','));
  assert.equal(ordem.length, 35);
  assert.equal(m.get('v32'), null); assert.deepEqual(m.get('t0'), [{ a:1 }]);
  assert.equal(prog[prog.length - 1], '38/38');
});

test('amostrar: falha vira null, resposta que não é lista vira null, corta em 10 linhas, para quando não está mais vivo', async () => {
  const c = cat([['a','TABLE',1,[['x','int']]], ['b','TABLE',1,[['x','int']]], ['c','TABLE',1,[['x','int']]], ['d','TABLE',1,[['x','int']]]]);
  const m = await P.amostrar(c, async n => { if(n === 'a') throw new Error('sem permissão'); if(n === 'b') return 'lixo'; return gera(15, i => ({ x:i })); });
  assert.equal(m.get('a'), null); assert.equal(m.get('b'), null); assert.equal(m.get('c').length, 10);
  let vivo = true; const chamadas = [];
  const m2 = await P.amostrar(c, async n => { chamadas.push(n); vivo = false; return [{ x:1 }]; }, { vivo:() => vivo, paralelo:1 });
  assert.deepEqual(chamadas, ['a']); assert.equal(m2.size, 0);
});

// Proteção do banco: o prazo fica dentro de amostrar e uma view lenta para as views seguintes.
test('amostrar: view que estoura o prazo para as views seguintes, que ficam null sem chamar o banco', async () => {
  const c = cat([['t0','TABLE',1,[['a','int']]], ...gera(4, i => ['v' + i, 'VIEW', 0, [['a','int']]])]);
  const chamadas = [], prog = [];
  const m = await P.amostrar(c, nome => { chamadas.push(nome); return nome === 'v0' ? new Promise(() => {}) : Promise.resolve([{ a:1 }]); },
    { prazo:20, onProgresso:(n, t) => prog.push(n + '/' + t) });
  assert.deepEqual(chamadas, ['t0', 'v0']);
  for(const v of ['v0', 'v1', 'v2', 'v3']) assert.equal(m.get(v), null, v);
  assert.deepEqual(m.get('t0'), [{ a:1 }]);
  assert.equal(prog[prog.length - 1], '5/5');
});

test('amostrar: depois de 3 tabelas estourando o prazo, as não iniciadas ficam null sem chamar o banco', async () => {
  const c = cat(gera(6, i => ['t' + i, 'TABLE', 1, [['a','int']]]));
  const chamadas = [], prog = [];
  const m = await P.amostrar(c, nome => { chamadas.push(nome); return new Promise(() => {}); },
    { prazo:20, paralelo:1, onProgresso:(n, t) => prog.push(n + '/' + t) });
  assert.deepEqual(chamadas, ['t0', 't1', 't2']);
  assert.equal(m.size, 6); assert.ok([...m.values()].every(v => v === null));
  assert.equal(prog[prog.length - 1], '6/6');
});

test('amostrar: amostra rápida limpa o timer do prazo e não deixa nada pendurado', async () => {
  const c = cat([['a','TABLE',1,[['x','int']]], ['b','TABLE',1,[['x','int']]], ['v','VIEW',0,[['x','int']]]]);
  const st = globalThis.setTimeout, ct = globalThis.clearTimeout, vivos = new Set(); let criados = 0;
  globalThis.setTimeout = (f, ms, ...a) => { const h = st(() => { vivos.delete(h); f(...a); }, ms); if(ms === 60000){ criados++; vivos.add(h); } return h; };
  globalThis.clearTimeout = h => { vivos.delete(h); ct(h); };
  const t0 = Date.now();
  try{ await P.amostrar(c, async () => [{ x:1 }], { prazo:60000 }); }
  finally{ globalThis.setTimeout = st; globalThis.clearTimeout = ct; }
  assert.equal(criados, 3); assert.equal(vivos.size, 0);
  assert.ok(Date.now() - t0 < 1000);
});

// Estilo 4: prefixos colados no nome (Consinco e parecidos).
test('perfil: ERP com prefixos colados no nome', () => {
  const c = cat([
    ['FI_TITULO','TABLE',1661,[['SEQTITULO','NUMBER'],['NROEMPRESA','NUMBER'],['SEQPESSOA','NUMBER'],['DTAEMISSAO','DATE'],['DTAVENCIMENTO','DATE'],['VLRORIGINAL','NUMBER']]],
    ['GE_PESSOA','TABLE',400,[['SEQPESSOA','NUMBER'],['NOMERAZAO','VARCHAR2'],['FANTASIA','VARCHAR2'],['DTAALTERACAO','DATE'],['USUALTERACAO','VARCHAR2']]],
    ['MAP_PRODUTO','TABLE',900,[['SEQPRODUTO','NUMBER'],['DESCCOMPLETA','VARCHAR2'],['SEQFAMILIA','NUMBER']]],
    ['MFL_DOCTOFISCAL','TABLE',5000,[['SEQNF','NUMBER'],['NROEMPRESA','NUMBER'],['DTAEMISSAO','DATE'],['VLRTOTAL','NUMBER']]],
    ['MFL_DFITEM','TABLE',15000,[['SEQNF','NUMBER'],['SEQPRODUTO','NUMBER'],['QUANTIDADE','NUMBER'],['VLRITEM','NUMBER']]]
  ]);
  P.analisarCatalogo(c, new Map([
    ['FI_TITULO', gera(10, i => ({ SEQTITULO:100 + i, NROEMPRESA:1, SEQPESSOA:i % 4, DTAEMISSAO:'2026-01-0' + (i % 9 + 1), DTAVENCIMENTO:'2026-02-0' + (i % 9 + 1), VLRORIGINAL:50 + i }))],
    ['GE_PESSOA', gera(4, i => ({ SEQPESSOA:i + 1, NOMERAZAO:'Pessoa ' + i, FANTASIA:'P' + i, DTAALTERACAO:'2025-0' + (i + 1) + '-01', USUALTERACAO:'adm' }))],
    ['MAP_PRODUTO', gera(10, i => ({ SEQPRODUTO:i + 1, DESCCOMPLETA:'Produto ' + i, SEQFAMILIA:1 + i % 3 }))],
    ['MFL_DOCTOFISCAL', gera(10, i => ({ SEQNF:500 + i, NROEMPRESA:1, DTAEMISSAO:'2026-03-0' + (i % 9 + 1), VLRTOTAL:80 + i }))],
    ['MFL_DFITEM', gera(10, i => ({ SEQNF:500 + (i >> 1), SEQPRODUTO:1 + i, QUANTIDADE:1 + i % 3, VLRITEM:7 + i }))]
  ]));
  assert.deepEqual(papeis(c), { FI_TITULO:'movimento', GE_PESSOA:'cadastro', MAP_PRODUTO:'cadastro', MFL_DOCTOFISCAL:'movimento', MFL_DFITEM:'itens' });
  assert.deepEqual(perfil(c, 'FI_TITULO').colunas, { data:'DTAEMISSAO', valor:'VLRORIGINAL' });
});

test('perfil: cadastro com data que varia, mas sem valor nem quantidade, continua cadastro; data de geração é auditoria', () => {
  const c = cat([
    ['GE_PESSOA','TABLE',400,[['SEQPESSOA','NUMBER'],['NOMERAZAO','VARCHAR2'],['DTAATIVACAO','DATE'],['UF','CHAR']]],
    ['MRL_CLIENTECARTAO','TABLE',12,[['NROCARTAO','NUMBER'],['STATUSCARTAO','CHAR'],['DTAGERACAO','DATE'],['USUGERACAO','VARCHAR2']]]
  ]);
  P.analisarCatalogo(c, new Map([
    ['GE_PESSOA', gera(4, i => ({ SEQPESSOA:i + 1, NOMERAZAO:'Pessoa ' + i, DTAATIVACAO:'2024-0' + (i + 1) + '-10', UF:'SP' }))],
    ['MRL_CLIENTECARTAO', gera(6, i => ({ NROCARTAO:9000 + i, STATUSCARTAO:'A', DTAGERACAO:'2025-0' + (i + 1) + '-01', USUGERACAO:'adm' }))]
  ]));
  assert.deepEqual(papeis(c), { GE_PESSOA:'cadastro', MRL_CLIENTECARTAO:'cadastro' });
});

test('perfil: unitário e data de atributo também colados no começo da palavra (VLRUNITARIOECF, DTANASCFUND)', () => {
  const c = cat([
    ['MFL_DFITEM','TABLE',10,[['SEQNF','NUMBER'],['QUANTIDADE','NUMBER'],['VLRUNITARIOECF','NUMBER'],['VLRITEM','NUMBER']]],
    ['GE_PESSOA','TABLE',4,[['SEQPESSOA','NUMBER'],['DTANASCFUND','DATE'],['VLRALUGUEL','NUMBER']]]
  ]);
  P.analisarCatalogo(c, new Map([
    ['MFL_DFITEM', gera(4, i => ({ SEQNF:i, QUANTIDADE:1 + i, VLRUNITARIOECF:2 + i, VLRITEM:3 + i }))],
    ['GE_PESSOA', gera(4, i => ({ SEQPESSOA:i, DTANASCFUND:'1990-0' + (i + 1) + '-01', VLRALUGUEL:100 + i }))]
  ]));
  assert.equal(perfil(c, 'MFL_DFITEM').colunas.valor, 'VLRITEM');
  assert.notEqual(perfil(c, 'GE_PESSOA').papel, 'movimento');
  assert.equal(perfil(c, 'GE_PESSOA').colunas.data, undefined);
});

test('perfil: com o.so só as tabelas novas ganham perfil, e a de itens nova se liga ao movimento já guardado', () => {
  const c = PT();
  P.analisarCatalogo(c, AM_PT);
  const guardado = { papel:'cadastro', colunas:{}, amostra:true, chave:null, descricao:'Lojas.' };
  c.tables.find(t => t.name === 'loja').perfil = guardado;
  const item = c.tables.find(t => t.name === 'mov_venda_item'); delete item.perfil;
  P.analisarCatalogo(c, new Map([['mov_venda_item', AM_PT.get('mov_venda_item')]]), { so:new Set(['mov_venda_item']) });
  assert.equal(perfil(c, 'mov_venda_item').papel, 'itens');
  assert.equal(perfil(c, 'loja'), guardado);
  assert.equal(perfil(c, 'mov_venda').papel, 'movimento');
});
