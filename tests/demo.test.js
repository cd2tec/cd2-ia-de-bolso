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
  assert.match(D.EXEMPLOS_TOPO, /^Exemplos. Com o seu banco, você salva com 👍 as perguntas que mais usa/);
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

// ---- agendamento na Demonstração ----
const A = require('../web/agenda.js');
const AGORA = new Date(2026, 9, 24, 14, 37); // sábado, 14:37

test('flash de vendas: gráfico novo, de hora em hora das 08h até agora', () => {
  const g = D.grafico('flash_de_vendas', { hora:14 });
  assert.deepEqual(g.colunas, ['Hora', 'Venda (R$)']);
  assert.equal(g.x, 'Hora'); assert.equal(g.y, 'Venda (R$)');
  assert.ok(g.visual === 'barra' || g.visual === 'linha');
  assert.deepEqual(g.linhas.map(r => r.Hora), ['08h', '09h', '10h', '11h', '12h', '13h', '14h']);
  assert.match(G.render(g.visual, g.colunas, g.linhas, { x:g.x, y:g.y }), /<svg/);
  // A frase cita o pico do que já está na tabela.
  const pico = g.linhas.reduce((a, b) => b['Venda (R$)'] > a['Venda (R$)'] ? b : a);
  assert.ok(g.frase.includes(pico.Hora), g.frase);
  // Sem hora: o dia todo, até 20h; antes das 8h, pelo menos a primeira hora.
  assert.equal(D.grafico('flash_de_vendas').linhas.length, 13);
  assert.equal(D.grafico('flash_de_vendas', { hora:6 }).linhas.length, 1);
  assert.equal(D.grafico('flash_de_vendas', { hora:23 }).linhas.length, 13);
  assert.ok(!D.TOOLS.some(t => t.name === 'flash_de_vendas'), 'não é ferramenta do modelo');
});

test('exemplos agendados: flash de vendas e ruptura, com agenda válida e bolinha', () => {
  const ex = D.exemplos(AGORA);
  const flash = ex.find(e => e.nome === 'Flash de vendas de hoje'), rup = ex.find(e => e.nome === 'Curva A em ruptura nas lojas');
  assert.ok(flash && rup);
  assert.deepEqual(flash.agenda.janela, { de:'08:00', ate:'20:00' });
  assert.equal(flash.agenda.freq, 'horas'); assert.equal(flash.agenda.a_cada, 1);
  assert.deepEqual(flash.agenda.dias, [1, 2, 3, 4, 5, 6]);
  assert.equal(rup.agenda.freq, 'semanal'); assert.equal(rup.agenda.hora, '07:30');
  for(const e of [flash, rup]){ assert.ok(A.validar(e.agenda).ok, e.id); assert.equal(e.agenda.ligado, true); }
  assert.equal(flash.novas, 2); assert.equal(rup.novas, 1);
  assert.equal(ex.filter(e => e.agenda).length, 2);
  for(const e of ex) if(!e.agenda) assert.ok(!e.novas, e.id);
  // O flash tem um parâmetro de data fictício, com o período "hoje" na agenda.
  assert.deepEqual(flash.params, [{ nome:'Data', tipo:'data', valor:'2026-10-24' }]);
  assert.deepEqual(flash.agenda.params, [{ modo:'periodo', periodo:'hoje', ponta:'inicio' }]);
  assert.equal(A.marcaBarra(flash.agenda, AGORA), '⏰ a cada 1 h · próxima 15:00');
  // O gráfico do flash vai até a hora de agora.
  assert.equal(flash.g.linhas[flash.g.linhas.length - 1].Hora, '14h');
  // Cada chamada devolve agendas novas (a tela pode mexer sem estragar o exemplo).
  flash.agenda.dias.push(0);
  assert.deepEqual(D.exemplos(AGORA).find(e => e.id === flash.id).agenda.dias, [1, 2, 3, 4, 5, 6]);
});

