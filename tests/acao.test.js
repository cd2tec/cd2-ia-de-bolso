'use strict';
// Ação com confirmação humana (Fase 4, CIDB-21): tipos, leitura do que o modelo sugeriu, estado pela
// auditoria e o conteúdo de cada ação. Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const A = require('../web/acao.js');

const exec = (acao, auditoria, extra) => Object.assign({
  seq:4, em:'2026-11-24T10:30:00Z',
  resultado:{ ok:true, colunas:['loja', 'total'], todas:[{ loja:'Loja 01', total:1234.5 }, { loja:'Loja 02; centro', total:10 }] },
  resumo:{ linhas:['A soma foi 1.244,50.', 'A Loja 01 vendeu 1.234,50.'], acao },
  auditoria
}, extra || {});

test('enum: as três ações do brief e "nenhuma"', () => {
  assert.deepEqual(A.ENUM, ['copiar_resumo', 'rascunho_email', 'gerar_csv', 'nenhuma']);
  for(const t of A.TIPOS) assert.ok(A.rotulo(t));
});

test('lerAcao: tipo do enum vale; "nenhuma" e ausente são sem ação; fora do enum é descartado (H5-CT08)', () => {
  assert.deepEqual(A.lerAcao('gerar_csv'), { acao:'gerar_csv' });
  assert.deepEqual(A.lerAcao('nenhuma'), { acao:'' });
  assert.deepEqual(A.lerAcao(undefined), { acao:'' });
  assert.deepEqual(A.lerAcao('enviar_email'), { acao:'', fora:'enviar_email' });
  assert.deepEqual(A.lerAcao({ x:1 }), { acao:'', fora:'{"x":1}' });
});

test('textoAviso: diz a ação e que a decisão é no histórico', () => {
  assert.equal(A.textoAviso('copiar_resumo'), 'Ação sugerida: Copiar o resumo. Abra o histórico para aprovar ou recusar.');
  assert.equal(A.textoAviso(''), '');
  assert.equal(A.textoAviso('outra'), '');
});

test('estado: sem ação sugerida não há bloco (História 3, H5-CT09)', () => {
  assert.equal(A.estado(exec('')), null);
  assert.equal(A.estado(exec('apagar_tudo')), null);
  assert.equal(A.estado({ resumo:{ linhas:['x'] } }), null);
  assert.equal(A.estado(null), null);
});

test('estado: pendente, recusada, aprovada sem resultado, executada e falhou', () => {
  const p = { evento:'proposta', acao:'gerar_csv', em:'2026-11-24T10:30:00Z', por:'IA de Bolso' };
  const ap = { evento:'aprovada', acao:'gerar_csv', em:'2026-11-24T10:31:00Z', por:'maria' };
  assert.equal(A.estado(exec('gerar_csv', [p])).fase, 'pendente');
  assert.equal(A.estado(exec('gerar_csv', [p, { evento:'recusada', acao:'gerar_csv' }])).fase, 'recusada');
  assert.equal(A.estado(exec('gerar_csv', [p, ap])).fase, 'aprovada');
  assert.equal(A.estado(exec('gerar_csv', [p, ap, { evento:'executada' }])).fase, 'executada');
  const f = A.estado(exec('gerar_csv', [p, ap, { evento:'falhou', detalhe:'o resultado não tem linhas' }]));
  assert.equal(f.fase, 'falhou');
  assert.equal(f.resultado.detalhe, 'o resultado não tem linhas');
  assert.equal(f.rotulo, 'Gerar o arquivo CSV com o resultado');
  assert.equal(f.eventos.length, 3);
});

test('textoEvento: o que, quem e quando (H5-CT07)', () => {
  const em = new Date(2026, 10, 24, 10, 32, 5).toISOString();
  assert.equal(A.textoEvento({ evento:'aprovada', por:'maria', em }), 'Aprovada por maria em 24/11/2026 às 10:32:05.');
  assert.equal(A.textoEvento({ evento:'proposta', por:'IA de Bolso', em }), 'Sugerida pela IA em 24/11/2026 às 10:32:05.');
  assert.equal(A.textoEvento({ evento:'falhou', por:'maria', em, detalhe:'sem linhas' }), 'Falhou por maria em 24/11/2026 às 10:32:05: sem linhas.');
  assert.equal(A.textoEvento({ evento:'outro' }), '');
});

test('copiar: as frases do resumo, uma por linha', () => {
  assert.equal(A.textoCopia(exec('copiar_resumo')), 'A soma foi 1.244,50.\nA Loja 01 vendeu 1.234,50.');
  assert.equal(A.textoCopia({}), '');
});

test('rascunho de e-mail: mailto sem destinatário, com o resumo; nada é enviado (H5-CT03)', () => {
  const r = A.rascunhoEmail('Vendas da semana', exec('rascunho_email'));
  assert.equal(r.ok, true);
  assert.match(r.url, /^mailto:\?subject=/, 'sem destinatário: quem escolhe e envia é a pessoa');
  const q = new URLSearchParams(r.url.slice('mailto:?'.length));
  assert.equal(q.get('subject'), 'IA de Bolso · Vendas da semana');
  assert.match(q.get('body'), /^A soma foi 1\.244,50\.\nA Loja 01 vendeu 1\.234,50\.\n\nConsulta: Vendas da semana\nRodou em /);
  const longo = exec('rascunho_email', [], { resumo:{ linhas:[ 'x'.repeat(5000) ], acao:'rascunho_email' } });
  assert.ok(decodeURIComponent(A.rascunhoEmail('V', longo).url.split('body=')[1]).length <= A.MAX_CORPO);
  assert.deepEqual(A.rascunhoEmail('V', exec('rascunho_email', [], { resumo:{ linhas:[], acao:'rascunho_email' } })), { ok:false, erro:'a execução não tem resumo' });
});

test('CSV: o resultado da execução, no formato do botão CSV do card (H5-CT04)', () => {
  const a = A.arquivoCsv(exec('gerar_csv'), new Date(2026, 10, 24, 10, 32));
  assert.equal(a.ok, true);
  assert.equal(a.nome, 'cd2-relatorio-20261124-1032.csv');
  assert.equal(a.conteudo, '﻿loja;total\r\nLoja 01;1234,5\r\n"Loja 02; centro";10\r\n');
});

test('CSV: sem resultado ou sem linhas, falha com o motivo (H5-CT06)', () => {
  assert.deepEqual(A.arquivoCsv(exec('gerar_csv', [], { resultado:{ ok:true, colunas:['a'], todas:[] } })), { ok:false, erro:'o resultado não tem linhas' });
  assert.deepEqual(A.arquivoCsv(exec('gerar_csv', [], { resultado:{ ok:false, erro:'x' } })), { ok:false, erro:'a execução não tem resultado' });
  assert.deepEqual(A.arquivoCsv({}), { ok:false, erro:'a execução não tem resultado' });
});
