'use strict';
// Testes do entendimento guardado do banco (web/entendimento.js). Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const E = require('../web/entendimento.js');
const Cat = require('../web/catalog.js');

const col = (name, type) => ({ name, type:type || 'VARCHAR', kind:'texto', tokens:Cat.tokens(name) });
const tab = (name, cols, perfil) => {
  const t = { name, type:'TABLE', rows:null, tokens:Cat.tokens(name), columns:cols.map(c => typeof c === 'string' ? col(c) : c) };
  if(perfil) t.perfil = perfil;
  return t;
};
const AGORA = new Date('2026-10-21T12:00:00.000Z');

function varejoCompleto(){
  return { tables:[
    tab('vendas', ['nr_cupom', 'dt_venda', 'vlr_total', 'cd_loja'], {
      papel:'movimento', colunas:{ data:'dt_venda', valor:'vlr_total' }, chave:'nr_cupom', amostra:true,
      descricao:'cupons de venda por loja', descricaoTokens:Cat.tokens('cupons de venda por loja', true) }),
    tab('vendas_itens', ['nr_cupom', 'cd_produto', 'qtd_item', 'vlr_item'], {
      papel:'itens', colunas:{ valor:'vlr_item', quantidade:'qtd_item' }, chave:null, amostra:true }),
    tab('produtos', ['cd_produto', 'ds_produto'], {
      papel:'cadastro', colunas:{}, chave:'cd_produto', amostra:false,
      descricao:'cadastro de produtos', descricaoTokens:Cat.tokens('cadastro de produtos', true) })
  ] };
}

test('capturar e aplicar devolvem o mesmo perfil sem amostras e repõem os tokens', () => {
  const cat = varejoCompleto();
  const doc = E.capturar(cat, AGORA);
  assert.equal(doc.versao, 1);
  assert.equal(doc.completo, true);
  assert.equal(doc.atualizado, '2026-10-21T12:00:00.000Z');
  assert.equal(doc.varejo, undefined);
  assert.deepEqual(doc.tabelas.vendas, {
    papel:'movimento', colunas:{ data:'dt_venda', valor:'vlr_total' }, chave:'nr_cupom', amostra:true,
    descricao:'cupons de venda por loja', campos:['nr_cupom', 'dt_venda', 'vlr_total', 'cd_loja'] });
  assert.equal(doc.tabelas.vendas_itens.descricao, undefined);
  // Nada de tokens nem linhas no documento que vai para o disco.
  const json = JSON.stringify(doc);
  assert.ok(!json.includes('descricaoTokens'));
  assert.ok(!json.includes('"tokens"'));
  // Ida e volta pelo disco: o catálogo novo fica com o mesmo perfil, tokens da descrição inclusive.
  const novo = varejoCompleto();
  for(const t of novo.tables) delete t.perfil;
  const r = E.aplicar(novo, JSON.parse(json));
  assert.deepEqual(r, { ok:true, desconhecidas:[] });
  const orig = varejoCompleto();
  for(let i = 0; i < orig.tables.length; i++) assert.deepEqual(novo.tables[i].perfil, orig.tables[i].perfil);
  assert.deepEqual(novo.tables[0].perfil.descricaoTokens, Cat.tokens('cupons de venda por loja', true));
  // Mudar o documento depois não mexe no catálogo (cópia, não referência).
  doc.tabelas.vendas.colunas.data = 'outra';
  assert.equal(cat.tables[0].perfil.colunas.data, 'dt_venda');
});

test('aplicar recusa documento estragado, de outra versão ou incompleto', () => {
  const ruins = [null, undefined, 'x', [], {}, { versao:2, completo:true, tabelas:{} }, { versao:1, tabelas:{} },
    { versao:1, completo:false, tabelas:{} }, { versao:1, completo:true }, { versao:1, completo:true, tabelas:[] },
    { versao:1, completo:true, tabelas:{ vendas:'lixo' } },
    { versao:1, completo:true, tabelas:{ vendas:{ papel:'movimento', colunas:'x', campos:[] } } },
    { versao:1, completo:true, tabelas:{ vendas:{ papel:'movimento', colunas:{}, campos:'nr_cupom' } } }];
  for(const d of ruins){
    const cat = varejoCompleto();
    const antes = JSON.stringify(cat);
    const r = E.aplicar(cat, d);
    assert.equal(r.ok, false, JSON.stringify(d));
    assert.deepEqual(r.desconhecidas, []);
    assert.equal(JSON.stringify(cat), antes, 'não mexe no catálogo');
  }
});

