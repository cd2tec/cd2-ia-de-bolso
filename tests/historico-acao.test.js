'use strict';
// Card do histórico, ação sugerida (Fase 4, CIDB-21): Aprovar e Recusar, nada roda sem o clique e sem a
// aprovação gravada na auditoria, o resultado (ou a falha) volta para a auditoria e para a tela.
// As funções moram no index.html; o teste recorta o texto delas e roda com um DOM mínimo. Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs'), path = require('node:path'), vm = require('node:vm');
const html = fs.readFileSync(path.join(__dirname, '..', 'web', 'index.html'), 'utf8');

function recorte(inicio, fim){
  const i = html.indexOf(inicio);
  assert.ok(i >= 0, 'não achei ' + inicio + ' no index.html');
  const f = fim || '\n}\n';
  return html.slice(i, html.indexOf(f, i) + f.length);
}

// ---- DOM mínimo ----
class El {
  constructor(tag){ this.tag = tag; this.className = ''; this.dataset = {}; this.attrs = {}; this.filhos = []; this.pai = null; this._t = ''; this.disabled = false; }
  get textContent(){ return this._t + this.filhos.map(f => f.textContent).join(''); }
  set textContent(v){ this._t = String(v); this.filhos = []; }
  set innerHTML(v){ this.filhos = []; this._t = String(v).replace(/<[^>]+>/g, ''); }
  appendChild(f){ f.pai = this; this.filhos.push(f); return f; }
  append(...fs){ fs.forEach(f => this.appendChild(f)); }
  setAttribute(k, v){ this.attrs[k] = String(v); }
  todos(){ return this.filhos.flatMap(f => f instanceof El ? [f, ...f.todos()] : []); }
  querySelector(s){ return this.todos().find(e => e.className.split(' ').includes(s.slice(1))) || null; }
  querySelectorAll(tag){ return this.todos().filter(e => e.tag === tag); }
  replaceWith(n){ const i = this.pai.filhos.indexOf(this); n.pai = this.pai; this.pai.filhos[i] = n; }
  remove(){ if(this.pai) this.pai.filhos = this.pai.filhos.filter(f => f !== this); }
  click(){ if(this.onclick) return this.onclick(); if(this.tag === 'a') ambiente.log.push('abriu ' + this.href); }
}
let ambiente;

function montar(o){
  const log = [], posts = [];
  const opts = Object.assign({ resposta:() => null, clipboard:true }, o);
  const auditoria = [{ evento:'proposta', acao:opts.acao, em:'2026-11-24T10:30:00Z', por:'IA de Bolso' }];
  const body = new El('body');
  const ctx = {
    CD2Acao:require('../web/acao.js'), CD2Report:require('../web/report.js'),
    hs:{ chave:'k', id:'c1', demo:!!opts.demo }, MSG_SEM_CTL:'sem ctl',
    consultaDoHistorico:() => ({ id:'c1', nome:'Vendas' }),
    document:{ createElement:t => new El(t), createTextNode:t => ({ textContent:String(t) }), body },
    navigator:{ clipboard:{ writeText:async t => { if(!opts.clipboard) throw new Error('negado'); log.push('copiou ' + t); } } },
    baixarArquivo:(nome, conteudo) => log.push('baixou ' + nome + ' ' + JSON.stringify(conteudo)),
    bancoMudouAgenda:() => log.push('banco mudou'),
    ctlRequest:async (metodo, rota, corpo) => {
      posts.push(corpo); log.push('POST ' + corpo.evento + (corpo.detalhe ? ' (' + corpo.detalhe + ')' : ''));
      const r = opts.resposta(corpo);
      if(r) return r;
      auditoria.push({ evento:corpo.evento, acao:corpo.acao, em:'2026-11-24T10:31:00Z', por:'maria', detalhe:corpo.detalhe });
      return { status:200, data:{ auditoria:auditoria.slice() } };
    },
    Date, URLSearchParams
  };
  vm.createContext(ctx);
  vm.runInContext(recorte('function botaoSm(', '}\n'), ctx);
  vm.runInContext(recorte('function mostrarErro('), ctx);
  vm.runInContext(recorte('const TEXTO_FASE_ACAO', '};\n'), ctx);
  for(const f of ['function blocoAcao(', 'async function auditarAcao(', 'async function executarAcao(', 'async function decidirAcao(']) vm.runInContext(recorte(f), ctx);
  vm.runInContext(recorte('function trocarBlocoAcao(', '}\n'), ctx);
  ambiente = { log };
  const e = { seq:4, em:'2026-11-24T10:30:00Z',
    resultado:opts.resultado || { ok:true, colunas:['loja', 'total'], todas:[{ loja:'Loja 01', total:1234.5 }] },
    resumo:{ linhas:['A soma foi 1.234,50.'], acao:opts.acao }, auditoria:auditoria.slice() };
  const card = new El('div'), bloco = ctx.blocoAcao(e);
  if(bloco) card.appendChild(bloco);
  const atual = () => card.filhos[0];
  const botao = nome => atual().querySelectorAll('button').find(b => b.textContent === nome);
  return { ctx, e, card, atual, botao, log, posts };
}

