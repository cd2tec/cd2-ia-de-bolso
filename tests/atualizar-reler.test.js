'use strict';
// "Atualizar com o banco" numa pasta (planilhas, DBF): o POST /reler e o que vem depois dele. A função
// mora no index.html; o teste recorta o texto dela e roda com o resto da página trocado por dublês.
// Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs'), path = require('node:path'), vm = require('node:vm');
const html = fs.readFileSync(path.join(__dirname, '..', 'web', 'index.html'), 'utf8');

function recorte(inicio){
  const i = html.indexOf(inicio);
  assert.ok(i >= 0, 'não achei ' + inicio + ' no index.html');
  return html.slice(i, html.indexOf('\n}\n', i) + 3);
}

const SEM_RESPOSTA = 'O controle do banco não respondeu. Feche e abra a IA de Bolso pelo atalho INICIAR. Nenhuma nota foi apagada.';

// reler: o que o POST /reler devolve ({status, data}) ou um Error para lançar. depois: o estado do
// cd2-ctl que o esperarCtl encontra (null: não respondeu). reabre: se a sessão nova abre.
function montar({ reler, depois = 'rodando', reabre = true, busy = false, duranteReler }){
  const passos = [], chamadas = [], pilulas = [];
  const janela = { open:false, showModal(){ this.open = true; }, close(){ this.open = false; passos.push('fechou'); } };
  const nova = { call: async () => 'catálogo novo' };
  const st = { catalog:{}, notas:{}, entendimentoPronto:true, busy, catalogGen:1, tools:['x'], dbStatus:{ engine:'excel' },
    provider:{ call: async () => 'catálogo' }, ctlStatus:{ motor:'excel', estado:'rodando' } };
  const ctx = {
    st, at:{ leitura:0, alvo:null }, demo:{}, SEM_CHAVE_ATUALIZAR:'sem chave',
    $: id => { assert.equal(id, '#atualizar'); return janela; },
    motorPasta: () => 'excel',
    etapaAtualizar: (etapa, erro) => passos.push(erro ? etapa + ': ' + erro : etapa),
    ctlRequest: async (...args) => {
      chamadas.push(['ctlRequest', ...args]);
      if(duranteReler) duranteReler(st);
      if(reler instanceof Error) throw reler;
      return reler;
    },
    esperarCtl: async () => { chamadas.push(['esperarCtl']); st.ctlStatus = depois ? { motor:'excel', estado:depois } : null; },
    reabrirToolbox: async () => { chamadas.push(['reabrirToolbox']); if(!reabre) throw new Error('não abriu'); st.provider = nova; },
    renderToolbox(){}, syncBotaoNotas(){ chamadas.push(['syncBotaoNotas']); },
    setPill: (id, cls, texto) => pilulas.push([id, cls, texto]),
    chaveEntendimento: () => 'chave',
    CD2Ask:{ parseRows: () => ({ ok:true, rows:[{}] }) }, CD2Catalog:{ buildCatalog: () => ({}) },
    CD2Entendimento:{ comparar: () => ({ nadaMudou:true }) },
  };
  vm.createContext(ctx);
  vm.runInContext(recorte('function semSessaoToolbox(') + recorte('async function abrirAtualizar('), ctx);
  return { st, janela, passos, chamadas, pilulas, nova, rodar: () => vm.runInContext('abrirAtualizar()', ctx) };
}

const morta = (t) => {
  // A lista vem do contexto do vm (outro Array): confere pelo tamanho.
  assert.ok(Array.isArray(t.st.tools) && t.st.tools.length === 0, 'tools'); assert.equal(t.st.provider, null); assert.equal(t.st.dbStatus, null);
  assert.deepEqual(t.pilulas.at(-1), ['#pMcp', 'err', 'sem banco']);
  assert.ok(t.chamadas.some(c => c[0] === 'syncBotaoNotas'));
};