const TIPOS = ['agendada', 'manual', 'perda'];
function confereExecucao(e){
  assert.ok(Number.isInteger(e.seq) && e.seq > 0);
  assert.ok(TIPOS.includes(e.tipo), e.tipo);
  assert.ok(A.dataDe(e.em) && A.dataDe(e.ref), 'em/ref no formato do cd2-ctl');
  assert.ok(Array.isArray(e.valores));
  if(e.tipo === 'perda'){
    assert.ok(e.n >= 1 && A.dataDe(e.de) && A.dataDe(e.ate) && e.motivo);
    assert.equal(e.resultado, undefined);
    return;
  }
  const r = e.resultado;
  assert.equal(typeof r.ok, 'boolean');
  if(r.ok){
    for(const k of ['visual', 'x', 'y', 'titulo', 'frase']) assert.ok(r[k], k);
    assert.ok(Array.isArray(r.colunas) && Array.isArray(r.linhas) && Array.isArray(r.todas));
  } else assert.ok(r.erro);
}

test('execuções fictícias do flash: agendada nova, manual, perda, erro e dias anteriores', () => {
  const flash = D.exemplos(AGORA).find(e => e.nome === 'Flash de vendas de hoje');
  const l = D.execucoes(flash.id, AGORA);
  l.forEach(confereExecucao);
  // Da mais nova para a mais velha, seq decrescente e nada no futuro.
  for(let i = 1; i < l.length; i++) assert.ok(l[i].seq < l[i - 1].seq);
  for(const e of l) assert.ok(A.diaDaExecucao(e) <= AGORA);
  assert.equal(l[0].tipo, 'agendada'); assert.equal(l[0].novo, true);
  assert.equal(l[0].resultado.ok, true);
  assert.equal(l[0].resultado.titulo, flash.g.titulo);
  assert.equal(A.textoExecucao(l[0]), 'Rodou às 14:00 (agendada)');
  assert.deepEqual(l[0].valores, ['2026-10-24']);
  assert.ok(l.some(e => e.tipo === 'manual'));
  const perda = l.find(e => e.tipo === 'perda');
  assert.equal(perda.n, 3); assert.equal(perda.motivo, 'fechada'); assert.ok(!perda.rodada);
  assert.match(A.tituloPerda(perda), /^Não rodou 3 vezes entre /);
  const erro = l.find(e => e.resultado && !e.resultado.ok);
  assert.equal(erro.resultado.erro, 'Não consegui falar com o banco.');
  // Novidades: as marcadas como NOVO batem com a bolinha.
  assert.equal(l.filter(e => e.novo).length, flash.novas);
  // Dias anteriores: pelo menos três grupos de dia.
  assert.ok(A.agruparPorDia(l, AGORA).length >= 3);
});

test('execuções fictícias da ruptura e de exemplo sem agenda', () => {
  const rup = D.exemplos(AGORA).find(e => e.nome === 'Curva A em ruptura nas lojas');
  const l = D.execucoes(rup.id, AGORA);
  l.forEach(confereExecucao);
  assert.equal(l[0].tipo, 'agendada'); assert.equal(A.textoExecucao(l[0]), 'Rodou às 07:30 (agendada)');
  assert.equal(l.filter(e => e.novo).length, rup.novas);
  assert.ok(A.agruparPorDia(l, AGORA).length >= 2);
  // Antes das 7:30 a mais nova é a de ontem.
  const cedo = D.execucoes(rup.id, new Date(2026, 9, 24, 6, 0));
  assert.equal(A.rotuloDia(A.diaDaExecucao(cedo[0]), new Date(2026, 9, 24, 6, 0)), 'Ontem, 23/10');
  assert.deepEqual(D.execucoes('ex-perdas', AGORA), []);
  assert.deepEqual(D.execucoes('nao-existe', AGORA), []);
});

