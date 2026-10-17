/* Fluxo das telas do instalador: plugue, confirmar, gravando, pronto e erro. É lógica pura, sem DOM:
   passo(estado, acao) devolve um estado novo e nunca altera o recebido. Ação que não cabe na tela
   atual devolve o mesmo estado. Os textos que dependem de dado (arquivos, tempo, porcentagem) também
   moram aqui, para o teste conferir. */
(function(root){
"use strict";

const TEXTOS = {
  procurando: 'Procurando pen drive...',
  semDiscos: 'Não consegui ver os discos. Tire o pen drive e plugue de novo.',
  pequeno: 'Pequeno demais: precisa de pelo menos 4 GB.',
  proprio: 'O instalador está rodando deste pen drive. Copie o instalador para o computador e abra de lá.',
  aviso: 'Funciona, mas um de 16 GB ou mais é melhor.',
  pronto: 'Pronto, pode tirar o pen drive.',
};

// Depois de tantas falhas seguidas de ListarDiscos, a tela avisa (e continua tentando).
const FALHAS_PARA_AVISAR = 5;

function inicial(){
  return { tela: 'plugue', discos: [], escolhido: null, podeApagar: false, progresso: null, frase: '', falhas: 0 };
}

// Só ok e aviso podem ser escolhidos; situação desconhecida também não.
const escolhivel = d => !!d && (d.situacao === 'ok' || d.situacao === 'aviso');
const achar = (estado, id) => estado.discos.find(d => d.id === id) || null;

function podeContinuar(estado){
  return estado.tela === 'plugue' && escolhivel(achar(estado, estado.escolhido));
}

function passo(estado, acao){
  const t = estado.tela;
  switch(acao && acao.tipo){
    case 'discos': {
      if(t !== 'plugue') return estado;
      const lista = Array.isArray(acao.lista) ? acao.lista.slice() : [];
      let escolhido = estado.escolhido;
      const atual = lista.find(d => d.id === escolhido);
      if(!escolhivel(atual)) escolhido = null;
      // Com um disco só, ele já vem escolhido. Com dois ou mais, a pessoa escolhe.
      if(lista.length === 1 && escolhivel(lista[0])) escolhido = lista[0].id;
      return { ...estado, discos: lista, escolhido, falhas: 0 };
    }
    case 'falhaListar': {
      if(t !== 'plugue') return estado;
      const falhas = (estado.falhas || 0) + 1;
      // Com a busca falhando, a lista velha pode não valer mais.
      if(falhas >= FALHAS_PARA_AVISAR) return { ...estado, falhas, discos: [], escolhido: null };
      return { ...estado, falhas };
    }
    case 'escolher': {
      if(t !== 'plugue' || !escolhivel(achar(estado, acao.id))) return estado;
      return { ...estado, escolhido: acao.id };
    }
    case 'continuar':
      if(!podeContinuar(estado)) return estado;
      return { ...estado, tela: 'confirmar', podeApagar: false };
    case 'marcar':
      if(t !== 'confirmar') return estado;
      return { ...estado, podeApagar: !!acao.valor };
    case 'voltar':
      if(t !== 'confirmar') return estado;
      return { ...estado, tela: 'plugue', podeApagar: false };
    case 'gravar':
      if(t !== 'confirmar' || !estado.podeApagar) return estado;
      return { ...estado, tela: 'gravando', progresso: null, frase: '' };
    case 'progresso':
      if(t !== 'gravando' || !acao.evento) return estado;
      return { ...estado, progresso: { ...acao.evento }, frase: acao.evento.frase || '' };
    case 'fim':
      if(t !== 'gravando') return estado;
      return { ...estado, tela: acao.ok ? 'pronto' : 'erro', frase: acao.frase || '' };
    case 'outro':
      if(t !== 'pronto' && t !== 'erro') return estado;
      return inicial();
    default:
      return estado;
  }
}

// etapa da barra "n de 4"; o erro aparece onde a gravação parou
function etapa(tela){
  return { plugue: 1, confirmar: 2, gravando: 3, erro: 3, pronto: 4 }[tela] || 1;
}

function escapar(s){
  return String(s == null ? '' : s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}

// 2345 -> "2.345"
function milhar(n){
  return String(Math.max(0, Math.floor(n))).replace(/\B(?=(\d{3})+(?!\d))/g, '.');
}

// O que a lista mostra quando não há disco: procurando, ou o aviso depois de muitas falhas.
function textoProcurando(estado){
  return (estado.falhas || 0) >= FALHAS_PARA_AVISAR ? TEXTOS.semDiscos : TEXTOS.procurando;
}

// Nota embaixo do nome do disco que não pode ser escolhido.
function textoSituacao(d){
  if(d && d.situacao === 'pequeno') return TEXTOS.pequeno;
  if(d && d.situacao === 'proprio') return TEXTOS.proprio;
  return '';
}

// Frase da confirmação, em HTML: o rótulo vem escapado dentro do <b>.
function textoArquivos(d){
  // Sem acesso à raiz do pen drive, zero arquivos não quer dizer vazio.
  if(d.semAcesso) return 'Não consegui ver os arquivos do pen drive <b>' + escapar(d.rotulo) + '</b>. Tudo que estiver nele será apagado.';
  const nome = 'O pen drive <b>' + escapar(d.rotulo) + '</b> ';
  if(d.maisDe) return nome + 'tem mais de 10.000 arquivos. Eles vão sumir.';
  const n = d.arquivos || 0;
  if(n === 0) return nome + 'está vazio.';
  if(n === 1) return nome + 'tem 1 arquivo. Ele vai sumir.';
  return nome + 'tem ' + milhar(n) + ' arquivos. Eles vão sumir.';
}

// Segundos estimados para o fim; -1 (ou ausente) ainda não dá para estimar e não mostra nada.
function textoRestante(seg){
  if(typeof seg !== 'number' || !isFinite(seg) || seg < 0) return '';
  if(seg < 60) return 'Falta menos de um minuto.';
  const m = Math.round(seg / 60);
  if(m <= 1) return 'Falta cerca de um minuto.';
  return 'Faltam uns ' + m + ' minutos.';
}

function porcentagem(progresso){
  const f = progresso && typeof progresso.fracao === 'number' ? progresso.fracao : 0;
  return Math.max(0, Math.min(100, Math.floor(f * 100)));
}

function tituloGravando(progresso){
  return 'Gravando... ' + porcentagem(progresso) + '%';
}

// "Pronto." vira o título padrão; outra frase (a de ejetar pelo sistema) aparece como veio.
function tituloPronto(frase){
  return frase && frase !== 'Pronto.' ? frase : TEXTOS.pronto;
}

const api = { inicial, passo, podeContinuar, escolhivel, etapa, escapar, textoProcurando, textoSituacao, textoArquivos, textoRestante, porcentagem, tituloGravando, tituloPronto, TEXTOS };
if(typeof module !== 'undefined' && module.exports) module.exports = api; else root.CD2Fluxo = api;
})(typeof window !== 'undefined' ? window : globalThis);