test('aplicar lista tabelas do banco que o documento não conhece', () => {
  const doc = E.capturar(varejoCompleto(), AGORA);
  // Tabela nova, sem perfil (veio de mesclar), e nome com outra caixa.
  doc.tabelas.LOJAS = { campos:['cd_loja'] };
  const cat = { tables:[tab('VENDAS', ['nr_cupom']), tab('lojas', ['cd_loja']), tab('estoque', ['cd_produto']), tab('constructor', ['x'])] };
  const r = E.aplicar(cat, doc);
  assert.equal(r.ok, true);
  assert.deepEqual(r.desconhecidas, ['lojas', 'estoque', 'constructor']);
  assert.equal(cat.tables[0].perfil.papel, 'movimento');
  assert.equal(cat.tables[1].perfil, undefined);
  assert.equal(cat.tables[3].perfil, undefined);
});

test('comparar ignora caixa: VENDAS no documento e vendas no banco não mudou', () => {
  const doc = E.capturar({ tables:[tab('VENDAS', ['NR_CUPOM', 'DT_VENDA'], {
    papel:'movimento', colunas:{}, chave:'NR_CUPOM', amostra:true, descricao:'cupons de venda' })] }, AGORA);
  const cat = { tables:[tab('vendas', ['nr_cupom', 'dt_venda'])] };
  const d = E.comparar(cat, doc, { versao:1, tabelas:{}, consultas:[] });
  assert.deepEqual(d, { novasTabelas:[], novosCampos:[], sumiram:{ tabelas:[], campos:[] }, notasApagadas:[],
    semDescricao:[], consultasAfetadas:[], nadaMudou:true });
});

function docAntigo(){
  return E.capturar({ tables:[
    tab('vendas', ['nr_cupom', 'dt_venda', 'cd_vendedor'], { papel:'movimento', colunas:{}, chave:'nr_cupom', amostra:true, descricao:'cupons de venda' }),
    tab('promocoes', ['cd_promo', 'ds_promo'], { papel:'cadastro', colunas:{}, chave:'cd_promo', amostra:true, descricao:'promoções' }),
    tab('__proto__', ['x'], { papel:'parametro', colunas:{}, chave:null, amostra:true })
  ] }, AGORA);
}
const notasCheias = () => ({ versao:1, tabelas:{
  Promocoes:{ nota:'Promoções antigas, não usar', campos:{ ds_promo:'texto livre' } },
  vendas:{ nota:'Tabela oficial', campos:{ CD_VENDEDOR:'vendedor da venda', nr_cupom:'número do cupom', dt_venda:'' } }
}, consultas:[
  { id:'1', nome:'Promoções do mês', sql:'SELECT * FROM PROMOCOES WHERE x = 1' },
  { id:'2', nome:'Vendas do dia', sql:"SELECT * FROM vendas WHERE obs = 'promocoes'" },
  { id:'3', nome:'Prefixo parecido', sql:'SELECT * FROM promocoes_2025' },
  { id:'4', nome:'Junta', sql:'SELECT * FROM vendas v JOIN dbo.promocoes p ON 1=1' }
] });

