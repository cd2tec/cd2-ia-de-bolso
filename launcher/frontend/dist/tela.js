/* Desenha a janelinha do lançador a partir do estado que o Go manda (evento "estado") e liga os botões
   aos métodos do Wails (window.go.main.App). Aberta direto no navegador, sem Wails, mostra um estado de
   exemplo e não quebra. render() é pura: devolve o que mostrar, e o DOM só aplica. */
(function(root){
"use strict";
const nodo = typeof module !== 'undefined' && module.exports;
const Mascote = nodo ? require('./mascote.js') : root.CD2Mascote;
const Cena = nodo ? require('./cena.js') : root.CD2Cena;
const Piadas = nodo ? require('./piadas.js') : root.CD2Piadas;

const B_NOVA = 'Abrir uma nova janela da conversa';
const B_DORMIR = 'Colocar a IA para dormir';
const B_REGISTROS = 'Abrir a pasta dos registros';
const B_FECHAR = 'Fechar';
const B_INSTALAR = 'Instalar neste computador';
const B_JA_TENHO = 'Já tenho a chave';
const B_LIBERAR = 'Liberar';
const B_VOLTAR = 'Voltar';
// Texto do botão -> método do Go (Task 9). Instalar, Já tenho e Voltar só trocam o modo da tela.
const ACAO = { [B_NOVA]: 'AbrirConversa', [B_DORMIR]: 'Dormir', [B_REGISTROS]: 'AbrirRegistros', [B_FECHAR]: 'Fechar',
  [B_LIBERAR]: 'ColarChave' };

// O convite para instalar, embaixo dos botões de Pronta, só quando a IA roda do pen drive. Clicar abre a conversa
// já com o modal de instalação (AbrirInstalar).
const CONVITE = { pergunta: 'Quer me usar sem o pen drive?', acao: 'Instale neste computador' };

// Cópia instalada sem chave (a fase Trancada vem do Go, a IA nem começou a subir).
const T_TRANCADA = 'Esta cópia não foi instalada neste computador';
const L_TRANCADA = 'Para me usar daqui, instale pelo pen drive. Se você já pagou por este computador, cole a sua chave.';
const L_INSTALAR = 'Espete o pen drive da IA de Bolso e abra o lançador de lá. A instalação é feita por ele.';
const T_COLAR = 'Cole a sua chave aqui:';
const CENA_TRISTE = { rosto: 'sono', nivel: 0 };

// estado: {fase, pct, aviso, erro, pendrive}; modo: 'normal' ou 'chamada' (e, na Trancada, 'instalar' ou 'colar');
// msg: a resposta do ColarChave, mostrada no modo 'colar'.
function render(estado, modo, msg){
  const e = estado || {};
  switch(e.fase){
    case 'Lendo':
    case 'Subindo':
      return { titulo: '', linha: e.aviso || 'carregando o modelo de IA ... ' + (e.pct || 0) + '%',
        piadaLista: Mascote.PIADAS, botoes: [] };
    case 'Pronta': {
      // Com o convite a piada sai: a janelinha (380 px) não tem altura para os dois junto do aviso.
      const convite = e.pendrive ? CONVITE : null;
      return { titulo: modo === 'chamada' ? 'Me chamou?' : 'Acordei! A conversa abriu no navegador.', linha: e.aviso || '',
        piadaLista: convite ? null : (modo === 'chamada' ? Piadas.CHAMADA : Mascote.PIADAS), botoes: [B_NOVA, B_DORMIR],
        convite };
    }
    case 'Dormindo':
      return { titulo: 'Colocando a IA para dormir...', linha: '', piadaLista: null, botoes: [] };
    case 'Dormiu':
      // Com erro, algo não parou: o aviso de reiniciar no lugar do "pode tirar", e sem piada.
      if(e.erro) return { titulo: e.erro, linha: '', piadaLista: null, botoes: [] };
      return { titulo: 'Boa noite! Pode tirar o pen drive.', linha: '', piadaLista: Piadas.DESPEDIDA, botoes: [] };
    case 'Erro':
      return { titulo: e.erro || 'Algo deu errado.', linha: '', piadaLista: null, botoes: [B_REGISTROS, B_FECHAR] };
    case 'Trancada':
      if(modo === 'colar') return { titulo: T_COLAR, linha: msg || '', piadaLista: null, botoes: [B_LIBERAR, B_VOLTAR],
        campo: true, cena: CENA_TRISTE };
      return { titulo: T_TRANCADA, linha: modo === 'instalar' ? L_INSTALAR : L_TRANCADA, piadaLista: null,
        botoes: [B_INSTALAR, B_JA_TENHO], cena: CENA_TRISTE };
    default:
      return { titulo: '', linha: '', piadaLista: null, botoes: [] };
  }
}

// Trava dos botões: o primeiro clique vale e o resto espera o próximo estado (clique duplo não dispara
// Dormir/AbrirConversa duas vezes). AbrirConversa e AbrirRegistros não mudam a fase, então destravam sozinhas depois de um tempo.
function criarTrava(aoMudar, esperar, ms){
  let travado = false, timer = null;
  const muda = v => { travado = v; if(aoMudar) aoMudar(v); };
  const limpa = () => { if(timer !== null){ (clearTimeout)(timer); timer = null; } };
  return {
    get travado(){ return travado; },
    clicar(acao){
      if(travado) return false;
      muda(true);
      if(acao !== 'Dormir' && acao !== 'Fechar') timer = (esperar || setTimeout)(() => { timer = null; muda(false); }, ms || 1500);
      return true;
    },
    liberar(){ limpa(); muda(false); }
  };
}

// O estado pode chegar como objeto ou como texto JSON.
function lerEstado(v){
  if(typeof v === 'string'){ try { return JSON.parse(v); } catch(_){ return null; } }
  return v && typeof v === 'object' ? v : null;
}

const api = { render, criarTrava, lerEstado };
if(nodo){ module.exports = api; return; }
root.CD2Tela = api;
if(typeof document === 'undefined') return;

// ---------- DOM ----------
const $ = id => document.getElementById(id);
let estado = { fase: 'Lendo', pct: 0 }, modo = 'normal';
let passo = 0, lista = null, ultimaPiada = -1, relogioPiada = null, chegouEvento = false, msgColar = '';
const trava = criarTrava(v => document.querySelectorAll('#botoes .btn, #convite').forEach(b => { b.disabled = v; }));

function ponte(){ return root.go && root.go.main && root.go.main.App; }

function sortearPiada(){
  if(!lista){ $('piada').hidden = true; clearInterval(relogioPiada); return; }
  ultimaPiada = Piadas.proxima(lista, ultimaPiada);
  $('piada').textContent = lista[ultimaPiada];
  $('piada').hidden = false;
  // Piada nova (por troca de fase, chamada ou pelo relógio) recomeça os 6 s.
  clearInterval(relogioPiada);
  relogioPiada = setInterval(sortearPiada, 6000);
}

function cena(r){
  const f = estado.fase, el = $('cena');
  if(f === 'Lendo' || f === 'Subindo'){ el.className = 'cena larga'; el.innerHTML = Cena.svgCarregando(estado.pct || 0, passo); }
  else if(r && r.cena){ el.className = 'cena'; el.innerHTML = Cena.svgCerebro(r.cena.rosto, r.cena.nivel); }
  else { el.className = 'cena'; el.innerHTML = Cena.svgCerebro(f === 'Pronta' ? 'acordado' : 'sono', f === 'Erro' ? 0 : 6); }
}

// Botões da tela trancada que só trocam o modo, sem chamar o Go.
const LOCAL = {
  [B_INSTALAR]: () => { modo = 'instalar'; },
  [B_JA_TENHO]: () => { modo = 'colar'; msgColar = ''; },
  [B_VOLTAR]: () => { modo = 'normal'; msgColar = ''; }
};

// Liberar: o Go confere e grava a chave. Aceita, o próximo estado (Lendo) troca a tela; recusada, o motivo vai para a linha.
function liberar(){
  const A = ponte();
  if(!trava.clicar('ColarChave') || !A || !A.ColarChave) return;
  Promise.resolve(A.ColarChave($('campo').value)).then(txt => {
    if(!txt) return;
    msgColar = txt; trava.liberar(); desenhar();
  }).catch(() => trava.liberar());
}

function desenhar(){
  const r = render(estado, modo, msgColar), carregando = estado.fase === 'Lendo' || estado.fase === 'Subindo';
  cena(r);
  $('titulo').textContent = r.titulo;
  $('titulo').hidden = !r.titulo;
  $('trilho').hidden = !carregando;
  $('preenche').style.width = Math.max(0, Math.min(100, estado.pct || 0)) + '%';
  $('linha').textContent = r.linha;
  $('linha').hidden = !r.linha;
  $('linha').className = 'linha' + (carregando ? '' : ' centro');
  const campo = $('campo'), mostrarCampo = !!r.campo, apareceu = mostrarCampo && campo.hidden;
  campo.hidden = !mostrarCampo;
  if(apareceu){ campo.value = ''; campo.focus(); }
  if(r.piadaLista !== lista){ lista = r.piadaLista; ultimaPiada = -1; sortearPiada(); }
  const conv = $('convite');
  conv.hidden = !r.convite;
  if(r.convite){
    $('convitePergunta').textContent = r.convite.pergunta;
    $('conviteAcao').textContent = r.convite.acao;
    conv.disabled = trava.travado;
  }
  const caixa = $('botoes');
  caixa.textContent = '';
  r.botoes.forEach((rotulo, i) => {
    const b = document.createElement('button');
    b.type = 'button';
    b.className = 'btn ' + (i === 0 ? 'primario' : 'secundario');
    b.textContent = rotulo;
    b.addEventListener('click', () => {
      if(LOCAL[rotulo]){ LOCAL[rotulo](); desenhar(); return; }
      if(rotulo === B_LIBERAR) return liberar();
      const A = ponte(), nome = ACAO[rotulo];
      if(trava.clicar(nome) && A && A[nome]) A[nome]();
    });
    b.disabled = trava.travado;
    caixa.appendChild(b);
  });
}

// O modo "chamada" vale até o próximo estado que mude de fase.
function aoEstado(bruto){
  const novo = lerEstado(bruto);
  if(!novo) return;
  chegouEvento = true;
  // Só uma troca de fase libera os botões: um estado repetido (o pct andando) não destrava.
  if(novo.fase !== estado.fase){ modo = 'normal'; msgColar = ''; trava.liberar(); }
  estado = novo;
  desenhar();
}
function aoChamada(){ modo = 'chamada'; lista = undefined; trava.liberar(); desenhar(); }

function noWails(){
  return !!root.runtime || location.protocol === 'wails:' || location.hostname === 'wails.localhost';
}

function abrirInstalar(){
  const A = ponte();
  if(trava.clicar('AbrirInstalar') && A && A.AbrirInstalar) A.AbrirInstalar();
}

function iniciar(){
  $('convite').addEventListener('click', abrirInstalar);
  $('campo').addEventListener('keydown', ev => { if(ev.key === 'Enter'){ ev.preventDefault(); liberar(); } });
  const A = ponte();
  if(A && root.runtime){
    root.runtime.EventsOn('estado', aoEstado);
    root.runtime.EventsOn('chamada', aoChamada);
    // O estado de agora, antes de esperar eventos (um evento que já chegou é mais novo e vale mais).
    try {
      Promise.resolve(A.Estado()).then(v => { if(!chegouEvento) aoEstado(v); }).catch(() => {});
    } catch(_){}
  } else {
    // Sem Wails: um estado de exemplo (?fase=Pronta&pendrive=1, Erro, Dormiu, Trancada&modo=colar...), sem tocar em nada.
    $('demo').hidden = false;
    const q = new URLSearchParams(location.search);
    estado = { fase: q.get('fase') || 'Lendo', pct: Number(q.get('pct') || 62), erro: q.get('erro') || 'Não consegui acordar a IA. Os registros têm o motivo.',
      aviso: q.get('aviso') || '', pendrive: q.get('pendrive') === '1' };
    if(q.get('modo')) modo = q.get('modo');
  }
  desenhar();
  setInterval(() => { passo++; if(estado.fase === 'Lendo' || estado.fase === 'Subindo') cena(); }, 300);
}

// Dentro do Wails a ponte pode chegar um instante depois da página: espera por ela e nunca cai no exemplo.
function esperarPonte(){
  if(!noWails() || (ponte() && root.runtime)) return iniciar();
  setTimeout(esperarPonte, 50);
}

if(document.readyState === 'loading') document.addEventListener('DOMContentLoaded', esperarPonte);
else esperarPonte();
})(typeof window !== 'undefined' ? window : globalThis);
