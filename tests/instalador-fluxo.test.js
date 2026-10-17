'use strict';
// Testes do fluxo das telas do instalador (installer/frontend/dist/fluxo.js). Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const F = require('../installer/frontend/dist/fluxo.js');

const ok = { id: 'disk4', nome: 'SanDisk Ultra', bytes: 32e9, situacao: 'ok', arquivos: 3, maisDe: false, rotulo: 'SanDisk 32 GB' };
const aviso = { id: 'disk6', nome: 'Multilaser', bytes: 8e9, situacao: 'aviso', arquivos: 0, maisDe: false, rotulo: 'Multilaser 8 GB' };
const pequeno = { id: 'disk5', nome: 'Kingston', bytes: 2e9, situacao: 'pequeno', arquivos: 0, maisDe: false, rotulo: 'Kingston 2 GB' };

// aplica uma sequência de ações a partir do estado inicial
const rodar = (...acoes) => acoes.reduce(F.passo, F.inicial());
const ateConfirmar = () => rodar({ tipo: 'discos', lista: [ok] }, { tipo: 'continuar' });
const ateGravando = () => [{ tipo: 'marcar', valor: true }, { tipo: 'gravar' }].reduce(F.passo, ateConfirmar());

test('estado inicial', () => {
  assert.deepEqual(F.inicial(), { tela: 'plugue', discos: [], escolhido: null, podeApagar: false, progresso: null, frase: '', falhas: 0 });
});

test('sem discos não dá para continuar', () => {
  const e = rodar({ tipo: 'discos', lista: [] });
  assert.equal(F.podeContinuar(e), false);
  assert.equal(F.passo(e, { tipo: 'continuar' }).tela, 'plugue');
});

test('um disco ok já vem escolhido', () => {
  const e = rodar({ tipo: 'discos', lista: [ok] });
  assert.equal(e.escolhido, 'disk4');
  assert.equal(F.podeContinuar(e), true);
});

test('um disco com aviso também vem escolhido', () => {
  assert.equal(rodar({ tipo: 'discos', lista: [aviso] }).escolhido, 'disk6');
});

test('um disco pequeno sozinho não vem escolhido', () => {
  const e = rodar({ tipo: 'discos', lista: [pequeno] });
  assert.equal(e.escolhido, null);
  assert.equal(F.podeContinuar(e), false);
});

test('dois discos: nenhum escolhido até escolher', () => {
  const e = rodar({ tipo: 'discos', lista: [ok, aviso] });
  assert.equal(e.escolhido, null);
  assert.equal(F.podeContinuar(e), false);
  const e2 = F.passo(e, { tipo: 'escolher', id: 'disk6' });
  assert.equal(e2.escolhido, 'disk6');
  assert.equal(F.podeContinuar(e2), true);
});

const proprio = { id: 'disk8', nome: 'Kingston', bytes: 32e9, situacao: 'proprio', arquivos: 2, maisDe: false, rotulo: 'Kingston 32 GB' };

test('o pen drive de onde o instalador roda não pode ser escolhido', () => {
  const e = rodar({ tipo: 'discos', lista: [proprio] });
  assert.equal(e.escolhido, null);
  assert.equal(F.podeContinuar(e), false);
  assert.deepEqual(F.passo(rodar({ tipo: 'discos', lista: [ok, proprio] }), { tipo: 'escolher', id: 'disk8' }).escolhido, null);
  assert.equal(F.escolhivel(proprio), false);
  assert.equal(F.escolhivel(ok), true);
  assert.equal(F.escolhivel({ ...ok, situacao: 'desconhecida' }), false);
});

test('nota de cada situação na lista de discos', () => {
  assert.equal(F.textoSituacao(pequeno), 'Pequeno demais: precisa de pelo menos 4 GB.');
  assert.equal(F.textoSituacao(proprio), 'O instalador está rodando deste pen drive. Copie o instalador para o computador e abra de lá.');
  assert.equal(F.textoSituacao(ok), '');
  assert.equal(F.textoSituacao(aviso), '');
});