test('comparar acha tabela e campo novos, sumidos, notas que vão ser apagadas e consultas afetadas', () => {
  const cat = { tables:[tab('vendas', ['nr_cupom', 'dt_venda', 'cd_loja']), tab('lojas', ['cd_loja']), tab('__proto__', ['x'])] };
  const d = E.comparar(cat, docAntigo(), notasCheias());
  assert.deepEqual(d.novasTabelas, ['lojas']);
  assert.deepEqual(d.novosCampos, [{ tabela:'vendas', campo:'cd_loja' }]);
  assert.deepEqual(d.sumiram, { tabelas:['promocoes'], campos:[{ tabela:'vendas', campo:'cd_vendedor' }] });
  assert.deepEqual(d.notasApagadas, [
    { tipo:'tabela', onde:'Promocoes', nota:'Promoções antigas, não usar' },
    { tipo:'campo', onde:'Promocoes.ds_promo', nota:'texto livre' },
    { tipo:'campo', onde:'vendas.CD_VENDEDOR', nota:'vendedor da venda' }
  ]);
  assert.deepEqual(d.semDescricao, []);
  assert.deepEqual(d.consultasAfetadas, ['Promoções do mês', 'Junta']);
  assert.equal(d.nadaMudou, false);
});

test('comparar: mudança só de tipo não conta', () => {
  const doc = E.capturar({ tables:[tab('vendas', [col('nr_cupom', 'INTEGER')], { papel:'movimento', colunas:{}, chave:null, amostra:true, descricao:'cupons' })] }, AGORA);
  const cat = { tables:[tab('vendas', [col('nr_cupom', 'BIGINT')])] };
  const d = E.comparar(cat, doc, null);
  assert.equal(d.nadaMudou, true);
  assert.deepEqual(d.novosCampos, []);
  assert.deepEqual(d.sumiram.campos, []);
});

test('comparar: tabela sem descrição faz nadaMudou false', () => {
  const doc = E.capturar({ tables:[
    tab('vendas', ['nr_cupom'], { papel:'movimento', colunas:{}, chave:null, amostra:true }),
    tab('itens', ['nr_cupom'], { papel:'itens', colunas:{}, chave:null, amostra:true, descricao:'' }),
    tab('aux', ['x'], { papel:'indefinida', colunas:{}, chave:null, amostra:true }),
    tab('produtos', ['cd'], { papel:'cadastro', colunas:{}, chave:null, amostra:true, descricao:'produtos' }),
    tab('parametros', ['k', 'v'], { papel:'parametro', colunas:{}, chave:null, amostra:true }),
    tab('vazia', ['x'], { papel:'vazia', colunas:{}, chave:null, amostra:true })
  ] }, AGORA);
  const cat = { tables:['vendas', 'itens', 'aux', 'produtos', 'parametros', 'vazia'].map((n, i) => tab(n, [['nr_cupom'], ['nr_cupom'], ['x'], ['cd'], ['k', 'v'], ['x']][i])) };
  const d = E.comparar(cat, doc, { versao:1, tabelas:{}, consultas:[] });
  assert.deepEqual(d.semDescricao, ['vendas', 'itens', 'aux']);
  assert.equal(d.nadaMudou, false);
  assert.deepEqual(d.novasTabelas, []);
});

test('limparNotas tira só o que sumiu e mantém consultas', () => {
  const notas = notasCheias();
  const antes = JSON.stringify(notas);
  const diff = { sumiram:{ tabelas:['promocoes'], campos:[{ tabela:'VENDAS', campo:'cd_vendedor' }] } };
  const r = E.limparNotas(notas, diff);
  assert.equal(JSON.stringify(notas), antes, 'não muta a entrada');
  assert.deepEqual(Object.keys(r.tabelas), ['vendas']);
  assert.deepEqual(r.tabelas.vendas, { nota:'Tabela oficial', campos:{ nr_cupom:'número do cupom', dt_venda:'' } });
  assert.deepEqual(r.consultas, notasCheias().consultas);
  assert.equal(r.versao, 1);
  // Nome perigoso continua dado comum, sem virar protótipo.
  const n2 = { versao:1, consultas:[], tabelas:{} };
  Object.defineProperty(n2.tabelas, '__proto__', { value:{ nota:'p', campos:{} }, enumerable:true });
  const r2 = E.limparNotas(n2, { sumiram:{ tabelas:[], campos:[] } });
  assert.ok(Object.prototype.hasOwnProperty.call(r2.tabelas, '__proto__'));
  assert.equal(r2.tabelas.__proto__.nota, 'p');
  assert.equal(Object.getPrototypeOf(r2.tabelas), Object.prototype);
});