test('rodar agora na Demonstração: execução manual na hora, depois da última', () => {
  const flash = D.exemplos(AGORA).find(e => e.nome === 'Flash de vendas de hoje');
  const l = D.execucoes(flash.id, AGORA);
  const e = D.rodar(flash.id, AGORA, l[0].seq + 1);
  confereExecucao(e);
  assert.equal(e.tipo, 'manual'); assert.equal(e.seq, l[0].seq + 1);
  assert.equal(A.textoExecucao(e), 'Rodou às 14:37 (manual)');
  assert.equal(e.resultado.ok, true);
  // Da perda: referente ao último horário perdido.
  const perda = l.find(x => x.tipo === 'perda');
  const p = D.rodar(flash.id, AGORA, 99, perda);
  assert.equal(p.ref, perda.ate);
  assert.match(A.textoExecucao(p), /^Rodou às 14:37 \(referente a /);
  assert.equal(D.rodar('nao-existe', AGORA, 1), null);
});

test('aviso de exemplo: notificação e bandeja, sem nada ao vivo', () => {
  const flash = D.exemplos(AGORA).find(e => e.nome === 'Flash de vendas de hoje');
  const a = D.aviso(flash.id, AGORA);
  assert.equal(a.titulo, 'IA de Bolso · Flash de vendas de hoje');
  assert.equal(a.texto, 'Rodou às 14:00 · ' + D.execucoes(flash.id, AGORA)[0].resultado.frase);
  assert.equal(a.novos, '3 resultados novos');
  assert.deepEqual(a.itens, ['⏰ Flash de vendas de hoje (2)', '⏰ Curva A em ruptura nas lojas (1)']);
  assert.equal(a.cabecalho, 'Como o aviso chega');
  assert.equal(a.nota, 'Na versão instalada, cada execução avisa assim, mesmo com a janela fechada.');
  assert.equal(D.aviso('ex-perdas', AGORA), null);
});

test('convite do agendamento na Demonstração: texto exato', () => {
  assert.equal(D.CONVITE_AGENDA, 'Na Demonstração os agendamentos são exemplos. Conecte o banco na versão instalada para agendar de verdade.');
});

test('roteiro: agendamento cai na resposta certa, sem roubar as outras', () => {
  for(const q of ['dá pra rodar sozinho todo dia?', 'Como funciona o agendamento?', 'Como configuro um agendamento?',
    'Dá pra rodar automático de hora em hora?', 'Você me avisa quando a ruptura aparecer?', 'Manda notificação?',
    'Consigo monitorar as vendas toda hora?', 'Dá pra apagar um agendamento?'])
    assert.equal((D.roteiro(q) || {}).id, 'agendamento', q);
  for(const q of ['Qual a venda por horário hoje?', 'Mostre as vendas por hora', 'Quanto vendemos ontem?'])
    assert.equal(D.roteiro(q), null, q);
});

test('roteiro do agendamento: fatos, gráfico do flash, barra e fecho', () => {
  const r = D.ROTEIRO.find(x => x.id === 'agendamento');
  const t = D.texto(r);
  assert.match(t, /mínimo 5/);
  assert.match(t, /de hora em hora, todo dia, semanal ou mensal/);
  assert.match(t, /hoje, ontem, esta semana, este mês, últimas X horas/);
  assert.match(t, /histórico/); assert.match(t, /notificação do sistema/); assert.match(t, /mesmo com a janela fechada/);
  assert.match(t, /não roda depois sozinha/);
  assert.match(t, /flash de vendas no horário de pico e ruptura de estoque toda manhã/);
  assert.deepEqual(r.blocos.filter(b => b.grafico).map(b => b.grafico), ['flash_de_vendas']);
  assert.match(r.blocos[r.blocos.length - 1].p, /⏰/);
  assert.match(D.instrucao(r), /Termine a resposta dizendo, com as suas palavras, que o agendamento é da versão instalada no computador\./);
  // Antes de 'conectar' e de 'aprendizado' (que casariam "configur" e "como funciona").
  const i = id => D.ROTEIRO.findIndex(x => x.id === id);
  assert.ok(i('agendamento') < i('conectar') && i('agendamento') < i('aprendizado') && i('agendamento') < i('seguranca'));
});

test('boas-vindas e topo da barra falam do agendamento', () => {
  assert.match(D.texto(D.roteiro('O que você consegue fazer?')), /agend/i);
  assert.equal(D.EXEMPLOS_TOPO, 'Exemplos. Com o seu banco, você salva com 👍 as perguntas que mais usa, vê o resultado atualizado num clique e agenda para rodar sozinha.');
});
