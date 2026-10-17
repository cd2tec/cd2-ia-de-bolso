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
// Texto do botão -> método do Go (Task 9).
const ACAO = { [B_NOVA]: 'AbrirConversa', [B_DORMIR]: 'Dormir', [B_REGISTROS]: 'AbrirRegistros', [B_FECHAR]: 'Fechar' };

// estado: {fase, pct, aviso, erro}; modo: 'normal' ou 'chamada'.
function render(estado, modo){
  const e = estado || {};
  switch(e.fase){
    case 'Lendo':
    case 'Subindo':
      return { titulo: '', linha: e.aviso || 'carregando o modelo de IA ... ' + (e.pct || 0) + '%',
        piadaLista: Mascote.PIADAS, botoes: [] };
    case 'Pronta':
      return { titulo: modo === 'chamada' ? 'Me chamou?' : 'Acordei! A conversa abriu no navegador.', linha: e.aviso || '',
        piadaLista: modo === 'chamada' ? Piadas.CHAMADA : Mascote.PIADAS, botoes: [B_NOVA, B_DORMIR] };
    case 'Dormindo':
      return { titulo: 'Colocando a IA para dormir...', linha: '', piadaLista: null, botoes: [] };
    case 'Dormiu':
      return { titulo: 'Boa noite! Pode tirar o pen drive.', linha: '', piadaLista: Piadas.DESPEDIDA, botoes: [] };
    case 'Erro':
      return { titulo: e.erro || 'Algo deu errado.', linha: '', piadaLista: null, botoes: [B_REGISTROS, B_FECHAR] };
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
let passo = 0, lista = null, ultimaPiada = -1, relogioPiada = null, chegouEvento = false;
const trava = criarTrava(v => document.querySelectorAll('#botoes .btn').forEach(b => { b.disabled = v; }));

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

function cena(){
  const f = estado.fase, el = $('cena');
  if(f === 'Lendo' || f === 'Subindo'){ el.className = 'cena larga'; el.innerHTML = Cena.svgCarregando(estado.pct || 0, passo); }
  else { el.className = 'cena'; el.innerHTML = Cena.svgCerebro(f === 'Pronta' ? 'acordado' : 'sono', f === 'Erro' ? 0 : 6); }
}

function desenhar(){
  const r = render(estado, modo), carregando = estado.fase === 'Lendo' || estado.fase === 'Subindo';
  cena();
  $('titulo').textContent = r.titulo;
  $('titulo').hidden = !r.titulo;
  $('trilho').hidden = !carregando;
  $('preenche').style.width = Math.max(0, Math.min(100, estado.pct || 0)) + '%';
  $('linha').textContent = r.linha;
  $('linha').hidden = !r.linha;
  $('linha').className = 'linha' + (carregando ? '' : ' centro');
  if(r.piadaLista !== lista){ lista = r.piadaLista; ultimaPiada = -1; sortearPiada(); }
  const caixa = $('botoes');
  caixa.textContent = '';
  r.botoes.forEach((rotulo, i) => {
    const b = document.createElement('button');
    b.type = 'button';
    b.className = 'btn ' + (i === 0 ? 'primario' : 'secundario');
    b.textContent = rotulo;
    b.addEventListener('click', () => {
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
  if(novo.fase !== estado.fase){ modo = 'normal'; trava.liberar(); }
  estado = novo;
  desenhar();
}
function aoChamada(){ modo = 'chamada'; lista = undefined; trava.liberar(); desenhar(); }

function noWails(){
  return !!root.runtime || location.protocol === 'wails:' || location.hostname === 'wails.localhost';
}

function iniciar(){
  const A = ponte();
  if(A && root.runtime){
    root.runtime.EventsOn('estado', aoEstado);
    root.runtime.EventsOn('chamada', aoChamada);
    // O estado de agora, antes de esperar eventos (um evento que já chegou é mais novo e vale mais).
    try {
      Promise.resolve(A.Estado()).then(v => { if(!chegouEvento) aoEstado(v); }).catch(() => {});
    } catch(_){}
  } else {
    // Sem Wails: um estado de exemplo (?fase=Pronta, Erro, Dormiu...), sem tocar em nada.
    $('demo').hidden = false;
    const q = new URLSearchParams(location.search);
    estado = { fase: q.get('fase') || 'Lendo', pct: Number(q.get('pct') || 62), erro: q.get('erro') || 'Não consegui acordar a IA. Os registros têm o motivo.' };
    if(q.get('modo') === 'chamada') modo = 'chamada';
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