test('mesclar tira o que sumiu, põe as novas sem perfil e atualiza campos', () => {
  const doc = docAntigo();
  doc.varejo = { verificado:true, parece:true };
  const cat = { tables:[tab('vendas', ['nr_cupom', 'dt_venda', 'cd_loja']), tab('lojas', ['cd_loja', 'nm_loja']), tab('__proto__', ['x', 'y'])] };
  const diff = E.comparar(cat, doc, null);
  const novo = E.mesclar(doc, cat, diff, new Date('2026-10-22T08:00:00.000Z'));
  assert.equal(novo.versao, 1);
  assert.equal(novo.completo, true);
  assert.equal(novo.atualizado, '2026-10-22T08:00:00.000Z');
  assert.deepEqual(novo.varejo, { verificado:true, parece:true });
  assert.deepEqual(Object.keys(novo.tabelas), ['vendas', 'lojas', '__proto__']);
  assert.deepEqual(novo.tabelas.vendas, { papel:'movimento', colunas:{}, chave:'nr_cupom', amostra:true,
    descricao:'cupons de venda', campos:['nr_cupom', 'dt_venda', 'cd_loja'] });
  assert.deepEqual(novo.tabelas.lojas, { campos:['cd_loja', 'nm_loja'] });
  assert.ok(Object.prototype.hasOwnProperty.call(novo.tabelas, '__proto__'));
  assert.deepEqual(novo.tabelas.__proto__.campos, ['x', 'y']);
  assert.equal(novo.tabelas.__proto__.papel, 'parametro');
  assert.equal(doc.atualizado, '2026-10-21T12:00:00.000Z', 'não muta o documento antigo');
  assert.deepEqual(doc.tabelas.vendas.campos, ['nr_cupom', 'dt_venda', 'cd_vendedor']);
});

test('pareceVarejo: ERP com nomes claros (produtos, vendas_cab, lojas) parece', () => {
  const cat = { tables:[
    tab('produtos', ['cd_produto', 'ds_produto', 'cd_ean', 'vlr_preco'], { papel:'cadastro', colunas:{}, chave:'cd_produto', amostra:true }),
    tab('vendas_cab', ['nr_cupom', 'dt_venda', 'vlr_total', 'cd_loja'], { papel:'movimento', colunas:{}, chave:'nr_cupom', amostra:true }),
    tab('lojas', ['cd_loja', 'nm_loja'], { papel:'cadastro', colunas:{}, chave:'cd_loja', amostra:true })
  ] };
  const r = E.pareceVarejo(cat);
  assert.equal(r.parece, true);
  assert.ok(r.grupos >= 4, String(r.grupos));
});

test('pareceVarejo: nomes em código ZAN_M45/ZAN_M43 com descrições de cupom e itens vendidos parece', () => {
  const p = (papel, descricao) => ({ papel, colunas:{}, chave:null, amostra:true, descricao, descricaoTokens:Cat.tokens(descricao, true) });
  const cat = { tables:[
    tab('ZAN_M43', ['M43AA', 'M43AB', 'M43AC', 'M43AK'], p('itens', 'itens vendidos em cada cupom')),
    tab('ZAN_M45', ['M45AA', 'M45AB', 'M45AC', 'M45AD'], p('movimento', 'cabeçalho dos cupons de venda por loja e caixa')),
    tab('ZAN_M00', ['M00AA', 'M00AB', 'M00AC'], p('cadastro', 'cadastro de produtos com código EAN'))
  ] };
  // Sem as descrições, os nomes em código não dizem nada.
  const semDesc = { tables:cat.tables.map(t => tab(t.name, t.columns.map(c => c.name), { papel:t.perfil.papel, colunas:{}, chave:null, amostra:true })) };
  assert.equal(E.pareceVarejo(semDesc).grupos, 0);
  const r = E.pareceVarejo(cat);
  assert.equal(r.parece, true);
  assert.ok(r.grupos >= 3, String(r.grupos));
  // Só um grupo, mas um movimento com sinal de venda já basta para não avisar.
  const um = { tables:[tab('ZAN_M45', ['M45AA'], p('movimento', 'cupons de venda do dia'))] };
  assert.deepEqual(E.pareceVarejo(um), { parece:true, grupos:1 });
});

