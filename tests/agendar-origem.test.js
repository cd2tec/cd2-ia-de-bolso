'use strict';
// "⏰ Agendar…" na barra (web/index.html): o convite da versão instalada só abre quando a origem é,
// com certeza, o pen drive. Antes do /instalacao responder, o item fica desligado ("Carregando…").
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

function montar({ status, ctlKey = 'k', semStatus = false, demo = false }){
  const abertos = [];
  const ctx = { st:{ ctlKey }, inst:{ status, semStatus }, abertos, emDemo: () => demo,
    instalada: () => !!(status && status.origem === 'instalada'),
    abrirConviteAgenda: () => abertos.push('convite'),
    podeSalvar: () => { abertos.push('agendar'); return false; } };
  vm.createContext(ctx);
  vm.runInContext(recorte('function estadoAgendar(') + recorte('function abrirAgendar('), ctx);
  return ctx;
}

test('origem ainda desconhecida: carregando, e o clique não abre o convite', () => {
  const c = montar({ status:null });
  assert.equal(c.estadoAgendar(), 'carregando');
  c.abrirAgendar({});
  assert.deepEqual(c.abertos, []);
});
test('pen drive: abre o convite', () => {
  const c = montar({ status:{ origem:'pendrive' } });
  assert.equal(c.estadoAgendar(), 'pendrive');
  c.abrirAgendar({});
  assert.deepEqual(c.abertos, ['convite']);
});
test('instalada: segue para a janela Agendar', () => {
  const c = montar({ status:{ origem:'instalada' } });
  assert.equal(c.estadoAgendar(), 'instalada');
  c.abrirAgendar({});
  assert.deepEqual(c.abertos, ['agendar']);
});
test('sem cd2-ctl ou /instalacao falhou: indisponível, sem convite', () => {
  for(const c of [montar({ status:null, ctlKey:null }), montar({ status:null, semStatus:true })]){
    assert.equal(c.estadoAgendar(), 'indisponivel');
    c.abrirAgendar({});
    assert.deepEqual(c.abertos, []);
  }
});
test('Demonstração: a janela Agendar abre com os exemplos, nunca o convite, mesmo no pen drive', () => {
  for(const status of [null, { origem:'pendrive' }, { origem:'instalada' }]){
    const c = montar({ status, demo:true });
    assert.equal(c.estadoAgendar(), 'demo');
    try{ c.abrirAgendar({}); }catch(e){} // segue para montar a janela (sem dublês de DOM aqui)
    assert.ok(!c.abertos.includes('convite'));
    assert.ok(!c.abertos.includes('agendar'), 'na Demonstração não depende de podeSalvar');
  }
});
test('o item do menu usa o estado: desligado com "Carregando…" enquanto não sabe', () => {
  assert.match(html, /agendar\.disabled = ea === 'carregando' \|\| ea === 'indisponivel'/);
  assert.match(html, /'Carregando…'/);
});
