'use strict';
// Testes do fluxo das telas do instalador (installer/frontend/dist/fluxo.js). Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const F = require('../installer/frontend/dist/fluxo.js');

const ok = { id: 'disk4', nome: 'SanDisk Ultra', bytes: 32e9, situacao: 'ok', arquivos: 3, maisDe: false, rotulo: 'SanDisk 32 GB' };
const aviso = { id: 'disk6', nome: 'Multilaser', bytes: 8e9, situacao: 'aviso', arquivos: 0, maisDe: false, rotulo: 'Multilaser 8 GB' };
const pequeno = { id: 'disk5', nome: 'Kingston', bytes: 2e9, situacao: 'pequeno', arquivos: 0, maisDe: false, rotulo: 'Kingston 2 GB' };

// aplica uma sequência de ações a partir da tela plugue (o pen drive escolhido na primeira tela)
const noPlugue = () => F.passo(F.inicial(), { tipo: 'escolherPendrive' });
const rodar = (...acoes) => acoes.reduce(F.passo, noPlugue());
const ateConfirmar = () => rodar({ tipo: 'discos', lista: [ok] }, { tipo: 'continuar' });
const ateGravando = () => [{ tipo: 'marcar', valor: true }, { tipo: 'gravar' }].reduce(F.passo, ateConfirmar());

test('inicial é escolha', () => {
  const e = F.inicial();
  assert.equal(e.tela, 'escolha');
  assert.deepEqual(e.discos, []);
  assert.equal(e.escolhido, null);
  assert.equal(e.podeApagar, false);
});