test('cinco falhas seguidas de ListarDiscos trocam o procurando pelo aviso', () => {
  const falha = { tipo: 'falhaListar' };
  let e = rodar(falha, falha, falha, falha);
  assert.equal(e.falhas, 4);
  assert.equal(F.textoProcurando(e), 'Procurando pen drive...');
  e = F.passo(e, falha);
  assert.equal(F.textoProcurando(e), 'Não consegui ver os discos. Tire o pen drive e plugue de novo.');
  // Continua tentando: uma lista que chega zera a contagem.
  e = F.passo(e, { tipo: 'discos', lista: [] });
  assert.equal(e.falhas, 0);
  assert.equal(F.textoProcurando(e), 'Procurando pen drive...');
});

test('depois de cinco falhas a lista velha some', () => {
  const falha = { tipo: 'falhaListar' };
  const e = rodar({ tipo: 'discos', lista: [ok] }, falha, falha, falha, falha, falha);
  assert.deepEqual(e.discos, []);
  assert.equal(e.escolhido, null);
  assert.equal(F.podeContinuar(e), false);
  // Uma falha só não mexe na lista.
  assert.equal(rodar({ tipo: 'discos', lista: [ok] }, falha).escolhido, 'disk4');
});

test('falhaListar fora da tela plugue não muda o estado', () => {
  const g = ateGravando();
  assert.deepEqual(F.passo(g, { tipo: 'falhaListar' }), g);
});

test('escolher disco pequeno não muda o estado', () => {
  const e = rodar({ tipo: 'discos', lista: [ok, pequeno] });
  assert.deepEqual(F.passo(e, { tipo: 'escolher', id: 'disk5' }), e);
});

test('escolher disco que não existe não muda o estado', () => {
  const e = rodar({ tipo: 'discos', lista: [ok, aviso] });
  assert.deepEqual(F.passo(e, { tipo: 'escolher', id: 'disk9' }), e);
});

test('o disco escolhido some da lista e o escolhido volta a null', () => {
  const e = rodar({ tipo: 'discos', lista: [ok, aviso] }, { tipo: 'escolher', id: 'disk4' }, { tipo: 'discos', lista: [aviso, pequeno] });
  assert.equal(e.escolhido, null);
});

test('o disco escolhido continua na lista nova e segue escolhido', () => {
  const e = rodar({ tipo: 'discos', lista: [ok, aviso] }, { tipo: 'escolher', id: 'disk6' }, { tipo: 'discos', lista: [ok, aviso, pequeno] });
  assert.equal(e.escolhido, 'disk6');
});

test('continuar leva para confirmar sem a caixa marcada', () => {
  const e = ateConfirmar();
  assert.equal(e.tela, 'confirmar');
  assert.equal(e.podeApagar, false);
});

test('em confirmar, gravar sem marcar não muda a tela', () => {
  assert.equal(F.passo(ateConfirmar(), { tipo: 'gravar' }).tela, 'confirmar');
  const desmarcado = [{ tipo: 'marcar', valor: true }, { tipo: 'marcar', valor: false }, { tipo: 'gravar' }].reduce(F.passo, ateConfirmar());
  assert.equal(desmarcado.tela, 'confirmar');
});

test('marcar e gravar leva para gravando', () => {
  const e = ateGravando();
  assert.equal(e.tela, 'gravando');
  assert.equal(e.progresso, null);
});

test('voltar em confirmar leva para plugue com podeApagar falso', () => {
  const e = F.passo(F.passo(ateConfirmar(), { tipo: 'marcar', valor: true }), { tipo: 'voltar' });
  assert.equal(e.tela, 'plugue');
  assert.equal(e.podeApagar, false);
  assert.equal(e.escolhido, 'disk4');
});

