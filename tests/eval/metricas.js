'use strict';
// Métricas da bateria da História 6 (CIDB-26): acerto na escolha do aprofundamento, resumos sem número
// inventado e latência por etapa. Funções puras sobre os itens que tests/eval/bateria.js grava; os testes
// (metricas.test.js) conferem as contas com casos de resposta conhecida (H6-CT04, CT05, CT06).
//
// Item de escolha:  { id, esperada, aceitaveis, tentativas:[{ ms, resposta, ok, motivo }], escolha, falha }
// Item de resumo:   { id, tentativas:[{ ms, resposta, ok, motivo }], linhas, falha, frasesTrocadas }
// falha: 'modelo' (sem resposta), 'formato', 'numero', 'fora' (escolha fora da lista) ou ausente.

// Prazos do cd2-ctl (PrazoEscolha em aprofundar.go, PrazoResumo em resumo.go), para todas as tentativas.
const PRAZO = { escolha:15000, resumo:25000 };

const soma = xs => xs.reduce((a, b) => a + b, 0);
const msTotal = it => soma((it.tentativas || []).map(t => t.ms || 0));
const pct = (a, b) => b ? Math.round(a / b * 1000) / 10 : null;

// Quantil pelo método do posto mais próximo: p90 de 30 valores é o 27º menor.
function quantil(xs, q){
  const s = xs.slice().sort((a, b) => a - b);
  return s.length ? s[Math.min(s.length - 1, Math.max(0, Math.ceil(q * s.length) - 1))] : 0;
}

function latencia(ms, prazo){
  if(!ms.length) return { n:0, media:0, p50:0, p90:0, pior:0, acimaDoPrazo:0, prazo };
  return { n:ms.length, media:Math.round(soma(ms) / ms.length), p50:quantil(ms, 0.5), p90:quantil(ms, 0.9),
    pior:Math.max.apply(null, ms), acimaDoPrazo:ms.filter(x => x > prazo).length, prazo };
}

// Escolha do aprofundamento. acerto: a escolha final está entre as aceitáveis; estrito: é a esperada.
// noPrazo: acertou e coube no prazo do cd2-ctl (o que a pessoa veria).
function escolha(itens){
  const xs = itens.filter(i => !i.controle);
  const respondeu = xs.filter(i => i.falha !== 'modelo');
  const acerto = xs.filter(i => !i.falha && (i.aceitaveis || [i.esperada]).includes(i.escolha));
  const estrito = xs.filter(i => !i.falha && i.escolha === i.esperada);
  const ms = respondeu.map(msTotal);
  const lat = latencia(ms, PRAZO.escolha);
  const noPrazo = acerto.filter(i => msTotal(i) <= PRAZO.escolha);
  const falhas = {};
  xs.filter(i => i.falha).forEach(i => { falhas[i.falha] = (falhas[i.falha] || 0) + 1; });
  return { casos:xs.length, respondeu:respondeu.length, acertos:acerto.length, acertoPct:pct(acerto.length, xs.length),
    estritos:estrito.length, estritoPct:pct(estrito.length, xs.length),
    acertosNoPrazo:noPrazo.length, acertoNoPrazoPct:pct(noPrazo.length, xs.length),
    segundaTentativa:xs.filter(i => (i.tentativas || []).length > 1).length,
    erradas:xs.filter(i => !i.falha && !(i.aceitaveis || [i.esperada]).includes(i.escolha)).map(i => i.id),
    falhas, latencia:lat };
}

