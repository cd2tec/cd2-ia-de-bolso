/* Lógica do modal de instalação (sem DOM): decide a tela inicial, monta a URL de compra,
   normaliza a chave, entrega a chave entre abas e formata o progresso da cópia.
   A validade da assinatura da chave é conferida no servidor; aqui só a forma. */
(function(root){
"use strict";
const WORKER = 'https://instalariadebolso.cd2.com.br';
const ALFABETO = '0123456789ABCDEFGHJKMNPQRSTVWXYZ';

function urlComprar(codigo, porta){
  return WORKER + '/comprar?codigo=' + encodeURIComponent(codigo) + '&porta=' + encodeURIComponent(porta);
}

// Mesma regra do Normalizar em Go: tira espaços e hífens, maiúsculas, O->0, I e L->1,
// exige prefixo CD2 e 103 caracteres do alfabeto Crockford com os 3 bits finais zerados.
function normalizar(chave){
  if(typeof chave !== 'string') return null;
  const s = chave.replace(/[\s-]/g, '').toUpperCase().replace(/O/g, '0').replace(/[IL]/g, '1');
  if(s.slice(0, 3) !== 'CD2') return null;
  const corpo = s.slice(3);
  if(corpo.length !== 103) return null;
  for(const c of corpo) if(ALFABETO.indexOf(c) < 0) return null;
  // 64 bytes = 512 bits = 102*5 + 2: no último caractere só os 2 bits altos valem
  if((ALFABETO.indexOf(corpo[102]) & 0b111) !== 0) return null;
  return 'CD2-' + corpo;
}

function telaInicial(status){
  if(!status || status.origem !== 'pendrive') return null;
  return status.temChave ? 'reinstalar' : 'compra';
}

function lerParametros(search){
  const p = new URLSearchParams(String(search || '').replace(/^\?/, ''));
  return { abrir: p.get('instalar') === '1', chave: p.get('chave') || null };
}

// O endereço sem instalar e chave, para o history.replaceState: o resto (como o ?v=) fica.
function semParametros(search){
  const p = new URLSearchParams(String(search || '').replace(/^\?/, ''));
  p.delete('instalar'); p.delete('chave');
  const s = p.toString();
  return s ? '?' + s : '';
}

function entregarChave(opts){
  const { canal, chave } = opts;
  const esperarMs = opts.esperarMs == null ? 1500 : opts.esperarMs;
  const timers = opts.timers || { setTimeout: (f, ms) => setTimeout(f, ms), clearTimeout: id => clearTimeout(id) };
  return new Promise(resolve => {
    let id = null;
    const fim = r => {
      canal.removeEventListener('message', ouvir);
      if(id !== null) timers.clearTimeout(id);
      id = null;
      resolve(r);
    };
    const ouvir = ev => { if(ev && ev.data && ev.data.tipo === 'recebi') fim('entregue'); };
    canal.addEventListener('message', ouvir);
    id = timers.setTimeout(() => fim('sem-aba'), esperarMs);
    try{ canal.postMessage({ tipo: 'chave', chave }); }catch(e){ fim('sem-aba'); }
  });
}

function ouvirChave(opts){
  const { canal, aoReceber } = opts;
  const ouvir = ev => {
    if(!ev || !ev.data || ev.data.tipo !== 'chave') return;
    const chave = normalizar(ev.data.chave);
    if(chave === null) return; // lixo na rede: não confirma nem avisa
    canal.postMessage({ tipo: 'recebi' });
    aoReceber(chave);
  };
  canal.addEventListener('message', ouvir);
  return { parar(){ canal.removeEventListener('message', ouvir); } };
}

// Telas em que uma chave nova (colada, de outra aba ou do ?chave=) não muda nada: a chave já foi
// aceita e o modal não pode voltar para trás.
const TELAS_SEM_CHAVE = ['perguntas', 'instalando', 'pronto'];
function aceitaChave(tela){ return TELAS_SEM_CHAVE.indexOf(tela) < 0; }

// Só a IA que roda do pen drive escuta a chave vinda da outra aba. Fora dele, devolve null.
function ouvirNoPendrive(opts){
  if(telaInicial(opts.status) === null) return null;
  return ouvirChave(opts);
}

// Texto da tela Pronto. O motor ainda roda do pen drive até a instalada abrir.
function textoPronto(area){
  return 'A partir de agora, abra a IA de Bolso pelo ícone nos programas' + (area ? ' ou na área de trabalho' : '') +
    '. Depois de abrir a IA instalada, pode tirar o pen drive.';
}

function gb(bytes){ return (bytes / 1e9).toFixed(1).replace('.', ','); }
function textoProgresso(p){
  p = p || {};
  if(!p.total) return 'copiando ...';
  return 'copiando ... ' + gb(p.feitos || 0) + ' de ' + gb(p.total) + ' GB';
}

const api = { WORKER, urlComprar, normalizar, telaInicial, lerParametros, semParametros, entregarChave, ouvirChave, ouvirNoPendrive, aceitaChave, textoPronto, textoProgresso };
if(typeof module !== 'undefined' && module.exports) module.exports = api; else root.CD2Instalar = api;
})(typeof window !== 'undefined' ? window : globalThis);