test('progresso guarda o evento e a frase', () => {
  const ev = { etapa: 'baixando', frase: 'Baixando o modelo (1,6 de 3,9 GB)', fracao: 0.42, restante: 480 };
  const e = F.passo(ateGravando(), { tipo: 'progresso', evento: ev });
  assert.deepEqual(e.progresso, ev);
  assert.equal(e.frase, ev.frase);
});

test('durante gravando, uma ação discos não muda a tela', () => {
  const g = ateGravando();
  const e = F.passo(g, { tipo: 'discos', lista: [] });
  assert.equal(e.tela, 'gravando');
  assert.deepEqual(e, g);
});

test('fim ok:false leva para erro com a frase', () => {
  const e = F.passo(ateGravando(), { tipo: 'fim', ok: false, frase: 'Gravação cancelada. Nada foi apagado.' });
  assert.equal(e.tela, 'erro');
  assert.equal(e.frase, 'Gravação cancelada. Nada foi apagado.');
});

test('fim ok:true leva para pronto com a frase', () => {
  const e = F.passo(ateGravando(), { tipo: 'fim', ok: true, frase: 'Pronto.' });
  assert.equal(e.tela, 'pronto');
  assert.equal(e.frase, 'Pronto.');
});

test('fim fora de gravando é ignorado', () => {
  const e = ateConfirmar();
  assert.deepEqual(F.passo(e, { tipo: 'fim', ok: false, frase: 'x' }), e);
});

test('outro em pronto volta para o inicial', () => {
  const e = F.passo(F.passo(ateGravando(), { tipo: 'fim', ok: true, frase: 'Pronto.' }), { tipo: 'outro' });
  assert.deepEqual(e, F.inicial());
});

test('outro em erro (Tentar de novo) volta para o inicial', () => {
  const e = F.passo(F.passo(ateGravando(), { tipo: 'fim', ok: false, frase: 'x' }), { tipo: 'outro' });
  assert.deepEqual(e, F.inicial());
});

test('passo não altera o estado recebido', () => {
  const e = rodar({ tipo: 'discos', lista: [ok, aviso] });
  const copia = JSON.parse(JSON.stringify(e));
  F.passo(e, { tipo: 'escolher', id: 'disk4' });
  assert.deepEqual(e, copia);
});

test('etapa da barra', () => {
  assert.equal(F.etapa('plugue'), 1);
  assert.equal(F.etapa('confirmar'), 2);
  assert.equal(F.etapa('gravando'), 3);
  assert.equal(F.etapa('erro'), 3);
  assert.equal(F.etapa('pronto'), 4);
});

test('texto dos arquivos na confirmação', () => {
  assert.equal(F.textoArquivos(ok), 'O pen drive <b>SanDisk 32 GB</b> tem 3 arquivos. Eles vão sumir.');
  assert.equal(F.textoArquivos({ ...ok, arquivos: 0 }), 'O pen drive <b>SanDisk 32 GB</b> está vazio.');
  assert.equal(F.textoArquivos({ ...ok, arquivos: 1 }), 'O pen drive <b>SanDisk 32 GB</b> tem 1 arquivo. Ele vai sumir.');
  assert.equal(F.textoArquivos({ ...ok, arquivos: 2345 }), 'O pen drive <b>SanDisk 32 GB</b> tem 2.345 arquivos. Eles vão sumir.');
  assert.equal(F.textoArquivos({ ...ok, arquivos: 10000, maisDe: true }), 'O pen drive <b>SanDisk 32 GB</b> tem mais de 10.000 arquivos. Eles vão sumir.');
});

test('sem acesso aos arquivos, a confirmação não diz que está vazio', () => {
  assert.equal(F.textoArquivos({ ...ok, arquivos: 0, semAcesso: true }),
    'Não consegui ver os arquivos do pen drive <b>SanDisk 32 GB</b>. Tudo que estiver nele será apagado.');
  assert.equal(F.textoArquivos({ ...ok, arquivos: 0, semAcesso: true, rotulo: '<x>' }),
    'Não consegui ver os arquivos do pen drive <b>&lt;x&gt;</b>. Tudo que estiver nele será apagado.');
});

