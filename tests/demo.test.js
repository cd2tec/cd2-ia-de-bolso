'use strict';
// Testes do modo Demonstração (web/demo.js). Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const D = require('../web/demo.js');
const G = require('../web/chart.js');

test('toda ferramenta responde com a etiqueta de fictício e tem gráfico que desenha', () => {
  for(const t of D.TOOLS){
    const out = D.call(t.name, {});
    assert.ok(out.startsWith(D.TAG), t.name);
    const g = D.grafico(t.name, {});
    assert.ok(g.titulo && g.frase, t.name);
    for(const r of g.linhas) assert.deepEqual(Object.keys(r), g.colunas, t.name);
    assert.ok(g.colunas.includes(g.x) && g.colunas.includes(g.y), t.name);
    assert.match(G.render(g.visual, g.colunas, g.linhas, { x:g.x, y:g.y }), /<svg/, t.name);
  }
});

test('texto da ferramenta e gráfico trazem o mesmo dado', () => {
  const out = D.call('vendas_por_departamento', {});
  assert.match(out, /Departamento \| Venda \(R\$\) \| Margem \(%\)/);
  assert.match(out, /Mercearia \| 1\.842\.300 \| 24,1/);
});

test('rupturas_criticas respeita o limite, entre 1 e o total', () => {
  assert.equal(D.grafico('rupturas_criticas', {}).linhas.length, 3);
  assert.equal(D.grafico('rupturas_criticas', { limite:2 }).linhas.length, 2);
  assert.equal(D.grafico('rupturas_criticas', { limite:99 }).linhas.length, 5);
  assert.equal(D.grafico('rupturas_criticas', { limite:0 }).linhas.length, 3);
});

test('ferramenta desconhecida falha', () => {
  assert.throws(() => D.call('apagar_tudo', {}), /desconhecida/);
  assert.equal(D.grafico('apagar_tudo'), null);
});

test('roteiro: perguntas de orientação caem na resposta certa', () => {
  const casos = {
    'O que você consegue fazer aqui?':'boas-vindas',
    'Que gráficos você faz com o meu banco?':'graficos',
    'Como você entende o meu banco?':'aprendizado',
    'É seguro conectar no meu banco?':'seguranca',
    'Como eu conecto o banco de dados?':'conectar',
    'Quais ferramentas estão conectadas?':'ferramentas',
    'quanto tempo demora pra ler as tabelas':'aprendizado',
    'Não tenho acesso a um banco. Posso usar planilhas?':'planilhas',
    'Funciona com Excel?':'planilhas',
    'não tenho ERP, uso planilha':'planilhas',
    'Como conecto o meu banco de dados?':'conectar'
  };
  for(const [q, id] of Object.entries(casos)) assert.equal((D.roteiro(q) || {}).id, id, q);
});

test('roteiro: pergunta de dado segue para o modelo', () => {
  for(const q of ['Mostre as vendas por departamento', 'Quais foram as maiores perdas do mês?',
    'Quais produtos da curva A estão em ruptura?', 'Qual o risco de ruptura do arroz?', 'Faça um gráfico das vendas por mês', ''])
    assert.equal(D.roteiro(q), null, q);
});

test('roteiro: os gráficos de exemplo cobrem vendas, ruptura e cliente', () => {
  const g = D.ROTEIRO.find(r => r.id === 'graficos').blocos.filter(b => b.grafico).map(b => b.grafico);
  assert.deepEqual(g, ['vendas_por_mes', 'rupturas_criticas', 'perfil_do_cliente']);
  for(const r of D.ROTEIRO) for(const b of r.blocos) if(b.grafico) assert.ok(D.grafico(b.grafico, b.args), r.id + ': ' + b.grafico);
});

test('roteiro em texto corrido sai sem marcação e com os gráficos citados', () => {
  const t = D.texto(D.roteiro('Que gráficos você faz?'));
  assert.ok(!t.includes('**'));
  assert.match(t, /Gráfico fictício: Sobre ruptura/);
});