test('escolherPendrive vai para plugue', () => {
  const e = noPlugue();
  assert.equal(e.tela, 'plugue');
  assert.deepEqual(e.discos, []);
  assert.equal(e.falhas, 0);
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

test('outro volta para o pen drive', () => {
  const e = F.passo(F.passo(ateGravando(), { tipo: 'fim', ok: true, frase: 'Pronto.' }), { tipo: 'outro' });
  assert.deepEqual(e, noPlugue());
});

test('outro em erro (Tentar de novo) volta para o pen drive', () => {
  const e = F.passo(F.passo(ateGravando(), { tipo: 'fim', ok: false, frase: 'x' }), { tipo: 'outro' });
  assert.deepEqual(e, noPlugue());
});

test('passo não altera o estado recebido', () => {
  const e = rodar({ tipo: 'discos', lista: [ok, aviso] });
  const copia = JSON.parse(JSON.stringify(e));
  F.passo(e, { tipo: 'escolher', id: 'disk4' });
  assert.deepEqual(e, copia);
});

test('etapa da barra', () => {
  // Só as telas do pen drive têm etapa; nas outras a barra some.
  for(const t of ['escolha', 'preparando', 'condicoes', 'aguardando', 'colar', 'reinstalar', 'perguntas', 'instalando', 'instalado', 'naoInstalou', 'bloqueado']){
    assert.equal(F.etapa(t), 0, t);
  }
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

// ---------- instalar neste computador ----------

const info = (extra) => ({ codigo: 'ABCD-1234', temChave: false, worker: 'https://w', sandbox: false, precisa: 7.6e9, livre: 50e9, cabe: true, bloqueio: '', ...extra });
const naEscolha = (...acoes) => acoes.reduce(F.passo, F.inicial());
const nasCondicoes = (extra) => naEscolha({ tipo: 'escolherInstalar' }, { tipo: 'info', info: info(extra) });

test('escolherInstalar espera a conta deste computador', () => {
  const e = naEscolha({ tipo: 'escolherInstalar' });
  assert.equal(e.tela, 'preparando');
});

test('escolherInstalar sem chave vai para condicoes', () => {
  const e = nasCondicoes();
  assert.equal(e.tela, 'condicoes');
  assert.equal(e.entendi, false);
  assert.equal(e.info.codigo, 'ABCD-1234');
});

test('escolherInstalar com chave vai para reinstalar', () => {
  assert.equal(nasCondicoes({ temChave: true }).tela, 'reinstalar');
  // a info também pode vir junto da ação
  assert.equal(naEscolha({ tipo: 'escolherInstalar', info: info({ temChave: true }) }).tela, 'reinstalar');
  assert.equal(naEscolha({ tipo: 'escolherInstalar', info: info() }).tela, 'condicoes');
});

test('bloqueio vai para bloqueado com a frase', () => {
  const frase = 'Abra o instalador de novo sem ser como administrador (clique duplo normal) para instalar neste computador.';
  const e = naEscolha({ tipo: 'escolherInstalar' }, { tipo: 'bloqueio', frase });
  assert.equal(e.tela, 'bloqueado');
  assert.equal(e.frase, frase);
  assert.equal(nasCondicoes({ bloqueio: frase }).tela, 'bloqueado');
  assert.equal(F.passo(e, { tipo: 'inicio' }).tela, 'escolha');
});

test('condicoes sem espaço não deixa pagar', () => {
  const e = F.passo(nasCondicoes({ cabe: false, precisa: 7.6e9, livre: 3.24e9 }), { tipo: 'entender', valor: true });
  assert.equal(F.podePagar(e), false);
  assert.deepEqual(F.passo(e, { tipo: 'pagar' }), e);
  assert.equal(F.textoEspaco(e.info), 'Falta espaço: a instalação precisa de uns 7,6 GB livres e este computador tem 3,2 GB.');
  assert.equal(F.textoEspaco(info()), '');
});

test('pagar só com Entendi marcado', () => {
  const e = nasCondicoes();
  assert.equal(F.podePagar(e), false);
  assert.deepEqual(F.passo(e, { tipo: 'pagar' }), e);
  const marcado = F.passo(e, { tipo: 'entender', valor: true });
  assert.equal(F.podePagar(marcado), true);
  const pagando = F.passo(marcado, { tipo: 'pagar' });
  assert.equal(pagando.tela, 'aguardando');
  // desmarcar volta a travar
  assert.equal(F.passo(F.passo(marcado, { tipo: 'entender', valor: false }), { tipo: 'pagar' }).tela, 'condicoes');
});

test('falha ao abrir o pagamento volta para condicoes com a frase', () => {
  const e = [{ tipo: 'entender', valor: true }, { tipo: 'pagar' }, { tipo: 'falhaPagar', frase: 'Não deu certo.' }].reduce(F.passo, nasCondicoes());
  assert.equal(e.tela, 'condicoes');
  assert.equal(e.erro, 'Não deu certo.');
});

test('Já tenho a chave vai para colar, e voltar volta para condicoes', () => {
  const e = F.passo(nasCondicoes(), { tipo: 'jaTenho' });
  assert.equal(e.tela, 'colar');
  assert.equal(F.passo(e, { tipo: 'voltar' }).tela, 'condicoes');
  const ag = [{ tipo: 'entender', valor: true }, { tipo: 'pagar' }, { tipo: 'voltar' }].reduce(F.passo, nasCondicoes());
  assert.equal(ag.tela, 'condicoes');
});

test('chave ok vai para perguntas', () => {
  const ag = [{ tipo: 'entender', valor: true }, { tipo: 'pagar' }].reduce(F.passo, nasCondicoes());
  const e = F.passo(ag, { tipo: 'chave', ok: true });
  assert.equal(e.tela, 'perguntas');
  assert.equal(e.area, true);
  assert.equal(e.inicio, true);
  // a chave colada crua não entra no estado: a tela final mostra a de MinhaChave()
  const c = F.passo(F.passo(nasCondicoes(), { tipo: 'jaTenho' }), { tipo: 'chave', ok: true, chave: ' cd2-xyz ' });
  assert.equal(c.tela, 'perguntas');
  assert.equal(c.chave, '');
  // a chave que volta enquanto a pessoa ainda está nas condições também segue
  assert.equal(F.passo(nasCondicoes(), { tipo: 'chave', ok: true }).tela, 'perguntas');
});

test('chave recusada fica na tela com o erro', () => {
  const c = F.passo(nasCondicoes(), { tipo: 'jaTenho' });
  const e = F.passo(c, { tipo: 'chave', ok: false, erro: 'Esta chave não é deste computador.' });
  assert.equal(e.tela, 'colar');
  assert.equal(e.erro, 'Esta chave não é deste computador.');
});

test('chave fora do caminho de instalar é ignorada', () => {
  const p = noPlugue();
  assert.deepEqual(F.passo(p, { tipo: 'chave', ok: true }), p);
  const g = ateGravando();
  assert.deepEqual(F.passo(g, { tipo: 'chave', ok: true }), g);
});

test('reinstalar vai para perguntas, mas não sem espaço', () => {
  assert.equal(F.passo(nasCondicoes({ temChave: true }), { tipo: 'reinstalar' }).tela, 'perguntas');
  const sem = nasCondicoes({ temChave: true, cabe: false });
  assert.deepEqual(F.passo(sem, { tipo: 'reinstalar' }), sem);
  assert.equal(F.passo(sem, { tipo: 'inicio' }).tela, 'escolha');
});

test('perguntas: as duas vêm marcadas e podem ser desmarcadas', () => {
  let e = F.passo(nasCondicoes({ temChave: true }), { tipo: 'reinstalar' });
  e = F.passo(e, { tipo: 'opcao', nome: 'area', valor: false });
  e = F.passo(e, { tipo: 'opcao', nome: 'inicio', valor: false });
  assert.equal(e.area, false);
  assert.equal(e.inicio, false);
  assert.deepEqual(F.passo(e, { tipo: 'opcao', nome: 'outra', valor: true }), e);
});

const ateInstalando = () => [{ tipo: 'reinstalar' }, { tipo: 'instalar' }].reduce(F.passo, nasCondicoes({ temChave: true }));

test('instalar e fim ok vai para instalado', () => {
  const i = ateInstalando();
  assert.equal(i.tela, 'instalando');
  assert.equal(i.progresso, null);
  const ev = { etapa: 'baixando', frase: 'Baixando o modelo (1,6 de 3,9 GB)', fracao: 0.42, restante: 480 };
  const p = F.passo(i, { tipo: 'progresso', evento: ev });
  assert.deepEqual(p.progresso, ev);
  assert.equal(F.tituloInstalando(p.progresso), 'Instalando... 42%');
  const e = F.passo(p, { tipo: 'fim', ok: true, frase: 'Pronto.' });
  assert.equal(e.tela, 'instalado');
});

test('fim com erro na instalação vai para naoInstalou, e tentar de novo volta para as perguntas', () => {
  const e = F.passo(ateInstalando(), { tipo: 'fim', ok: false, frase: 'Sem internet.' });
  assert.equal(e.tela, 'naoInstalou');
  assert.equal(e.frase, 'Sem internet.');
  assert.equal(F.passo(e, { tipo: 'outro' }).tela, 'perguntas');
});

test('texto do instalado e erro ao abrir a IA', () => {
  assert.equal(F.textoInstalado(true), 'A partir de agora, abra a IA de Bolso pelo ícone nos programas ou na área de trabalho.');
  assert.equal(F.textoInstalado(false), 'A partir de agora, abra a IA de Bolso pelo ícone nos programas.');
  const e = F.passo(F.passo(ateInstalando(), { tipo: 'fim', ok: true }), { tipo: 'falhaAbrir', frase: 'Não consegui abrir a IA de Bolso. Abra pelo ícone nos programas.' });
  assert.equal(e.tela, 'instalado');
  assert.equal(e.erro, 'Não consegui abrir a IA de Bolso. Abra pelo ícone nos programas.');
});

test('os dois caminhos não se misturam', () => {
  // instalando, a busca de discos e as ações do pen drive não mexem em nada
  const i = ateInstalando();
  for(const a of [{ tipo: 'discos', lista: [ok] }, { tipo: 'continuar' }, { tipo: 'gravar' }, { tipo: 'outro' }, { tipo: 'escolherPendrive' }]){
    assert.deepEqual(F.passo(i, a), i, a.tipo);
  }
  // no pen drive, as ações de instalar também não
  const g = ateGravando();
  for(const a of [{ tipo: 'pagar' }, { tipo: 'instalar' }, { tipo: 'reinstalar' }, { tipo: 'escolherInstalar' }, { tipo: 'info', info: info() }]){
    assert.deepEqual(F.passo(g, a), g, a.tipo);
  }
});

test('janela reaberta sem administrador não segue para o pen drive', () => {
  const e = F.passo(F.inicial({ semAdmin: true }), { tipo: 'escolherPendrive' });
  assert.equal(e.tela, 'escolha');
  assert.equal(e.avisoPendrive, true);
  assert.equal(F.TEXTOS.semAdmin, 'Esta janela não pode gravar pen drive. Para criar o pen drive, feche o instalador e abra de novo com clique duplo.');
  // mesmo que chegue ao plugue de algum jeito, não continua nem grava
  const p = [{ tipo: 'discos', lista: [ok] }].reduce(F.passo, { ...noPlugue(), semAdmin: true });
  assert.equal(F.podeContinuar(p), false);
  assert.deepEqual(F.passo(p, { tipo: 'continuar' }), p);
  const c = { ...ateConfirmar(), podeApagar: true, semAdmin: true };
  assert.deepEqual(F.passo(c, { tipo: 'gravar' }), c);
  // e instalar continua valendo
  assert.equal(F.passo(F.inicial({ semAdmin: true }), { tipo: 'escolherInstalar' }).tela, 'preparando');
});

test('Agora não volta para a escolha', () => {
  assert.equal(F.passo(nasCondicoes(), { tipo: 'inicio' }).tela, 'escolha');
  assert.equal(F.passo(nasCondicoes(), { tipo: 'inicio' }).entendi, false);
  // nada de voltar para a escolha no meio da instalação ou da gravação
  const i = ateInstalando();
  assert.deepEqual(F.passo(i, { tipo: 'inicio' }), i);
});

test('cancelar a instalação volta para as perguntas, sem passar pelo erro', () => {
  const i = ateInstalando();
  const c = F.passo(i, { tipo: 'cancelar' });
  assert.equal(c.tela, 'instalando');
  assert.equal(c.cancelando, true);
  const e = F.passo(c, { tipo: 'fim', ok: false, frase: 'Instalação cancelada.' });
  assert.equal(e.tela, 'perguntas');
  assert.equal(e.cancelando, false);
  assert.equal(e.frase, '');
  // terminou antes do cancelamento pegar: fica instalado
  assert.equal(F.passo(c, { tipo: 'fim', ok: true }).tela, 'instalado');
  // sem cancelar, o erro continua indo para naoInstalou
  assert.equal(F.passo(i, { tipo: 'fim', ok: false, frase: 'x' }).tela, 'naoInstalou');
  // cancelar a gravação do pen drive continua como antes
  const g = ateGravando();
  assert.equal(F.passo(F.passo(g, { tipo: 'cancelar' }), { tipo: 'fim', ok: false, frase: 'Gravação cancelada. Nada foi apagado.' }).tela, 'erro');
});

test('minhaChave guarda a chave normalizada para a tela final e o reinstalar', () => {
  const r = F.passo(nasCondicoes({ temChave: true }), { tipo: 'minhaChave', chave: 'CD2-ABC' });
  assert.equal(r.chave, 'CD2-ABC');
  const fim = F.passo(F.passo(ateInstalando(), { tipo: 'fim', ok: true }), { tipo: 'minhaChave', chave: 'CD2-DEF' });
  assert.equal(fim.chave, 'CD2-DEF');
  assert.equal(F.passo(fim, { tipo: 'minhaChave', chave: '' }).chave, '');
  // no pen drive não muda nada
  const p = noPlugue();
  assert.deepEqual(F.passo(p, { tipo: 'minhaChave', chave: 'x' }), p);
});

test('Agora não nas perguntas volta para a escolha', () => {
  const e = F.passo(F.passo(nasCondicoes({ temChave: true }), { tipo: 'reinstalar' }), { tipo: 'inicio' });
  assert.equal(e.tela, 'escolha');
});

test('Voltar no plugue volta para a escolha', () => {
  const e = F.passo(rodar({ tipo: 'discos', lista: [ok] }), { tipo: 'inicio' });
  assert.equal(e.tela, 'escolha');
  // e o pen drive recomeça do zero
  assert.deepEqual(F.passo(e, { tipo: 'escolherPendrive' }).discos, []);
  // no meio da gravação, não
  const g = ateGravando();
  assert.deepEqual(F.passo(g, { tipo: 'inicio' }), g);
});

test('Pagamento confirmado só depois de pagar ou colar a chave', () => {
  const pagou = F.passo([{ tipo: 'entender', valor: true }, { tipo: 'pagar' }].reduce(F.passo, nasCondicoes()), { tipo: 'chave', ok: true });
  assert.equal(pagou.pago, true);
  const colou = F.passo(F.passo(nasCondicoes(), { tipo: 'jaTenho' }), { tipo: 'chave', ok: true });
  assert.equal(colou.pago, true);
  const reinst = F.passo(nasCondicoes({ temChave: true }), { tipo: 'reinstalar' });
  assert.equal(reinst.pago, false);
  // voltar para a escolha esquece
  assert.equal(F.passo(pagou, { tipo: 'inicio' }).pago, false);
});

test('instalado com aviso dos atalhos não promete atalho', () => {
  const aviso = 'A IA foi instalada, mas não consegui criar todos os atalhos.';
  const e = F.passo(ateInstalando(), { tipo: 'fim', ok: true, frase: aviso, aviso });
  assert.equal(e.tela, 'instalado');
  assert.equal(e.aviso, aviso);
  assert.equal(F.textoInstalado(e.area, e.aviso), 'A IA de Bolso está instalada neste computador.');
  assert.doesNotMatch(F.textoInstalado(true, aviso), /área de trabalho|ícone/);
  // sem aviso, o texto de sempre; e o aviso não vaza para a próxima instalação
  const ok = F.passo(ateInstalando(), { tipo: 'fim', ok: true, frase: 'Pronto. A IA de Bolso está instalada neste computador.' });
  assert.equal(ok.aviso, '');
  assert.equal(F.textoInstalado(true, ok.aviso), 'A partir de agora, abra a IA de Bolso pelo ícone nos programas ou na área de trabalho.');
});

test('gb legível', () => {
  assert.equal(F.gb(7.6e9), '7,6');
  assert.equal(F.gb(0), '0,0');
  assert.equal(F.gb(123.45e9), '123,5');
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
    'Gravar outro', 'Não deu certo', 'Tentar de novo', 'CD2 IA de Bolso',
    'Criar o pen drive da IA de Bolso', 'Grátis', 'Instalar neste computador', 'compre a cópia, R$ 25',
    'Cópia para este computador', 'Instale a IA de Bolso neste computador', 'pagamento único, por instalação',
    'O que está incluído', 'Como funciona', 'Código deste computador', 'Entendi que a instalação vale só para este computador.',
    'Agora não', 'Ir para o pagamento', 'Já tenho a chave', 'Aguardando o pagamento...', 'Reinstalar com a minha chave',
    'Tudo certo! Só duas perguntinhas', 'Copiar a chave', 'Abrir a IA de Bolso', 'modo de teste',
    'Pagamento confirmado', 'Guarde. Ela reinstala neste computador quando precisar.']){
    assert.ok(tudo.includes(t), 'falta o texto: ' + t);
  }
  assert.doesNotMatch(tudo, /[—–]/, 'sem travessão');
  assert.doesNotMatch(tudo, /\p{Extended_Pictographic}/u, 'sem emoji');
  assert.match(html, /<script src="fluxo\.js"><\/script>\s*<script src="mascote\.js"><\/script>\s*<script src="tela\.js"><\/script>/);
});
