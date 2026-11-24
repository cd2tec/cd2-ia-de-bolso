'use strict';
// Janela Agendar, aprofundamentos (Fase 3, CIDB-16): até 3 consultas marcadas, a 4ª fica travada com o
// limite à vista (H4-CT03), só com "Só quando…", e o que vai para agenda.aprofundamentos.
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

// Elementos mínimos: caixas de marcar dentro da lista e parágrafos com hidden/textContent.
function el(){
  const e = { hidden:false, textContent:'', filhos:[], dataset:{},
    appendChild(f){ this.filhos.push(f); return f; }, append(...fs){ fs.forEach(f => this.filhos.push(f)); },
    set innerHTML(v){ this.filhos = []; } };
  return e;
}

function montar(consultas, marcadas, condicao){
  const els = {};
  const ctx = {
    CD2Aprofundar:require('../web/aprofundar.js'),
    CD2Agenda:{ agendaDaTela: () => ({ ok:true, agenda:{ freq:'minutos', a_cada:5 } }) },
    CD2Condicao:{ daTela: () => ({ ok:true, condicao }) },
    ag:{ consulta:{ id:'c1', nome:'Vendas' }, t:{ aprof:marcadas.slice(), cond:{} } },
    consultasDaBarra: () => consultas,
    document:{ createElement: () => el() },
    $: s => (els[s] = els[s] || el())
  };
  vm.createContext(ctx);
  for(const f of ['function pintarAprofAgendar(', 'function mudouAprof(', 'function agendaDaJanela(']) vm.runInContext(recorte(f), ctx);
  const caixas = () => els['#agAprofLista'].filhos.map(l => l.filhos[0]);
  return { ctx, els, caixas };
}

const lista = [{ id:'c1', nome:'Vendas' }, { id:'a', nome:'Por loja' }, { id:'b', nome:'Por departamento' }, { id:'c', nome:'Por fornecedor' }, { id:'d', nome:'Ruptura' }];
const cond = { coluna:'total', agregacao:'soma', operador:'maior', valor:1 };

test('lista as outras consultas salvas, sem a própria, com as marcadas', () => {
  const m = montar(lista, ['b'], cond);
  m.ctx.pintarAprofAgendar();
  const cx = m.caixas();
  assert.deepEqual(cx.map(i => i.dataset.aprof), ['a', 'b', 'c', 'd']);
  assert.deepEqual(cx.map(i => i.checked), [false, true, false, false]);
  assert.equal(m.els['#agAprofLimite'].hidden, true);
});

test('H4-CT03: com 3 marcadas, a 4ª fica travada e o limite aparece; desmarcar destrava', () => {
  const m = montar(lista, ['a', 'b'], cond);
  m.ctx.pintarAprofAgendar();
  const c = m.caixas()[2]; c.checked = true; m.ctx.mudouAprof(c);
  assert.deepEqual(m.ctx.ag.t.aprof, ['a', 'b', 'c']);
  const d = m.caixas().find(i => i.dataset.aprof === 'd');
  assert.equal(d.disabled, true);
  assert.equal(m.els['#agAprofLimite'].hidden, false);
  assert.match(m.els['#agAprofLimite'].textContent, /Limite de 3 aprofundamentos/);
  // Mesmo que a caixa travada chegasse marcada, não passa de 3.
  d.checked = true; m.ctx.mudouAprof(d);
  assert.deepEqual(m.ctx.ag.t.aprof, ['a', 'b', 'c']);
  const a = m.caixas()[0]; a.checked = false; m.ctx.mudouAprof(a);
  assert.deepEqual(m.ctx.ag.t.aprof, ['b', 'c']);
  assert.equal(m.caixas().find(i => i.dataset.aprof === 'd').disabled, false);
});

test('vinculada apagada aparece marcada para desmarcar, e não deixa salvar', () => {
  const m = montar(lista, ['a', 'sumiu'], cond);
  m.ctx.pintarAprofAgendar();
  const ultima = m.els['#agAprofLista'].filhos.at(-1);
  assert.equal(ultima.filhos[1].textContent, '(consulta apagada)');
  assert.equal(ultima.filhos[0].checked, true);
  const r = m.ctx.agendaDaJanela();
  assert.equal(r.ok, false); assert.match(r.erro, /não existe/);
});

test('agenda: aprofundamentos só com condição; sem marcadas, nada gravado', () => {
  assert.deepEqual(montar(lista, ['a', 'c'], cond).ctx.agendaDaJanela().agenda.aprofundamentos, ['a', 'c']);
  assert.equal(montar(lista, [], cond).ctx.agendaDaJanela().agenda.aprofundamentos, undefined);
  const semCond = montar(lista, ['a'], null).ctx.agendaDaJanela();
  assert.equal(semCond.ok, true);
  assert.equal(semCond.agenda.aprofundamentos, undefined, '"Toda vez que rodar": a lista fica escondida e não é gravada');
});