test('instrução do roteiro leva os fatos sem marcação e cita os gráficos de exemplo', () => {
  const i = D.instrucao(D.roteiro('Que gráficos você faz?'));
  assert.match(i, /Fatos:/);
  assert.ok(!i.includes('**'));
  assert.match(i, /sobre ruptura \("Quais produtos da curva A/);
  const s = D.instrucao(D.roteiro('É seguro conectar?'));
  assert.match(s, /somente leitura/);
  assert.doesNotMatch(s, /gráficos de exemplo/);
});

test('instrução do roteiro: fecho obrigatório e o que evitar', () => {
  assert.match(D.instrucao(D.roteiro('Como você entende o meu banco?')), /Termine a resposta dizendo.*vários minutos/);
  assert.match(D.instrucao(D.roteiro('É seguro conectar?')), /Evite dizer que é 100% seguro/);
});

test('exemplos da barra: de 4 a 6, nomes únicos e cada um com gráfico que desenha', () => {
  const ex = D.exemplos();
  assert.ok(ex.length >= 4 && ex.length <= 6);
  assert.equal(new Set(ex.map(e => e.nome)).size, ex.length);
  assert.equal(new Set(ex.map(e => e.id)).size, ex.length);
  for(const e of ex){
    assert.ok(e.nome && e.g && e.g.titulo, e.id);
    assert.equal(e.visual, e.g.visual, e.id);
    assert.match(G.render(e.g.visual, e.g.colunas, e.g.linhas, { x:e.g.x, y:e.g.y }), /<svg/, e.id);
  }
  assert.equal(ex.find(e => e.id === 'ex-ruptura').g.linhas.length, 5);
});

test('exemplos da barra: textos exatos e nunca "sem IA"', () => {
  assert.equal(D.EXEMPLOS_TOPO, 'Exemplos. Com o seu banco, você salva com 👍 as perguntas que mais usa e vê o resultado atualizado num clique.');
  assert.equal(D.EXEMPLO_FRASE, 'No seu banco, esta consulta traz os números atualizados num clique.');
  for(const t of [D.EXEMPLOS_TOPO, D.EXEMPLO_FRASE, ...D.EXEMPLOS.map(e => e.nome)]) assert.doesNotMatch(t, /sem IA/i);
});

test('config.json sai com a Demonstração como modo padrão (pen drive novo abre nela)', () => {
  const cfg = JSON.parse(require('node:fs').readFileSync(require('node:path').join(__dirname, '..', 'web', 'config.json'), 'utf8'));
  assert.equal(cfg.mcp.mode, 'demo');
});

test('quebra-gelo de planilhas: a sugestão existe e cai na resposta das planilhas', () => {
  const cfg = JSON.parse(require('node:fs').readFileSync(require('node:path').join(__dirname, '..', 'web', 'config.json'), 'utf8'));
  const chip = cfg.suggestions.demo.find(s => /planilha/i.test(s));
  assert.ok(chip, 'falta a sugestão de planilhas no modo Demonstração');
  assert.equal(D.roteiro(chip).id, 'planilhas');
  for(const s of cfg.suggestions.demo) assert.ok(s.length <= 60, 'sugestão longa demais: ' + s);
});

test('resposta das planilhas: passo a passo curto, motor certo e sem prometer o que não faz', () => {
  const r = D.ROTEIRO.find(x => x.id === 'planilhas');
  const t = D.texto(r);
  assert.match(t, /Planilhas Excel/);
  assert.match(t, /Pasta das planilhas/);
  assert.match(t, /\.xlsx/);
  assert.match(t, /cada aba vira uma tabela/i);
  assert.match(t, /Atualizar com o banco/);
  assert.match(t, /não (são|é) alterad/);
  const passos = r.blocos.find(b => b.lista);
  assert.ok(passos && passos.numerada && passos.lista.length <= 4);
  assert.ok(r.blocos.length <= 4, 'resposta tem que ser breve');
  assert.match(D.instrucao(r), /Termine a resposta dizendo/);
});