// Resumo. "Número inventado" é a recusa da conferência por número (motivo 'numero'): conta na 1ª
// tentativa (o que o modelo faz sozinho) e no final (o que seria publicado: sempre 0, porque a
// conferência descarta o resumo recusado). aprovados: passou na conferência; noPrazo: aprovado dentro do prazo.
function resumo(itens){
  const xs = itens.filter(i => !i.controle);
  const respondeu = xs.filter(i => i.falha !== 'modelo');
  const primeira = respondeu.map(i => (i.tentativas || [])[0]).filter(Boolean);
  const inventou1 = primeira.filter(t => t.motivo === 'numero').length;
  const formato1 = primeira.filter(t => t.motivo === 'formato').length;
  const aprovados = xs.filter(i => !i.falha);
  const noPrazo = aprovados.filter(i => msTotal(i) <= PRAZO.resumo);
  const legiveis1 = primeira.length - formato1;
  const trocadas = aprovados.filter(i => (i.frasesTrocadas || []).length);
  const falhas = {};
  xs.filter(i => i.falha).forEach(i => { falhas[i.falha] = (falhas[i.falha] || 0) + 1; });
  return { casos:xs.length, respondeu:respondeu.length,
    primeiraSemInventado:legiveis1 - inventou1, primeiraLegiveis:legiveis1, primeiraSemInventadoPct:pct(legiveis1 - inventou1, legiveis1),
    aprovados:aprovados.length, aprovadosPct:pct(aprovados.length, xs.length),
    aprovadosNoPrazo:noPrazo.length, aprovadosNoPrazoPct:pct(noPrazo.length, xs.length),
    segundaTentativa:xs.filter(i => (i.tentativas || []).length > 1).length,
    comFraseTrocada:trocadas.map(i => i.id), falhas, latencia:latencia(respondeu.map(msTotal), PRAZO.resumo) };
}

// ---- frase com maior e menor trocados (verificação automática parcial) ----
// O erro mais comum da bateria da História 3: a frase diz "maior" e cita o número do menor valor da
// coluna (ou o contrário). Só pega esse tipo; as outras frases falsas pedem leitura humana.
const RE_NUM = /\d{1,3}(?:\.\d{3})+(?:,\d+)?|\d+(?:,\d+)?/g;
const numeroBR = s => Number(s.replace(/\./g, '').replace(',', '.'));
const MAIOR = /\b(maior|mais alt[oa]|pico|melhor|líder|lidera)\b/i;
const MENOR = /\b(menor|mais baix[oa]|pior|mínim[oa])\b/i;

function frasesTrocadas(linhas, todas, y){
  const vs = (todas || []).map(l => Number(l && l[y])).filter(Number.isFinite);
  if(vs.length < 2) return [];
  const max = Math.max.apply(null, vs), min = Math.min.apply(null, vs);
  if(max === min) return [];
  const perto = (a, b) => Math.abs(a - b) < 0.006 || (Math.abs(b) >= 1000 && Math.abs(a - Math.round(b)) < 0.5);
  return (linhas || []).filter(f => {
    const ns = (String(f).match(RE_NUM) || []).map(numeroBR);
    const temMin = ns.some(n => perto(n, min)), temMax = ns.some(n => perto(n, max));
    const diz = { maior:MAIOR.test(f), menor:MENOR.test(f) };
    return (diz.maior && !diz.menor && temMin && !temMax) || (diz.menor && !diz.maior && temMax && !temMin);
  });
}

// ---- duas rodadas iguais (H6-CT02) ----
// Caso a caso: o que mudou de uma rodada para a outra (escolha, aprovação, texto do resumo).
function comparar(a, b){
  const porId = new Map((b || []).map(i => [i.etapa + ':' + i.id, i]));
  const difs = [];
  let iguais = 0, total = 0;
  for(const x of a || []){
    if(x.controle) continue;
    const y = porId.get(x.etapa + ':' + x.id);
    if(!y) continue;
    total++;
    const d = [];
    if(x.etapa === 'escolha' && x.escolha !== y.escolha) d.push('escolha: "' + x.escolha + '" → "' + y.escolha + '"');
    if((x.falha || '') !== (y.falha || '')) d.push('falha: ' + (x.falha || 'nenhuma') + ' → ' + (y.falha || 'nenhuma'));
    if(x.etapa !== 'escolha' && JSON.stringify(x.linhas || []) !== JSON.stringify(y.linhas || [])) d.push('texto do resumo mudou');
    if(d.length) difs.push({ id:x.id, etapa:x.etapa, mudou:d }); else iguais++;
  }
  return { total, iguais, diferentes:difs.length, difs };
}

module.exports = { PRAZO, quantil, latencia, escolha, resumo, frasesTrocadas, comparar, msTotal };