test('sem ação sugerida (História 3) ou na demonstração: sem bloco (H5-CT09)', () => {
  assert.equal(montar({ acao:'' }).atual(), undefined);
  assert.equal(montar({ acao:'gerar_csv', demo:true }).atual(), undefined);
});

test('H5-CT01: a ação aparece com Aprovar e Recusar; sem clique, nada roda nem é gravado', () => {
  const m = montar({ acao:'gerar_csv' });
  assert.match(m.atual().textContent, /Ação sugerida pela IA: Gerar o arquivo CSV com o resultado/);
  assert.match(m.atual().textContent, /Nada é feito sem o seu clique/);
  assert.ok(m.botao('Aprovar') && m.botao('Recusar'));
  assert.deepEqual(m.log, []);
});

test('H5-CT02: aprovar "copiar resumo" grava a aprovação, copia e grava o resultado, nessa ordem', async () => {
  const m = montar({ acao:'copiar_resumo' });
  await m.botao('Aprovar').click();
  assert.deepEqual(m.log, ['POST aprovada', 'copiou A soma foi 1.234,50.', 'POST executada (resumo copiado)']);
  assert.equal(m.posts[0].seq, 4); assert.equal(m.posts[0].em, '2026-11-24T10:30:00Z'); assert.equal(m.posts[0].acao, 'copiar_resumo');
  assert.match(m.atual().textContent, /Feito\./);
  assert.match(m.atual().textContent, /Aprovada por maria/);
  assert.equal(m.atual().querySelectorAll('button').length, 0, 'decidida: sem botões');
});

test('H5-CT03: aprovar "rascunho de e-mail" só abre o mailto (sem destinatário, sem envio)', async () => {
  const m = montar({ acao:'rascunho_email' });
  assert.match(m.atual().textContent, /nada é enviado/);
  await m.botao('Aprovar').click();
  assert.equal(m.log[0], 'POST aprovada');
  assert.match(m.log[1], /^abriu mailto:\?subject=IA%20de%20Bolso%20%C2%B7%20Vendas&body=A%20soma%20foi/);
  assert.equal(m.log[2], 'POST executada (rascunho pedido ao programa de e-mail (sem envio))');
});

test('H5-CT04: aprovar "gerar CSV" baixa o resultado e grava o nome do arquivo', async () => {
  const m = montar({ acao:'gerar_csv' });
  await m.botao('Aprovar').click();
  assert.equal(m.log[0], 'POST aprovada');
  assert.match(m.log[1], /^baixou cd2-relatorio-\d{8}-\d{4}\.csv "\uFEFFloja;total\\r\\nLoja 01;1234,5\\r\\n"$/);
  assert.match(m.log[2], /^POST executada \(arquivo cd2-relatorio-/);
});

test('H5-CT05: recusar grava a recusa e não executa nada', async () => {
  const m = montar({ acao:'gerar_csv' });
  await m.botao('Recusar').click();
  assert.deepEqual(m.log, ['POST recusada']);
  assert.match(m.atual().textContent, /Recusada: nada foi feito/);
});

test('H5-CT06: a ação que falha é informada na tela e gravada com o motivo', async () => {
  const m = montar({ acao:'gerar_csv', resultado:{ ok:true, colunas:['a'], todas:[] } });
  await m.botao('Aprovar').click();
  assert.deepEqual(m.log, ['POST aprovada', 'POST falhou (o resultado não tem linhas)']);
  assert.match(m.atual().textContent, /Atenção Não deu certo\. Falhou por maria .*: o resultado não tem linhas\./);
  const c = montar({ acao:'copiar_resumo', clipboard:false });
  await c.botao('Aprovar').click();
  assert.match(c.log[1], /^POST falhou \(o navegador não deixou copiar \(negado\)\)$/);
});

test('sem a aprovação gravada, a ação não roda (o cd2-ctl recusou ou não respondeu)', async () => {
  const m = montar({ acao:'copiar_resumo', resposta:c => c.evento === 'aprovada' ? { status:409, data:{ erro:'Essa ação já foi aprovada ou recusada.' } } : null });
  await m.botao('Aprovar').click();
  assert.deepEqual(m.log, ['POST aprovada']);
  assert.match(m.atual().textContent, /Não consegui gravar a decisão, e nada foi feito: Essa ação já foi aprovada ou recusada\./);
  assert.equal(m.botao('Aprovar').disabled, false, 'pode tentar de novo');
  const s = montar({ acao:'gerar_csv', resposta:() => ({ status:0, data:{} }) });
  await s.botao('Aprovar').click();
  assert.deepEqual(s.log, ['POST aprovada']);
});

test('a ação rodou, mas o resultado não foi gravado: a tela avisa', async () => {
  const m = montar({ acao:'copiar_resumo', resposta:c => c.evento === 'executada' ? { status:500, data:{ erro:'disco cheio' } } : null });
  await m.botao('Aprovar').click();
  assert.match(m.atual().textContent, /Feito, mas não consegui gravar o resultado na auditoria: disco cheio/);
});
