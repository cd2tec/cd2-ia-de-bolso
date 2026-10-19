/* Desenha as telas do instalador a partir do estado do fluxo (fluxo.js) e fala com o Go pela ponte do
   Wails (window.go.main.App e window.runtime). Aberta direto no navegador, sem Wails, usa um backend
   de demonstração que não toca em disco nenhum, só para conferir as telas. */
(function(){
"use strict";
const F = window.CD2Fluxo;
const M = window.CD2Mascote;
const $ = id => document.getElementById(id);
const esc = F.escapar;

// ---------- backends ----------

function backendWails(){
  const A = window.go.main.App;
  return {
    listar: () => A.ListarDiscos(),
    gravar: d => A.Gravar(d),
    cancelar: () => A.Cancelar(),
    ouvir: (nome, fn) => window.runtime.EventsOn(nome, fn),
    comInstalar: () => A.IniciouComInstalar(),
    escolherInstalar: () => A.EscolherInstalar(),
    info: () => A.InfoInstalar(),
    pagar: () => A.IrParaPagamento(),
    colar: k => A.ColarChave(k),
    instalar: (area, inicio) => A.InstalarComputador(area, inicio),
    abrirIA: () => A.AbrirIA(),
    minhaChave: () => A.MinhaChave(),
    copiar: t => window.runtime.ClipboardSetText(t),
  };
}

// Demonstração: dois discos (um bom e um pequeno), uma gravação simulada de uns 10 segundos e o
// caminho de instalar com pagamento e instalação de mentira.
// Variações pelo endereço: ?demo=um, ?demo=aviso, ?demo=nenhum, ?demo=muitos, ?demo=erro, ?demo=falha
// (pen drive) e ?demo=chave, ?demo=cheio, ?demo=bloqueio, ?demo=reaberta, ?demo=erroInstalar (instalar).
function backendDemo(){
  const modo = new URLSearchParams(location.search).get('demo') || '';
  const bom = { id: 'disk4', nome: 'SanDisk Ultra', bytes: 32e9, situacao: 'ok', arquivos: 3, maisDe: false, rotulo: 'SanDisk 32 GB' };
  const aviso = { id: 'disk6', nome: 'Multilaser', bytes: 8e9, situacao: 'aviso', arquivos: 0, maisDe: false, rotulo: 'Multilaser 8 GB' };
  const pequeno = { id: 'disk5', nome: 'Kingston', bytes: 2e9, situacao: 'pequeno', arquivos: 12, maisDe: false, rotulo: 'Kingston 2 GB' };
  const cheio = { id: 'disk7', nome: 'Sony', bytes: 64e9, situacao: 'ok', arquivos: 10000, maisDe: true, rotulo: 'Sony 64 GB' };
  const proprio = { id: 'disk8', nome: 'Kingston', bytes: 32e9, situacao: 'proprio', arquivos: 2, maisDe: false, rotulo: 'Kingston 32 GB' };
  const fechado = { id: 'disk9', nome: 'Lexar', bytes: 32e9, situacao: 'ok', arquivos: 0, maisDe: false, semAcesso: true, rotulo: 'Lexar 32 GB' };
  const lista = { um: [bom], aviso: [aviso], nenhum: [], muitos: [bom, aviso, pequeno, cheio, proprio, fechado] }[modo] || [bom, pequeno];
  const ouvintes = {};
  const emitir = (nome, dado) => (ouvintes[nome] || []).forEach(fn => fn(dado));
  const esperar = (ms, valor) => new Promise(r => setTimeout(() => r(valor), ms));
  let relogio = null, fracao = 0, cancelado = null;
  const CHAVE_DEMO = 'CD2-K7QF2M9X4TPA-8H3J-QW2E-RT5Y-ZX7C-VB9N-ML4K-P2D6-S8F1';
  let guardada = modo === 'chave' ? CHAVE_DEMO : '';
  const gb = x => x.toFixed(1).replace('.', ',');
  function fim(ok, frase){ clearInterval(relogio); relogio = null; emitir('fim', { ok, frase }); }
  // Uns 10 segundos de baixar, depois a etapa do meio (copiar ou instalar) e a de conferir.
  function simular(meio, falhar){
    if(relogio) return Promise.reject('Já tem uma gravação em andamento.');
    fracao = 0;
    const inicio = Date.now();
    emitir('progresso', { etapa: 'baixando', frase: 'Baixando o modelo (0,0 de 3,9 GB)', fracao: 0, restante: -1 });
    relogio = setInterval(() => {
      fracao = Math.min(1, (Date.now() - inicio) / 10000);
      if(falhar && fracao >= 0.3) return fim(false, 'Não consegui baixar o que falta. Confira a internet e tente de novo.');
      const restante = Math.round((1 - fracao) * 600);
      if(fracao < 0.5) emitir('progresso', { etapa: 'baixando', frase: 'Baixando o modelo (' + gb(fracao * 2 * 3.9) + ' de 3,9 GB)', fracao, restante });
      else if(fracao < 0.85) emitir('progresso', { etapa: 'copiando', frase: meio, fracao, restante });
      else if(fracao < 1) emitir('progresso', { etapa: 'conferindo', frase: 'Conferindo o que foi gravado', fracao, restante: Math.round((1 - fracao) * 200) });
      else { emitir('progresso', { etapa: 'pronto', frase: 'Pronto.', fracao: 1, restante: 0 }); fim(true, 'Pronto.'); }
    }, 250);
    return Promise.resolve(null);
  }
  return {
    listar: () => new Promise((r, rej) => setTimeout(() => modo === 'falha' ? rej('sem discos') : r(lista.map(d => ({ ...d }))), 300)),
    gravar: () => { cancelado = 'gravar'; return simular('Copiando para o pen drive', modo === 'erro'); },
    cancelar: () => {
      if(!relogio) return Promise.resolve();
      setTimeout(() => fim(false, cancelado === 'instalar' ? 'Instalação cancelada. Nada foi instalado.' : fracao < 0.5
        ? 'Gravação cancelada. Nada foi apagado.'
        : 'Gravação cancelada. O pen drive ficou incompleto. Grave de novo antes de usar.'), 400);
      return Promise.resolve();
    },
    ouvir: (nome, fn) => { (ouvintes[nome] = ouvintes[nome] || []).push(fn); },
    comInstalar: () => Promise.resolve(modo === 'reaberta'),
    escolherInstalar: () => esperar(200, modo === 'bloqueio'
      ? 'Abra o instalador de novo sem ser como administrador (clique duplo normal) para instalar neste computador.' : ''),
    info: () => esperar(1200, {
      codigo: 'K7QF-2M9X-4TPA', temChave: modo === 'chave', worker: 'https://exemplo', sandbox: true,
      precisa: 7.6e9, livre: modo === 'cheio' ? 3.24e9 : 120e9, cabe: modo !== 'cheio', bloqueio: '',
    }),
    // O "pagamento" volta sozinho depois de uns segundos.
    pagar: () => { setTimeout(() => { guardada = CHAVE_DEMO; emitir('chave', { ok: true, erro: '' }); }, 4000); return Promise.resolve(null); },
    colar: k => {
      const erro = String(k).trim().length < 8 ? 'Esta chave não está completa. Confira se copiou ela inteira.'
        : /outro/i.test(k) ? 'Esta chave não é deste computador.' : '';
      if(!erro) guardada = String(k).trim().toUpperCase();
      return esperar(300, erro);
    },
    minhaChave: () => esperar(100, guardada),
    instalar: () => { cancelado = 'instalar'; return simular('Instalando neste computador', modo === 'erroInstalar'); },
    abrirIA: () => Promise.reject('Na demonstração não tem IA instalada para abrir.'),
    copiar: t => navigator.clipboard ? navigator.clipboard.writeText(t) : Promise.reject('sem área de transferência'),
  };
}

// ---------- ícones (SVG, sem emoji) ----------

const ICONE_PENDRIVE = '<svg class="icone" viewBox="0 0 28 28" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round" aria-hidden="true">'
  + '<rect x="9" y="3" width="10" height="7" rx="1.5"/><path d="M12 6h1M15 6h1" stroke-linecap="round"/>'
  + '<rect x="6.5" y="10" width="15" height="15" rx="3"/></svg>';
const ICONE_COMPUTADOR = '<svg class="icone" viewBox="0 0 28 28" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round" stroke-linecap="round" aria-hidden="true">'
  + '<rect x="3.5" y="5" width="21" height="14" rx="2"/><path d="M10 23.5h8M14 19v4.5"/></svg>';
const ICONE_OK = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M5 12.5l4.5 4.5L19 7.5"/></svg>';
const ICONE_ERRO = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.8" stroke-linecap="round" aria-hidden="true"><path d="M12 6.5v7"/><path d="M12 17.5v.01"/></svg>';

// ---------- telas ----------
// Cada tela tem montar (o esqueleto, uma vez ao entrar) e atualizar (só o que muda com o estado).

const telas = {
  escolha: {
    montar: () => '<h1 tabindex="-1">Como você quer usar a IA de Bolso?</h1>'
      + '<div class="cartoes">'
      + '<button type="button" class="cartao" data-acao="escolherPendrive"><span class="cartao-topo">' + ICONE_PENDRIVE
      + '<span class="selo-cartao gratis">Grátis</span></span>'
      + '<b>Criar o pen drive da IA de Bolso</b>'
      + '<small>Grava a IA num pen drive para usar onde você estiver.</small></button>'
      + '<button type="button" class="cartao" data-acao="escolherInstalar"><span class="cartao-topo">' + ICONE_COMPUTADOR
      + '<span class="selo-cartao preco">compre a cópia, R$ 25</span></span>'
      + '<b>Instalar neste computador</b>'
      + '<small>A IA passa a morar aqui e abre sem precisar de pen drive.</small></button>'
      + '</div>'
      + '<p class="nota" id="avisoPendrive" role="status" hidden>' + esc(F.TEXTOS.semAdmin) + '</p>',
    atualizar: e => { $('avisoPendrive').hidden = !e.avisoPendrive; },
  },
  preparando: {
    montar: () => '<h1 tabindex="-1">Conferindo este computador...</h1>'
      + '<p class="texto">Vendo o código deste computador e se a instalação cabe no disco.</p>'
      + '<div class="procurando espera" role="status"><span class="boneco">' + M.mascoteSvg() + '</span>Só um instante.</div>',
    atualizar: () => {},
  },
  bloqueado: {
    montar: e => '<div class="final"><div class="selo erro">' + ICONE_ERRO + '</div>'
      + '<h1 tabindex="-1">Não dá para instalar por esta janela</h1>'
      + '<p class="texto">' + esc(e.frase) + '</p></div>'
      + '<div class="acoes"><button type="button" class="btn primario" data-acao="inicio">Voltar</button></div>',
    atualizar: () => {},
  },
  condicoes: {
    montar: e => '<div class="rolagem">'
      + '<span class="sobre">Cópia para este computador</span><span class="regua" aria-hidden="true"></span>'
      + '<h1 tabindex="-1">Instale a IA de Bolso neste computador</h1>'
      + '<p class="texto">Com a instalação, a IA passa a morar neste computador e abre sem precisar de pen drive.</p>'
      + '<div class="preco"><b>R$ 25</b><span>pagamento único, por instalação</span></div>'
      + '<p class="aviso-erro" id="espaco" role="status" hidden></p>'
      + '<div class="sub">O que está incluído</div>'
      + '<ul class="itens">'
      + '<li>A IA baixada e instalada neste computador (uns 3,9 GB na primeira vez).</li>'
      + '<li>Ícone nos programas e, se você quiser, na área de trabalho.</li>'
      + '<li>Se você quiser, ela acorda junto com o computador (ocupa de 4 a 5 GB de memória desde que ele liga). Dá para desligar depois.</li>'
      + '<li>Desinstalar quando quiser.</li>'
      + '</ul>'
      + '<div class="sub">Como funciona</div>'
      + '<ul class="itens cuidado">'
      + '<li>É uma cópia avulsa, paga uma vez. Não é assinatura nem licença.</li>'
      + '<li>Vale só para este computador. Copiada para outro, ela não abre.</li>'
      + '<li>Você recebe uma chave por e-mail. Com ela, reinstala neste mesmo computador quando precisar.</li>'
      + '<li>Trocou de computador ou perdeu a chave? É uma nova instalação, com um novo pagamento.</li>'
      + '</ul>'
      + '<div class="codigo"><div><small>Código deste computador</small><b>' + esc(e.info && e.info.codigo) + '</b></div><small>vai junto com o pagamento</small></div>'
      + '<p class="aviso-erro" id="erroPagar" role="alert" hidden></p>'
      + '<label class="caixa longa"><input type="checkbox" id="entendi" data-foco="entendi"> <span>Entendi que a instalação vale só para este computador.</span></label>'
      + '<p class="rodape">Pagamento seguro pelo Stripe, no seu navegador.</p>'
      + '</div>'
      + '<div class="acoes"><button type="button" class="link" data-acao="jaTenho">Já tenho a chave</button>'
      + '<button type="button" class="btn secundario" data-acao="inicio">Agora não</button>'
      + '<button type="button" class="btn primario" data-acao="pagar">Ir para o pagamento</button></div>',
    atualizar: e => {
      $('entendi').checked = e.entendi;
      const espaco = F.textoEspaco(e.info);
      $('espaco').textContent = espaco;
      $('espaco').hidden = !espaco;
      $('erroPagar').textContent = e.erro;
      $('erroPagar').hidden = !e.erro;
      document.querySelector('[data-acao="pagar"]').disabled = !F.podePagar(e);
    },
  },
  aguardando: {
    montar: () => '<h1 tabindex="-1">Aguardando o pagamento...</h1>'
      + '<p class="texto">O pagamento abriu no seu navegador. Pagou? A instalação continua aqui sozinha. Se não continuar, cole a chave que chegou no seu e-mail.</p>'
      + campoChave()
      + '<div class="acoes"><button type="button" class="link" data-acao="abrirPagamento">Abrir o pagamento de novo</button>'
      + '<button type="button" class="btn secundario" data-acao="voltar">Voltar</button>'
      + '<button type="button" class="btn primario" data-acao="colarOk">Continuar</button></div>',
    atualizar: e => mostrarErroChave(e),
  },
  colar: {
    montar: () => '<h1 tabindex="-1">Cole a sua chave</h1>'
      + '<p class="texto">A chave chega no seu e-mail depois do pagamento.</p>'
      + campoChave()
      + '<div class="acoes"><button type="button" class="btn secundario" data-acao="voltar">Voltar</button>'
      + '<button type="button" class="btn primario" data-acao="colarOk">Continuar</button></div>',
    atualizar: e => mostrarErroChave(e),
  },
  reinstalar: {
    montar: () => '<h1 tabindex="-1">Este computador já tem a chave</h1>'
      + '<p class="texto">A chave deste computador já está guardada aqui. Reinstalar é livre, sem novo pagamento.</p>'
      + '<div id="blocoChave"></div>'
      + '<p class="aviso-erro" id="espaco" role="status" hidden></p>'
      + '<div class="acoes"><button type="button" class="btn secundario" data-acao="inicio">Agora não</button>'
      + '<button type="button" class="btn primario" data-acao="reinstalar">Reinstalar com a minha chave</button></div>',
    atualizar: e => {
      const espaco = F.textoEspaco(e.info);
      $('espaco').textContent = espaco;
      $('espaco').hidden = !espaco;
      document.querySelector('[data-acao="reinstalar"]').disabled = !(e.info && e.info.cabe);
      trocar($('blocoChave'), blocoChave(e.chave, true));
    },
  },
  perguntas: {
    montar: e => (e.pago ? '<span class="selo-pago"><i></i>Pagamento confirmado</span>' : '')
      + '<h1 tabindex="-1">Tudo certo! Só duas perguntinhas</h1>'
      + '<label class="caixa borda"><input type="checkbox" id="area"> <span>Criar um atalho na área de trabalho<small>O ícone nos programas é criado sempre.</small></span></label>'
      + '<label class="caixa borda"><input type="checkbox" id="inicio"> <span>Acordar quando o computador ligar<small>A IA fica pronta mais rápido, mas ocupa de 4 a 5 GB de memória desde que o computador liga. Dá para desligar depois, no ícone perto do relógio.</small></span></label>'
      + '<div class="acoes"><button type="button" class="btn secundario" data-acao="inicio">Agora não</button>'
      + '<button type="button" class="btn primario" data-acao="instalar">Instalar</button></div>',
    atualizar: e => { $('area').checked = e.area; $('inicio').checked = e.inicio; },
  },
  instalando: {
    montar: () => '<h1 tabindex="-1" id="titulo"></h1>'
      + '<div class="pista"><span class="boneco" id="robo">' + M.roboSvg() + '</span></div>'
      + '<div class="barra" role="progressbar" aria-label="Andamento da instalação" aria-valuemin="0" aria-valuemax="100" id="barra"><i></i></div>'
      + '<p class="frase" id="frase"></p>'
      + '<p class="restante" id="restante"></p>'
      + '<p class="calma">Pode deixar rodando.</p>'
      + '<div class="acoes"><button type="button" class="btn secundario" data-acao="cancelar">Cancelar</button></div>',
    atualizar: e => andamento(e, F.tituloInstalando(e.progresso)),
  },
  instalado: {
    montar: e => '<div class="final"><div class="cabeca-final"><div class="selo ok">' + ICONE_OK + '</div>'
      + '<h1 tabindex="-1">Pronto! Agora eu moro aqui</h1></div>'
      + '<p class="texto">' + esc(F.textoInstalado(e.area)) + '</p>'
      + '<div id="blocoChave" class="bloco-final"></div>'
      + '<p class="aviso-erro" id="erroAbrir" role="status" hidden></p></div>'
      + '<div class="acoes"><button type="button" class="btn secundario" data-acao="copiar" hidden>Copiar a chave</button>'
      + '<button type="button" class="btn primario" data-acao="abrirIA">Abrir a IA de Bolso</button></div>',
    atualizar: e => {
      $('erroAbrir').textContent = e.erro;
      $('erroAbrir').hidden = !e.erro;
      trocar($('blocoChave'), blocoChave(e.chave, false));
      document.querySelector('.acoes [data-acao="copiar"]').hidden = !e.chave;
    },
  },
  naoInstalou: {
    montar: e => '<div class="final"><div class="selo erro">' + ICONE_ERRO + '</div>'
      + '<h1 tabindex="-1">Não deu certo</h1>'
      + '<p class="texto">' + esc(e.frase) + '</p></div>'
      + '<div class="acoes"><button type="button" class="btn primario" data-acao="outro">Tentar de novo</button></div>',
    atualizar: () => {},
  },
  plugue: {
    montar: () => '<h1 tabindex="-1">Plugue o pen drive</h1>'
      + '<p class="texto">Use um de 16 GB ou mais. Tudo que estiver nele será apagado.</p>'
      + '<div id="discos" class="lista-area"></div>'
      + '<div class="acoes"><button type="button" class="link" data-acao="inicio">Voltar</button>'
      + '<button type="button" class="btn primario" data-acao="continuar" data-foco="continuar">Continuar</button></div>',
    atualizar: e => {
      trocar($('discos'), listaDeDiscos(e));
      document.querySelector('[data-acao="continuar"]').disabled = !F.podeContinuar(e);
    },
  },
  confirmar: {
    montar: e => '<h1 tabindex="-1">Confirme antes de apagar</h1>'
      + '<div class="alerta">' + F.textoArquivos(discoEscolhido(e) || { rotulo: '', arquivos: 0 }) + '</div>'
      + '<label class="caixa"><input type="checkbox" id="podeApagar" data-foco="podeApagar"> Pode apagar</label>'
      + '<div class="acoes"><button type="button" class="btn secundario" data-acao="voltar">Voltar</button>'
      + '<button type="button" class="btn primario" data-acao="gravar" data-foco="gravar">Gravar pen drive</button></div>',
    atualizar: e => {
      $('podeApagar').checked = e.podeApagar;
      document.querySelector('[data-acao="gravar"]').disabled = !e.podeApagar;
    },
  },
  gravando: {
    montar: () => '<h1 tabindex="-1" id="titulo"></h1>'
      + '<div class="pista"><span class="boneco" id="robo">' + M.roboSvg() + '</span></div>'
      + '<div class="barra" role="progressbar" aria-label="Andamento da gravação" aria-valuemin="0" aria-valuemax="100" id="barra"><i></i></div>'
      + '<p class="frase" id="frase"></p>'
      + '<p class="restante" id="restante"></p>'
      + '<p class="calma">Pode deixar rodando.</p>'
      + '<div class="acoes"><button type="button" class="btn secundario" data-acao="cancelar">Cancelar</button></div>',
    atualizar: e => andamento(e, F.tituloGravando(e.progresso)),
  },
  pronto: {
    montar: e => '<div class="final"><div class="vitoria"><div class="selo ok">' + ICONE_OK + '</div>'
      + '<span class="boneco">' + M.mascoteSvg() + '</span></div>'
      + '<h1 tabindex="-1">' + esc(F.tituloPronto(e.frase)) + '</h1></div>'
      + '<div class="acoes"><button type="button" class="btn primario" data-acao="outro">Gravar outro</button></div>',
    atualizar: () => {},
  },
  erro: {
    montar: e => '<div class="final"><div class="selo erro">' + ICONE_ERRO + '</div>'
      + '<h1 tabindex="-1">Não deu certo</h1>'
      + '<p class="texto">' + esc(e.frase) + '</p></div>'
      + '<div class="acoes"><button type="button" class="btn primario" data-acao="outro">Tentar de novo</button></div>',
    atualizar: () => {},
  },
};

// Barra, robô, frase e tempo restante da gravação e da instalação.
function andamento(e, titulo){
  const p = F.porcentagem(e.progresso);
  $('titulo').textContent = titulo;
  $('barra').setAttribute('aria-valuenow', String(p));
  $('barra').firstElementChild.style.width = p + '%';
  // O robô anda junto com a ponta da barra, sem sair pelas bordas.
  $('robo').style.left = 'clamp(21px, ' + p + '%, calc(100% - 21px))';
  $('frase').textContent = e.frase;
  const r = F.textoRestante(e.progresso ? e.progresso.restante : -1);
  $('restante').textContent = r;
  $('restante').hidden = !r;
}

function campoChave(){
  return '<label class="rotulo-campo" for="chaveCampo">A sua chave</label>'
    + '<input type="text" class="campo" id="chaveCampo" data-foco="chaveCampo" spellcheck="false" autocomplete="off" placeholder="CD2-...">'
    + '<p class="aviso-erro" id="erroChave" role="alert" hidden></p>';
}

// A chave guardada (MinhaChave), com a linha de guardar. Sem ela, a frase de que está guardada.
// No reinstalar, o "Copiar a chave" vai embaixo da chave, porque a linha de botões já está cheia.
function blocoChave(chave, comCopiar){
  if(!chave) return '<p class="texto">A sua chave está guardada neste computador e no seu e-mail.</p>';
  return '<p class="rotulo-chave"><b>A sua chave</b> (também foi para o seu e-mail):</p>'
    + '<textarea class="chave-mostra" rows="2" readonly aria-label="A sua chave">' + esc(chave) + '</textarea>'
    + '<p class="guarde">Guarde. Ela reinstala neste computador quando precisar.'
    + (comCopiar ? ' <button type="button" class="link" data-acao="copiar">Copiar a chave</button>' : '') + '</p>';
}

function mostrarErroChave(e){
  $('erroChave').textContent = e.erro;
  $('erroChave').hidden = !e.erro;
}

function discoEscolhido(e){
  return e.discos.find(d => d.id === e.escolhido) || null;
}

function listaDeDiscos(e){
  if(!e.discos.length){
    return '<div class="procurando espera" role="status"><span class="boneco">' + M.mascoteSvg() + '</span>' + esc(F.textoProcurando(e)) + '</div>';
  }
  let h = '<div class="lista" role="radiogroup" aria-label="Pen drives encontrados">';
  for(const d of e.discos){
    const bloqueado = !F.escolhivel(d);
    const nota = F.textoSituacao(d);
    const marcado = d.id === e.escolhido;
    h += '<button type="button" class="disco" role="radio" aria-checked="' + marcado + '"'
      + (bloqueado ? ' disabled' : '') + ' data-acao="escolher" data-id="' + esc(d.id) + '" data-foco="disco:' + esc(d.id) + '">'
      + '<span class="marca" aria-hidden="true"></span>' + ICONE_PENDRIVE
      + '<span class="nome"><b>' + esc(d.rotulo) + '</b>'
      + (nota ? '<small>' + esc(nota) + '</small>' : '') + '</span></button>';
    if(marcado && d.situacao === 'aviso') h += '<p class="nota">' + esc(F.TEXTOS.aviso) + '</p>';
  }
  return h + '</div>';
}

// Troca o conteúdo só quando mudou e devolve o foco ao mesmo controle, para a busca a cada 1,5 s
// não roubar o foco de quem navega pelo teclado.
function trocar(el, html){
  if(el.dataset.html === html) return;
  const foco = document.activeElement && el.contains(document.activeElement) ? document.activeElement.dataset.foco : null;
  el.innerHTML = html;
  el.dataset.html = html;
  if(foco){
    const alvo = el.querySelector('[data-foco="' + CSS.escape(foco) + '"]');
    if(alvo && !alvo.disabled) alvo.focus();
  }
}

// ---------- ciclo ----------

let estado = F.inicial();
let desenhada = null;
let backend = null;

function desenhar(){
  const t = estado.tela, tela = telas[t];
  const n = F.etapa(t);
  $('barraEtapas').hidden = !n;
  $('etapa').textContent = n + ' de 4';
  Array.from($('etapas').children).forEach((li, i) => li.classList.toggle('on', i < n));
  $('teste').hidden = !(estado.info && estado.info.sandbox) || F.TELAS_PENDRIVE.includes(t) || t === 'escolha';
  // As duas etiquetas juntas não cabem no topo: a da demonstração encurta.
  $('demo').textContent = $('teste').hidden ? 'Demonstração: nada é gravado' : 'Demonstração';
  $('tela').classList.toggle('com-rolagem', t === 'condicoes');
  if(desenhada !== t){
    $('tela').innerHTML = tela.montar(estado);
    desenhada = t;
    tela.atualizar(estado);
    const h1 = $('tela').querySelector('h1');
    if(h1) h1.focus({ preventScroll: true });
    if(t === 'plugue') procurar();
  } else {
    tela.atualizar(estado);
  }
}

function despachar(acao){
  const antes = estado;
  estado = F.passo(estado, acao);
  if(estado === antes) return;
  if(antes.tela === 'confirmar' && estado.tela === 'gravando') comecarGravacao();
  if(antes.tela === 'perguntas' && estado.tela === 'instalando') comecarInstalacao();
  if(antes.tela === 'condicoes' && estado.tela === 'aguardando') abrirPagamento();
  if(antes.tela !== estado.tela && (estado.tela === 'reinstalar' || estado.tela === 'instalado')) buscarMinhaChave();
  desenhar();
}

// A chave mostrada vem sempre do Go, já normalizada e conferida para este computador.
function buscarMinhaChave(){
  Promise.resolve(backend.minhaChave()).then(k => despachar({ tipo: 'minhaChave', chave: k || '' })).catch(() => {});
}

const fraseDoErro = err => String((err && err.message) || err || 'Não deu certo. Tente de novo.');

// Escolher instalar: no Windows como administrador, o Go reabre o instalador sem administrador e
// esta janela fecha. Senão, devolve o bloqueio ou segue para a conta deste computador.
async function escolherInstalar(){
  despachar({ tipo: 'escolherInstalar' });
  if(estado.tela !== 'preparando') return;
  try {
    const bloqueio = await backend.escolherInstalar();
    if(bloqueio) return despachar({ tipo: 'bloqueio', frase: bloqueio });
    despachar({ tipo: 'info', info: await backend.info() });
  } catch(err){
    despachar({ tipo: 'bloqueio', frase: fraseDoErro(err) });
  }
}

function abrirPagamento(){
  Promise.resolve(backend.pagar()).catch(err => despachar({ tipo: 'falhaPagar', frase: fraseDoErro(err) }));
}

async function colarChave(botao){
  const k = $('chaveCampo').value.trim();
  if(!k){ $('chaveCampo').focus(); return; }
  botao.disabled = true;
  try {
    const erro = await backend.colar(k);
    despachar({ tipo: 'chave', ok: !erro, erro: erro || '' });
  } catch(err){
    despachar({ tipo: 'chave', ok: false, erro: fraseDoErro(err) });
  } finally {
    if(document.body.contains(botao)) botao.disabled = false;
  }
}

function comecarInstalacao(){
  Promise.resolve(backend.instalar(estado.area, estado.inicio)).catch(err => {
    despachar({ tipo: 'fim', ok: false, frase: fraseDoErro(err) });
  });
}

function abrirIA(botao){
  botao.disabled = true;
  Promise.resolve(backend.abrirIA()).catch(err => {
    botao.disabled = false;
    despachar({ tipo: 'falhaAbrir', frase: fraseDoErro(err) });
  });
}

function copiarChave(botao){
  Promise.resolve(backend.copiar(estado.chave)).then(() => {
    botao.textContent = 'Copiada';
    setTimeout(() => { if(document.body.contains(botao)) botao.textContent = 'Copiar a chave'; }, 2000);
  }).catch(() => {
    // Sem área de transferência: seleciona a chave para a pessoa copiar na mão.
    const t = document.querySelector('.chave-mostra');
    if(t){ t.focus(); t.select(); }
  });
}

function comecarGravacao(){
  const d = discoEscolhido(estado);
  Promise.resolve(backend.gravar(d)).catch(err => {
    despachar({ tipo: 'fim', ok: false, frase: fraseDoErro(err) });
  });
}

// Busca os discos a cada 1,5 s, só na tela plugue e sem duas buscas ao mesmo tempo. Erro na busca
// não trava nada: a tela segue em "Procurando pen drive..." e tenta de novo; depois de cinco erros
// seguidos, avisa que não consegue ver os discos (e continua tentando).
let buscando = false, proxima = null;
async function procurar(){
  clearTimeout(proxima);
  if(buscando || estado.tela !== 'plugue') return;
  buscando = true;
  try {
    const lista = await backend.listar();
    despachar({ tipo: 'discos', lista: Array.isArray(lista) ? lista : [] });
  } catch(_){
    despachar({ tipo: 'falhaListar' });
  } finally {
    buscando = false;
    if(estado.tela === 'plugue') proxima = setTimeout(procurar, 1500);
  }
}

// ---------- eventos da página ----------

// Mascote do topo: uma piada por clique, que some sozinha depois de uns segundos.
let ultimaPiada = -1, tempoBalao = null;
function contarPiada(){
  ultimaPiada = M.piada(ultimaPiada);
  const b = $('balao');
  b.textContent = M.PIADAS[ultimaPiada];
  b.hidden = false;
  clearTimeout(tempoBalao);
  tempoBalao = setTimeout(() => { b.hidden = true; }, 4500);
}

document.addEventListener('click', ev => {
  if(ev.target.closest('#mascote')) return contarPiada();
  const b = ev.target.closest('[data-acao]');
  if(!b || b.disabled) return;
  const acao = b.dataset.acao;
  if(acao === 'escolher') despachar({ tipo: 'escolher', id: b.dataset.id });
  else if(acao === 'cancelar'){ b.disabled = true; despachar({ tipo: 'cancelar' }); backend.cancelar(); }
  else if(acao === 'escolherInstalar') escolherInstalar();
  else if(acao === 'abrirPagamento') abrirPagamento();
  else if(acao === 'colarOk') colarChave(b);
  else if(acao === 'abrirIA') abrirIA(b);
  else if(acao === 'copiar') copiarChave(b);
  else despachar({ tipo: acao });
});

document.addEventListener('change', ev => {
  const id = ev.target.id;
  if(id === 'podeApagar') despachar({ tipo: 'marcar', valor: ev.target.checked });
  else if(id === 'entendi') despachar({ tipo: 'entender', valor: ev.target.checked });
  else if(id === 'area' || id === 'inicio') despachar({ tipo: 'opcao', nome: id, valor: ev.target.checked });
});

// Enter no campo da chave é o mesmo que Continuar.
document.addEventListener('keydown', ev => {
  if(ev.key !== 'Enter' || ev.target.id !== 'chaveCampo') return;
  ev.preventDefault();
  const b = document.querySelector('[data-acao="colarOk"]');
  if(b && !b.disabled) colarChave(b);
});

// Setas movem a escolha entre os discos, como num grupo de rádio.
document.addEventListener('keydown', ev => {
  const atual = ev.target.closest && ev.target.closest('.disco');
  if(!atual || !['ArrowDown', 'ArrowUp', 'ArrowLeft', 'ArrowRight'].includes(ev.key)) return;
  const todos = Array.from(document.querySelectorAll('.disco:not(:disabled)'));
  const i = todos.indexOf(atual);
  if(i < 0 || !todos.length) return;
  ev.preventDefault();
  const passoSeta = ev.key === 'ArrowDown' || ev.key === 'ArrowRight' ? 1 : -1;
  const alvo = todos[(i + passoSeta + todos.length) % todos.length];
  despachar({ tipo: 'escolher', id: alvo.dataset.id });
  const novo = document.querySelector('.disco[data-id="' + CSS.escape(alvo.dataset.id) + '"]');
  if(novo) novo.focus();
});

// ---------- início ----------

function noWails(){
  return !!window.runtime || location.protocol === 'wails:' || location.hostname === 'wails.localhost';
}

async function iniciar(){
  $('mascote').innerHTML = M.mascoteSvg();
  if(window.go && window.go.main && window.go.main.App) backend = backendWails();
  else { backend = backendDemo(); $('demo').hidden = false; }
  backend.ouvir('progresso', ev => despachar({ tipo: 'progresso', evento: ev }));
  backend.ouvir('fim', f => despachar({ tipo: 'fim', ok: !!(f && f.ok), frase: (f && f.frase) || '' }));
  backend.ouvir('chave', c => despachar({ tipo: 'chave', ok: !!(c && c.ok), erro: (c && c.erro) || '' }));
  // Reaberta com --instalar (Windows, sem administrador): vai direto para instalar e não formata.
  let reaberta = false;
  try { reaberta = !!(await backend.comInstalar()); } catch(_){ reaberta = false; }
  estado = F.inicial({ semAdmin: reaberta });
  desenhar();
  if(reaberta) escolherInstalar();
}

// Dentro do Wails, a ponte pode chegar um instante depois da página: espera por ela e nunca cai
// na demonstração.
function esperarPonte(){
  if(!noWails() || (window.go && window.go.main && window.go.main.App)) return iniciar();
  setTimeout(esperarPonte, 50);
}

if(document.readyState === 'loading') document.addEventListener('DOMContentLoaded', esperarPonte);
else esperarPonte();
})();