test('/reler vai sem prazo e, rodando, reabre a sessão e lê o catálogo dela', async () => {
  const t = montar({ reler:{ status:200, data:{ estado:'rodando', motor:'excel' } } });
  await t.rodar();
  const pedido = t.chamadas.find(c => c[0] === 'ctlRequest');
  assert.deepEqual(pedido.slice(1, 4), ['POST', '/reler', null]);
  assert.equal(pedido[4], undefined, 'o /reler não pode ter prazo');
  assert.deepEqual(t.passos, ['lendo', 'relendo', 'lendo', 'igual']);
  assert.equal(t.st.provider, t.nova);
});

test('rede caiu no /reler e o toolbox voltou: reabre a sessão e mostra o erro de sempre', async () => {
  const t = montar({ reler:new TypeError('Failed to fetch'), depois:'rodando' });
  await t.rodar();
  const nomes = t.chamadas.map(c => c[0]);
  assert.ok(nomes.indexOf('esperarCtl') > nomes.indexOf('ctlRequest'));
  assert.ok(nomes.indexOf('reabrirToolbox') > nomes.indexOf('esperarCtl'));
  assert.equal(t.st.provider, t.nova);
  assert.equal(t.passos.at(-1), 'erro: ' + SEM_RESPOSTA);
  assert.deepEqual(t.pilulas, []);
});

test('rede caiu no /reler e o toolbox não voltou: sessão morta e o mesmo erro', async () => {
  const t = montar({ reler:new TypeError('Failed to fetch'), depois:'erro' });
  await t.rodar();
  assert.ok(!t.chamadas.some(c => c[0] === 'reabrirToolbox'));
  morta(t);
  assert.equal(t.passos.at(-1), 'erro: ' + SEM_RESPOSTA);
});

test('404 no /reler é cd2-ctl antigo: lê o catálogo da sessão de sempre, sem erro', async () => {
  const t = montar({ reler:{ status:404, data:{} } });
  const antiga = t.st.provider;
  await t.rodar();
  assert.deepEqual(t.passos, ['lendo', 'relendo', 'lendo', 'igual']);
  assert.ok(!t.chamadas.some(c => c[0] === 'reabrirToolbox' || c[0] === 'esperarCtl'));
  assert.equal(t.st.provider, antiga);
});

test('200 com o toolbox fora do ar: sessão morta e o erro do cd2-ctl', async () => {
  const t = montar({ reler:{ status:200, data:{ estado:'erro', motor:'excel', erro:'Não achei a pasta das planilhas.' } } });
  await t.rodar();
  morta(t);
  assert.equal(t.passos.at(-1), 'erro: Não achei a pasta das planilhas. Nenhuma nota foi apagada.');
});

test('a sessão nova não abre: sessão morta', async () => {
  const t = montar({ reler:{ status:200, data:{ estado:'rodando', motor:'excel' } }, reabre:false });
  await t.rodar();
  morta(t);
  assert.match(t.passos.at(-1), /^erro: A pasta foi relida, mas não consegui falar com o toolbox de novo/);
});

test('com uma pergunta em andamento não relê a pasta', async () => {
  const t = montar({ reler:{ status:200, data:{ estado:'rodando' } }, busy:true });
  await t.rodar();
  assert.ok(!t.chamadas.some(c => c[0] === 'ctlRequest'));
  assert.deepEqual(t.passos, ['lendo', 'erro: Espere a resposta terminar para atualizar.']);
  assert.deepEqual(t.pilulas, []);
});

test('o banco mudou durante o /reler: a janela fecha em vez de ficar presa em Relendo', async () => {
  const t = montar({ reler:{ status:200, data:{ estado:'rodando' } }, duranteReler: st => { st.catalogGen++; } });
  await t.rodar();
  assert.equal(t.passos.at(-1), 'fechou');
  assert.equal(t.janela.open, false);
  assert.ok(!t.chamadas.some(c => c[0] === 'reabrirToolbox'));
});
