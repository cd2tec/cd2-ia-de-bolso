'use strict';
// Histórico aberto (web/index.html): a cada tique, as execuções gravadas depois da mais nova da tela entram
// no topo, inclusive as "Sem aviso", que não viram novidade. Antes só apareciam ao reabrir.
// As funções moram no index.html; o teste recorta o texto delas e roda com dublês. Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs'), path = require('node:path'), vm = require('node:vm');
const html = fs.readFileSync(path.join(__dirname, '..', 'web', 'index.html'), 'utf8');

function recorte(inicio){
  const i = html.indexOf(inicio);
  assert.ok(i >= 0, 'não achei ' + inicio + ' no index.html');
  return html.slice(i, html.indexOf('\n}\n', i) + 3);
}

function montar(itens, hsExtra){
  const topo = [], vistos = [], pedidos = [];
  const ctx = {
    st:{ ctlKey:'k' },
    hs:Object.assign({ id:'c1', demo:false, carregando:false, rodando:false, lista:{}, chave:'ch', token:1, maisNovo:10, total:10 }, hsExtra),
    rotaHistorico: (chave, id) => '/agenda/historico?chave=' + chave + '&consulta=' + id,
    ctlRequest: async (m, rota) => { pedidos.push(rota); return { status:200, data:{ itens, total:20 } }; },
    execucaoNoTopo: (e, aberta) => { topo.push([e.seq, aberta]); if(e.seq > ctx.hs.maisNovo) ctx.hs.maisNovo = e.seq; },
    marcarVisto: tk => vistos.push(tk)
  };
  vm.createContext(ctx);
  vm.runInContext(recorte('async function novasNoHistorico('), ctx);
  return { ctx, topo, vistos, pedidos };
}

test('execuções novas entram no topo, da mais velha para a mais nova; só a mais nova aberta', async () => {
  const m = montar([{ seq:13, novo:false }, { seq:12, novo:false }, { seq:11, novo:false }, { seq:10 }, { seq:9 }]);
  await m.ctx.novasNoHistorico();
  assert.deepEqual(m.topo, [[11, false], [12, false], [13, true]]);
  assert.equal(m.ctx.hs.maisNovo, 13);
  assert.equal(m.ctx.hs.total, 20);
  assert.deepEqual(m.vistos, [], 'sem aviso: nada a marcar como visto');
});

test('execução nova que avisou é marcada como vista (o histórico está aberto)', async () => {
  const m = montar([{ seq:11, novo:true }, { seq:10 }]);
  await m.ctx.novasNoHistorico();
  assert.deepEqual(m.topo, [[11, true]]);
  assert.deepEqual(m.vistos, [1]);
});

test('nada novo: não mexe na tela', async () => {
  const m = montar([{ seq:10 }, { seq:9 }]);
  await m.ctx.novasNoHistorico();
  assert.deepEqual(m.topo, []);
});

test('não pede nada com o histórico fechado, carregando, rodando ou na Demonstração', async () => {
  for(const extra of [{ id:null }, { carregando:true }, { rodando:true }, { demo:true }, { lista:null }]){
    const m = montar([{ seq:11 }], extra);
    await m.ctx.novasNoHistorico();
    assert.deepEqual(m.pedidos, [], JSON.stringify(extra));
  }
});

test('histórico trocado enquanto pedia: descarta a resposta', async () => {
  const m = montar([{ seq:11 }]);
  m.ctx.ctlRequest = async () => { m.ctx.hs.token = 2; return { status:200, data:{ itens:[{ seq:11 }] } }; };
  await m.ctx.novasNoHistorico();
  assert.deepEqual(m.topo, []);
});