test('pareceVarejo: sistema de clínica (pacientes, consultas, prontuarios, medicos) não parece', () => {
  const cat = { tables:[
    tab('pacientes', ['id_paciente', 'nm_paciente', 'dt_nascimento', 'cd_convenio'], { papel:'cadastro', colunas:{}, chave:'id_paciente', amostra:true, descricao:'cadastro de pacientes da clínica' }),
    tab('consultas', ['id_consulta', 'id_paciente', 'id_medico', 'dt_consulta', 'vlr_consulta'], { papel:'movimento', colunas:{}, chave:'id_consulta', amostra:true, descricao:'consultas marcadas com médico e paciente' }),
    tab('prontuarios', ['id_prontuario', 'id_paciente', 'ds_anamnese'], { papel:'cadastro', colunas:{}, chave:'id_prontuario', amostra:true, descricao:'anotações clínicas de cada paciente' }),
    tab('medicos', ['id_medico', 'nm_medico', 'nr_crm', 'ds_especialidade'], { papel:'cadastro', colunas:{}, chave:'id_medico', amostra:true, descricao:'médicos e especialidades' })
  ] };
  assert.deepEqual(E.pareceVarejo(cat), { parece:false, grupos:0 });
});

test('pareceVarejo: sistema de RH (funcionarios, folha, ferias, cargos) não parece', () => {
  const cat = { tables:[
    tab('funcionarios', ['id_funcionario', 'nm_funcionario', 'dt_admissao', 'id_cargo'], { papel:'cadastro', colunas:{}, chave:'id_funcionario', amostra:true, descricao:'cadastro de funcionários' }),
    tab('folha_pagamento', ['id_funcionario', 'dt_competencia', 'vlr_salario', 'vlr_desconto'], { papel:'movimento', colunas:{}, chave:null, amostra:true, descricao:'folha de pagamento mensal de cada funcionário' }),
    tab('ferias', ['id_funcionario', 'dt_inicio', 'dt_fim'], { papel:'indefinida', colunas:{}, chave:null, amostra:true }),
    tab('cargos', ['id_cargo', 'ds_cargo', 'vlr_salario_base'], { papel:'cadastro', colunas:{}, chave:'id_cargo', amostra:true, descricao:'cargos e salário base' })
  ] };
  // "pagamento" da folha é um grupo só, e a folha não tem sinal de venda: avisa.
  assert.deepEqual(E.pareceVarejo(cat), { parece:false, grupos:1 });
});

test('pareceVarejo: catálogo vazio ou sem sinal nenhum mas com poucas tabelas não quebra', () => {
  // Sem tabela nenhuma não há o que julgar: na dúvida, não avisa.
  assert.deepEqual(E.pareceVarejo({ tables:[] }), { parece:true, grupos:0 });
  assert.deepEqual(E.pareceVarejo(null), { parece:true, grupos:0 });
  assert.deepEqual(E.pareceVarejo(undefined), { parece:true, grupos:0 });
  // Poucas tabelas sem sinal e sem perfil: não quebra e segue a regra (nenhum grupo, nenhum movimento de venda).
  const r = E.pareceVarejo({ tables:[tab('t1', ['a', 'b']), { name:'t2' }] });
  assert.deepEqual(r, { parece:false, grupos:0 });
});

test('TEXTO_VAREJO é o texto combinado', () => {
  assert.equal(E.TEXTO_VAREJO, 'Este banco não parece ser de um ERP de varejo. A IA de Bolso foi feita para funcionar melhor com ERPs de varejo: ela vai tentar ajudar mesmo assim, mas pode errar mais.');
});
