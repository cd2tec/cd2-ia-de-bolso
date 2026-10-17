/* Vigia do sono: quando o lançador do pen drive põe a IA para dormir, a janela da conversa continua
   aberta e o /health para de responder. Esta vigia pergunta de tempos em tempos e avisa quando o
   modelo some (aoDormir) e quando volta (aoAcordar). Uma falha só não conta: um /health lento
   com o modelo ocupado não é sono. Nada sai do computador: o /health é do próprio servidor local. */
(function(root){
"use strict";
function vigiar(opts){
  opts = opts || {};
  const doFetch = opts.fetch || (typeof fetch === 'function' ? fetch.bind(root) : null);
  const intervalo = opts.intervalo || 10000, limite = opts.falhas || 2;
  const timers = opts.timers || { setTimeout: (f, ms) => setTimeout(f, ms), clearTimeout: id => clearTimeout(id) };
  let falhou = 0, dormindo = false, parado = false, id = null;
  const agendar = () => { if(!parado) id = timers.setTimeout(passo, intervalo); };
  async function passo(){
    let ok = false;
    try{ const r = await doFetch('/health', { cache: 'no-store' }); ok = !!(r && r.ok); }catch(e){}
    if(parado) return; // parou enquanto o fetch estava no ar: não avisa nem reagenda
    if(ok){
      falhou = 0;
      if(dormindo){ dormindo = false; if(opts.aoAcordar) opts.aoAcordar(); }
    }else{
      falhou++;
      if(falhou >= limite && !dormindo){ dormindo = true; if(opts.aoDormir) opts.aoDormir(); }
    }
    agendar();
  }
  agendar();
  return { parar(){ parado = true; if(id !== null) timers.clearTimeout(id); id = null; } };
}

const api = { vigiar };
if(typeof module !== 'undefined' && module.exports) module.exports = api; else root.CD2Sono = api;
})(typeof window !== 'undefined' ? window : globalThis);