test('o rótulo é escapado na confirmação', () => {
  assert.equal(F.textoArquivos({ ...ok, rotulo: 'A<b>&"' }), 'O pen drive <b>A&lt;b&gt;&amp;&quot;</b> tem 3 arquivos. Eles vão sumir.');
});

test('texto do tempo restante', () => {
  assert.equal(F.textoRestante(-1), '');
  assert.equal(F.textoRestante(undefined), '');
  assert.equal(F.textoRestante(0), 'Falta menos de um minuto.');
  assert.equal(F.textoRestante(59), 'Falta menos de um minuto.');
  assert.equal(F.textoRestante(80), 'Falta cerca de um minuto.');
  assert.equal(F.textoRestante(480), 'Faltam uns 8 minutos.');
  assert.equal(F.textoRestante(500), 'Faltam uns 8 minutos.');
});

test('título do gravando com a porcentagem', () => {
  assert.equal(F.tituloGravando(null), 'Gravando... 0%');
  assert.equal(F.tituloGravando({ fracao: 0.426 }), 'Gravando... 42%');
  assert.equal(F.tituloGravando({ fracao: 1 }), 'Gravando... 100%');
  assert.equal(F.tituloGravando({ fracao: 1.4 }), 'Gravando... 100%');
});

test('título do pronto', () => {
  assert.equal(F.tituloPronto('Pronto.'), 'Pronto, pode tirar o pen drive.');
  assert.equal(F.tituloPronto(''), 'Pronto, pode tirar o pen drive.');
  assert.equal(F.tituloPronto('Pronto. Ejete o pen drive pelo sistema antes de tirar.'), 'Pronto. Ejete o pen drive pelo sistema antes de tirar.');
});

test('textos fixos das telas', () => {
  assert.equal(F.TEXTOS.procurando, 'Procurando pen drive...');
  assert.equal(F.TEXTOS.pequeno, 'Pequeno demais: precisa de pelo menos 4 GB.');
  assert.equal(F.TEXTOS.aviso, 'Funciona, mas um de 16 GB ou mais é melhor.');
  assert.equal(F.TEXTOS.proprio, 'O instalador está rodando deste pen drive. Copie o instalador para o computador e abra de lá.');
  assert.equal(F.TEXTOS.semDiscos, 'Não consegui ver os discos. Tire o pen drive e plugue de novo.');
});

test('o HTML tem os textos exatos e nenhum emoji ou travessão', () => {
  const fs = require('node:fs'), path = require('node:path');
  const dist = path.join(__dirname, '..', 'installer', 'frontend', 'dist');
  const html = fs.readFileSync(path.join(dist, 'index.html'), 'utf8');
  const tela = fs.readFileSync(path.join(dist, 'tela.js'), 'utf8');
  const fluxo = fs.readFileSync(path.join(dist, 'fluxo.js'), 'utf8');
  const tudo = html + tela + fluxo;
  for(const t of ['Plugue o pen drive', 'Use um de 16 GB ou mais. Tudo que estiver nele será apagado.', 'Continuar',
    'Confirme antes de apagar', 'Pode apagar', 'Voltar', 'Gravar pen drive', 'Pode deixar rodando.', 'Cancelar',
    'Gravar outro', 'Não deu certo', 'Tentar de novo', 'CD2 IA de Bolso']){
    assert.ok(tudo.includes(t), 'falta o texto: ' + t);
  }
  assert.doesNotMatch(tudo, /[—–]/, 'sem travessão');
  assert.doesNotMatch(tudo, /\p{Extended_Pictographic}/u, 'sem emoji');
  assert.match(html, /<script src="fluxo\.js"><\/script>\s*<script src="mascote\.js"><\/script>\s*<script src="tela\.js"><\/script>/);
});
