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
  };
}

// Demonstração: dois discos (um bom e um pequeno) e uma gravação simulada de uns 10 segundos.
// Variações pelo endereço: ?demo=um, ?demo=aviso, ?demo=nenhum, ?demo=muitos, ?demo=erro, ?demo=falha.
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
  let relogio = null, fracao = 0;
  const gb = x => x.toFixed(1).replace('.', ',');
  function fim(ok, frase){ clearInterval(relogio); relogio = null; emitir('fim', { ok, frase }); }
  return {
    listar: () => new Promise((r, rej) => setTimeout(() => modo === 'falha' ? rej('sem discos') : r(lista.map(d => ({ ...d }))), 300)),
    gravar: () => {
      if(relogio) return Promise.reject('Já tem uma gravação em andamento.');
      fracao = 0;
      const inicio = Date.now();
      emitir('progresso', { etapa: 'baixando', frase: 'Baixando o modelo (0,0 de 3,9 GB)', fracao: 0, restante: -1 });
      relogio = setInterval(() => {
        fracao = Math.min(1, (Date.now() - inicio) / 10000);
        if(modo === 'erro' && fracao >= 0.3) return fim(false, 'Não consegui baixar o que falta. Confira a internet e tente de novo. Nada foi apagado.');
        const restante = Math.round((1 - fracao) * 600);
        if(fracao < 0.5) emitir('progresso', { etapa: 'baixando', frase: 'Baixando o modelo (' + gb(fracao * 2 * 3.9) + ' de 3,9 GB)', fracao, restante });
        else if(fracao < 0.85) emitir('progresso', { etapa: 'copiando', frase: 'Copiando para o pen drive', fracao, restante });
        else if(fracao < 1) emitir('progresso', { etapa: 'conferindo', frase: 'Conferindo o que foi gravado', fracao, restante: Math.round((1 - fracao) * 200) });
        else { emitir('progresso', { etapa: 'pronto', frase: 'Pronto.', fracao: 1, restante: 0 }); fim(true, 'Pronto.'); }
      }, 250);
      return Promise.resolve(null);
    },
    cancelar: () => {
      if(!relogio) return Promise.resolve();
      setTimeout(() => fim(false, fracao < 0.5
        ? 'Gravação cancelada. Nada foi apagado.'
        : 'Gravação cancelada. O pen drive ficou incompleto. Grave de novo antes de usar.'), 400);
      return Promise.resolve();
    },
    ouvir: (nome, fn) => { (ouvintes[nome] = ouvintes[nome] || []).push(fn); },
  };
}

// ---------- ícones (SVG, sem emoji) ----------

const ICONE_PENDRIVE = '<svg class="icone" viewBox="0 0 28 28" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round" aria-hidden="true">'
  + '<rect x="9" y="3" width="10" height="7" rx="1.5"/><path d="M12 6h1M15 6h1" stroke-linecap="round"/>'
  + '<rect x="6.5" y="10" width="15" height="15" rx="3"/></svg>';
const ICONE_OK = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M5 12.5l4.5 4.5L19 7.5"/></svg>';
const ICONE_ERRO = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.8" stroke-linecap="round" aria-hidden="true"><path d="M12 6.5v7"/><path d="M12 17.5v.01"/></svg>';

// ---------- telas ----------
// Cada tela tem montar (o esqueleto, uma vez ao entrar) e atualizar (só o que muda com o estado).

const telas = {
  plugue: {
    montar: () => '<h1 tabindex="-1">Plugue o pen drive</h1>'
      + '<p class="texto">Use um de 16 GB ou mais. Tudo que estiver nele será apagado.</p>'
      + '<div id="discos" class="lista-area"></div>'
      + '<div class="acoes"><button type="button" class="btn primario" data-acao="continuar" data-foco="continuar">Continuar</button></div>',
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
    atualizar: e => {
      const p = F.porcentagem(e.progresso);
      $('titulo').textContent = F.tituloGravando(e.progresso);
      $('barra').setAttribute('aria-valuenow', String(p));
      $('barra').firstElementChild.style.width = p + '%';
      // O robô anda junto com a ponta da barra, sem sair pelas bordas.
      $('robo').style.left = 'clamp(21px, ' + p + '%, calc(100% - 21px))';
      $('frase').textContent = e.frase;
      const r = F.textoRestante(e.progresso ? e.progresso.restante : -1);
      $('restante').textContent = r;
      $('restante').hidden = !r;
    },
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
  $('etapa').textContent = n + ' de 4';
  Array.from($('etapas').children).forEach((li, i) => li.classList.toggle('on', i < n));
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
  desenhar();
}

function comecarGravacao(){
  const d = discoEscolhido(estado);
  Promise.resolve(backend.gravar(d)).catch(err => {
    despachar({ tipo: 'fim', ok: false, frase: String((err && err.message) || err || 'Não deu certo. Tente de novo.') });
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
  else if(acao === 'cancelar'){ b.disabled = true; backend.cancelar(); }
  else despachar({ tipo: acao });
});

document.addEventListener('change', ev => {
  if(ev.target.id === 'podeApagar') despachar({ tipo: 'marcar', valor: ev.target.checked });
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

function iniciar(){
  $('mascote').innerHTML = M.mascoteSvg();
  if(window.go && window.go.main && window.go.main.App) backend = backendWails();
  else { backend = backendDemo(); $('demo').hidden = false; }
  backend.ouvir('progresso', ev => despachar({ tipo: 'progresso', evento: ev }));
  backend.ouvir('fim', f => despachar({ tipo: 'fim', ok: !!(f && f.ok), frase: (f && f.frase) || '' }));
  desenhar();
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
