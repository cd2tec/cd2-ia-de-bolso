/* Fluxo das telas do instalador. A primeira tela (escolha) abre dois caminhos que não se misturam:
   o do pen drive (plugue, confirmar, gravando, pronto e erro) e o de instalar neste computador
   (preparando, condicoes, aguardando, colar, reinstalar, perguntas, instalando, instalado,
   naoInstalou e bloqueado). É lógica pura, sem DOM:
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
  semAdmin: 'Esta janela não pode gravar pen drive. Para criar o pen drive, feche o instalador e abra de novo com clique duplo.',
};

const TELAS_PENDRIVE = ['plugue', 'confirmar', 'gravando', 'pronto', 'erro'];
// Telas em que a chave que volta do pagamento (ou a colada) é aceita.
const TELAS_CHAVE = ['condicoes', 'aguardando', 'colar'];
// Telas de onde "Agora não" ou "Voltar" levam de volta à escolha.
const TELAS_INICIO = ['condicoes', 'reinstalar', 'bloqueado'];

// Depois de tantas falhas seguidas de ListarDiscos, a tela avisa (e continua tentando).
const FALHAS_PARA_AVISAR = 5;

// semAdmin: a janela foi reaberta sem administrador (Windows, --instalar) e não pode formatar.
function inicial(opcoes){
  return {
    tela: 'escolha', semAdmin: !!(opcoes && opcoes.semAdmin), avisoPendrive: false,
    // pen drive
    discos: [], escolhido: null, podeApagar: false, progresso: null, frase: '', falhas: 0,
    // instalar neste computador
    info: null, entendi: false, erro: '', chave: '', area: true, inicio: true,
  };
}

// O pen drive do zero, mantendo o que a janela já sabe (semAdmin e a info de instalar).
function pendriveDoZero(estado){
  return { ...estado, tela: 'plugue', discos: [], escolhido: null, podeApagar: false, progresso: null, frase: '', falhas: 0, avisoPendrive: false };
}

// Depois da conta deste computador: bloqueado, reinstalar (já tem chave) ou condições.
function depoisDaInfo(estado, info){
  const i = { ...info };
  if(i.bloqueio) return { ...estado, info: i, tela: 'bloqueado', frase: i.bloqueio };
  return { ...estado, info: i, tela: i.temChave ? 'reinstalar' : 'condicoes', entendi: false, erro: '' };
}

const cabe = estado => !!(estado.info && estado.info.cabe);

function podePagar(estado){
  return estado.tela === 'condicoes' && estado.entendi && cabe(estado);
}

// Só ok e aviso podem ser escolhidos; situação desconhecida também não.
const escolhivel = d => !!d && (d.situacao === 'ok' || d.situacao === 'aviso');
const achar = (estado, id) => estado.discos.find(d => d.id === id) || null;

function podeContinuar(estado){
  return estado.tela === 'plugue' && !estado.semAdmin && escolhivel(achar(estado, estado.escolhido));
}

function passo(estado, acao){
  const t = estado.tela;
  switch(acao && acao.tipo){
    // ---------- escolha ----------
    case 'escolherPendrive':
      if(t !== 'escolha') return estado;
      // A janela sem administrador não formata: avisa e fica na escolha.
      if(estado.semAdmin) return { ...estado, avisoPendrive: true };
      return pendriveDoZero(estado);
    case 'escolherInstalar':
      if(t !== 'escolha') return estado;
      if(acao.info) return depoisDaInfo({ ...estado, avisoPendrive: false }, acao.info);
      return { ...estado, tela: 'preparando', avisoPendrive: false, erro: '' };
    case 'info':
      if(t !== 'preparando' || !acao.info) return estado;
      return depoisDaInfo(estado, acao.info);
    case 'bloqueio':
      if(t !== 'preparando') return estado;
      return { ...estado, tela: 'bloqueado', frase: acao.frase || '' };
    case 'inicio':
      if(!TELAS_INICIO.includes(t)) return estado;
      return { ...estado, tela: 'escolha', entendi: false, erro: '', frase: '' };

    // ---------- instalar neste computador ----------
    case 'entender':
      if(t !== 'condicoes') return estado;
      return { ...estado, entendi: !!acao.valor };
    case 'pagar':
      if(!podePagar(estado)) return estado;
      return { ...estado, tela: 'aguardando', erro: '' };
    case 'falhaPagar':
      if(t !== 'aguardando') return estado;
      return { ...estado, tela: 'condicoes', erro: acao.frase || '' };
    case 'jaTenho':
      if(t !== 'condicoes') return estado;
      return { ...estado, tela: 'colar', erro: '' };
    case 'chave':
      if(!TELAS_CHAVE.includes(t)) return estado;
      if(!acao.ok) return { ...estado, erro: acao.erro || '' };
      return { ...estado, tela: 'perguntas', erro: '', chave: acao.chave || estado.chave, area: true, inicio: true };
    case 'reinstalar':
      if(t !== 'reinstalar' || !cabe(estado)) return estado;
      return { ...estado, tela: 'perguntas', erro: '', area: true, inicio: true };
    case 'opcao':
      if(t !== 'perguntas' || (acao.nome !== 'area' && acao.nome !== 'inicio')) return estado;
      return { ...estado, [acao.nome]: !!acao.valor };
    case 'instalar':
      if(t !== 'perguntas') return estado;
      return { ...estado, tela: 'instalando', progresso: null, frase: '', erro: '' };
    case 'falhaAbrir':
      if(t !== 'instalado') return estado;
      return { ...estado, erro: acao.frase || '' };

    // ---------- pen drive ----------
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
      if(t === 'aguardando' || t === 'colar') return { ...estado, tela: 'condicoes', erro: '' };
      if(t !== 'confirmar') return estado;
      return { ...estado, tela: 'plugue', podeApagar: false };
    case 'gravar':
      if(t !== 'confirmar' || !estado.podeApagar || estado.semAdmin) return estado;
      return { ...estado, tela: 'gravando', progresso: null, frase: '' };

    // ---------- os dois caminhos ----------
    case 'progresso':
      if((t !== 'gravando' && t !== 'instalando') || !acao.evento) return estado;
      return { ...estado, progresso: { ...acao.evento }, frase: acao.evento.frase || '' };
    case 'fim':
      if(t === 'instalando') return { ...estado, tela: acao.ok ? 'instalado' : 'naoInstalou', frase: acao.frase || '', erro: '' };
      if(t !== 'gravando') return estado;
      return { ...estado, tela: acao.ok ? 'pronto' : 'erro', frase: acao.frase || '' };
    case 'outro':
      // "Gravar outro" e "Tentar de novo" ficam no caminho em que a pessoa está.
      if(t === 'naoInstalou') return { ...estado, tela: 'perguntas', frase: '', erro: '' };
      if(t !== 'pronto' && t !== 'erro') return estado;
      return pendriveDoZero(estado);
    default:
      return estado;
  }
}

// etapa da barra "n de 4", só no pen drive (0 nas outras telas, e a barra some); o erro aparece
// onde a gravação parou
function etapa(tela){
  return { plugue: 1, confirmar: 2, gravando: 3, erro: 3, pronto: 4 }[tela] || 0;
}

// 7.6e9 -> "7,6"
function gb(bytes){
  const n = typeof bytes === 'number' && isFinite(bytes) ? Math.max(0, bytes) : 0;
  return (n / 1e9).toFixed(1).replace('.', ',');
}

// Aviso de falta de espaço da tela de condições; vazio quando cabe.
function textoEspaco(info){
  if(!info || info.cabe) return '';
  return 'Falta espaço: a instalação precisa de uns ' + gb(info.precisa) + ' GB livres e este computador tem ' + gb(info.livre) + ' GB.';
}

function tituloInstalando(progresso){
  return 'Instalando... ' + porcentagem(progresso) + '%';
}

function textoInstalado(area){
  return 'A partir de agora, abra a IA de Bolso pelo ícone nos programas' + (area ? ' ou na área de trabalho' : '') + '.';
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

const api = { inicial, passo, podeContinuar, podePagar, escolhivel, etapa, escapar, gb, textoProcurando, textoSituacao, textoArquivos, textoRestante, textoEspaco, porcentagem, tituloGravando, tituloInstalando, tituloPronto, textoInstalado, TEXTOS, TELAS_PENDRIVE };
if(typeof module !== 'undefined' && module.exports) module.exports = api; else root.CD2Fluxo = api;
})(typeof window !== 'undefined' ? window : globalThis);
