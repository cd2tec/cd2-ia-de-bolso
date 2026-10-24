/* Agendamento das consultas salvas: aqui só a lógica pura de frequência, sem DOM nem Intl (o cd2-ctl
   carrega este mesmo arquivo num interpretador JS embutido). Valida a agenda, lista os horários num
   intervalo, acha o próximo e escreve o resumo em português.
   Tudo em hora local e contado em dias de calendário (new Date(a, m, d, hh, mm)), nunca somando
   milissegundos: o horário de verão não pode deslocar a hora. */
(function(root){
"use strict";
const isNode = typeof module !== 'undefined' && module.exports;

const MAX_HORARIOS = 20000;
const FREQS = ['minutos', 'horas', 'diario', 'semanal', 'mensal'];
const ERRO_MIN = 'O intervalo mínimo é de 5 minutos.';
const ERRO_DIAS = 'Escolha pelo menos um dia.';
const ERRO_HORA = 'Horário inválido.';
const ERRO_DIA_MES = 'Dia do mês inválido.';
const ERRO_INTERVALO = 'Intervalo inválido.';
const ERRO_DATA = 'Data de início inválida.';
const ERRO_FREQ = 'Frequência inválida.';
const NOMES = ['domingo', 'segunda', 'terça', 'quarta', 'quinta', 'sexta', 'sábado'];
const CURTOS = ['dom', 'seg', 'ter', 'qua', 'qui', 'sex', 'sáb'];
const SEMANA = [1, 2, 3, 4, 5, 6, 0]; // segunda primeiro, para achar sequências
const ANCORA = '2000-01-03'; // segunda; âncora quando falta `inicio`

const pad = n => (n < 10 ? '0' : '') + n;
const hhmm = min => pad(Math.floor(min / 60)) + ':' + pad(min % 60);
const inteiro = v => typeof v === 'number' && isFinite(v) && Math.floor(v) === v;

// 'HH:MM' -> minutos desde 00:00, ou null.
function minutosDe(s){
  const m = /^(\d{2}):(\d{2})$/.exec(String(s == null ? '' : s));
  if(!m) return null;
  const h = +m[1], mi = +m[2];
  return h < 24 && mi < 60 ? h * 60 + mi : null;
}
// 'AAAA-MM-DD' -> nº do dia de calendário (UTC, sem horário de verão), ou null se não existe.
function diaDe(s){
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(String(s == null ? '' : s));
  if(!m) return null;
  const y = +m[1], mo = +m[2] - 1, d = +m[3], t = Date.UTC(y, mo, d);
  const c = new Date(t);
  return c.getUTCFullYear() === y && c.getUTCMonth() === mo && c.getUTCDate() === d ? t / 864e5 : null;
}
const numDia = (y, m, d) => Date.UTC(y, m, d) / 864e5;
const ultimoDiaDoMes = (y, m) => new Date(y, m + 1, 0).getDate();
const ancora = a => { const n = diaDe(a.inicio); return n == null ? diaDe(ANCORA) : n; };
const modulo = (a, n) => ((a % n) + n) % n;
const dow = n => modulo(n + 4, 7); // 1970-01-01 foi quinta
const diasDe = a => (Array.isArray(a.dias) ? a.dias : []).filter(d => inteiro(d) && d >= 0 && d <= 6);

function validar(a){
  if(!a || FREQS.indexOf(a.freq) < 0) return { ok:false, erro:ERRO_FREQ };
  const f = a.freq, n = a.a_cada;
  if(f === 'minutos' || f === 'horas'){
    if(f === 'minutos' && typeof n === 'number' && n < 5) return { ok:false, erro:ERRO_MIN };
    if(!inteiro(n) || n < 1) return { ok:false, erro:ERRO_INTERVALO };
    if(a.janela){
      const de = minutosDe(a.janela.de), ate = minutosDe(a.janela.ate);
      if(de == null || ate == null || ate < de) return { ok:false, erro:ERRO_HORA };
    }
    return { ok:true };
  }
  if(minutosDe(a.hora) == null) return { ok:false, erro:ERRO_HORA };
  if(f === 'diario'){
    if(!inteiro(n) || n < 1) return { ok:false, erro:ERRO_INTERVALO };
  } else if(f === 'semanal'){
    if(n !== 1 && n !== 2) return { ok:false, erro:ERRO_INTERVALO };
    if(!diasDe(a).length) return { ok:false, erro:ERRO_DIAS };
  } else if(!(a.dia_mes === 'ultimo' || (inteiro(a.dia_mes) && a.dia_mes >= 1 && a.dia_mes <= 31))){
    return { ok:false, erro:ERRO_DIA_MES };
  }
  if(a.inicio != null && a.inicio !== '' && diaDe(a.inicio) == null) return { ok:false, erro:ERRO_DATA };
  return { ok:true };
}

// Minutos do dia (0..1439) em que a agenda dispara no dia de calendário y-m-d (m de 0 a 11).
function minutosDoDia(a, y, m, d){
  const n = numDia(y, m, d), dw = dow(n);
  if(a.freq === 'minutos' || a.freq === 'horas'){
    const dias = diasDe(a);
    if(dias.length && dias.indexOf(dw) < 0) return [];
    const j = a.janela;
    const ini = j ? minutosDe(j.de) : 0, fim = j ? minutosDe(j.ate) : 1439;
    const passo = a.freq === 'horas' ? a.a_cada * 60 : a.a_cada, out = [];
    for(let t = ini; t <= fim; t += passo) out.push(t);
    return out;
  }
  const h = minutosDe(a.hora);
  if(a.freq === 'diario') return modulo(n - ancora(a), a.a_cada) === 0 ? [h] : [];
  if(a.freq === 'semanal'){
    if(diasDe(a).indexOf(dw) < 0) return [];
    const seg = x => x - modulo(dow(x) + 6, 7); // segunda da semana
    return modulo((seg(n) - seg(ancora(a))) / 7, a.a_cada) === 0 ? [h] : [];
  }
  const ult = ultimoDiaDoMes(y, m);
  const alvo = a.dia_mes === 'ultimo' ? ult : Math.min(a.dia_mes, ult);
  return d === alvo ? [h] : [];
}

// Horários em de < h <= ate, hora local, no máximo MAX_HORARIOS.
function horarios(a, de, ate){
  const out = [];
  if(!a || a.ligado === false || !validar(a).ok || !(de < ate)) return out;
  const y0 = de.getFullYear(), m0 = de.getMonth(), d0 = de.getDate();
  for(let i = 0; ; i++){
    const dia = new Date(y0, m0, d0 + i);
    if(dia > ate) break;
    const y = dia.getFullYear(), m = dia.getMonth(), d = dia.getDate();
    const mins = minutosDoDia(a, y, m, d);
    for(let k = 0; k < mins.length; k++){
      const h = new Date(y, m, d, Math.floor(mins[k] / 60), mins[k] % 60);
      if(h > de && h <= ate){
        out.push(h);
        if(out.length >= MAX_HORARIOS) return out;
      }
    }
  }
  return out;
}

// Primeiro horário depois de `depois`, ou null (desligada, inválida ou nada em ~11 anos).
function proximo(a, depois){
  if(!a || a.ligado === false || !validar(a).ok) return null;
  const y0 = depois.getFullYear(), m0 = depois.getMonth(), d0 = depois.getDate();
  for(let i = 0; i < 4000; i++){
    const dia = new Date(y0, m0, d0 + i);
    const y = dia.getFullYear(), m = dia.getMonth(), d = dia.getDate();
    const mins = minutosDoDia(a, y, m, d);
    for(let k = 0; k < mins.length; k++){
      const h = new Date(y, m, d, Math.floor(mins[k] / 60), mins[k] % 60);
      if(h > depois) return h;
    }
  }
  return null;
}

// Dias da semana em texto: 'todos os dias', 'de segunda a sábado' (3+ seguidos) ou 'segunda, quarta e sexta'.
function textoDias(a, nomes, prefixo){
  const dias = diasDe(a), set = SEMANA.filter(d => dias.indexOf(d) >= 0);
  if(!set.length || set.length === 7) return null;
  const i0 = SEMANA.indexOf(set[0]), i1 = SEMANA.indexOf(set[set.length - 1]);
  if(set.length >= 3 && i1 - i0 + 1 === set.length) return (prefixo ? 'de ' : '') + nomes[set[0]] + ' a ' + nomes[set[set.length - 1]];
  const l = set.map(d => nomes[d]);
  return l.length === 1 ? l[0] : prefixo ? l.slice(0, -1).join(', ') + ' e ' + l[l.length - 1] : l.join(', ');
}

function palavraDia(d, agora){
  const dif = numDia(d.getFullYear(), d.getMonth(), d.getDate()) - numDia(agora.getFullYear(), agora.getMonth(), agora.getDate());
  return dif === 0 ? 'hoje' : dif === 1 ? 'amanhã' : pad(d.getDate()) + '/' + pad(d.getMonth() + 1);
}

function resumo(a, agora){
  if(!a || a.ligado === false) return 'Desligado.';
  const v = validar(a);
  if(!v.ok) return v.erro;
  let t;
  if(a.freq === 'minutos' || a.freq === 'horas'){
    const dias = textoDias(a, NOMES, true) || 'todos os dias';
    const f = a.freq === 'horas' ? (a.a_cada === 1 ? 'de hora em hora' : 'a cada ' + a.a_cada + ' horas')
      : 'a cada ' + a.a_cada + ' minutos';
    const j = a.janela ? 'das ' + a.janela.de + ' às ' + a.janela.ate : 'o dia todo';
    t = dias + ', ' + f + ', ' + j;
  } else if(a.freq === 'diario'){
    t = (a.a_cada === 1 ? 'todo dia' : 'a cada ' + a.a_cada + ' dias') + ', às ' + a.hora;
  } else if(a.freq === 'semanal'){
    const dias = textoDias(a, NOMES, true) || 'todos os dias';
    t = (a.a_cada === 2 ? 'a cada 2 semanas, ' : '') + dias + ', às ' + a.hora;
  } else {
    t = (a.dia_mes === 'ultimo' ? 'no último dia do mês' : 'todo dia ' + a.dia_mes) + ', às ' + a.hora;
  }
  let s = 'Vai rodar ' + t + '.';
  const p = proximo(a, agora);
  if(p) s += ' Próxima: ' + palavraDia(p, agora) + ', ' + pad(p.getHours()) + ':' + pad(p.getMinutes()) + '.';
  return s;
}

function rotuloCurto(a){
  if(!a) return '';
  if(a.freq === 'minutos') return 'a cada ' + a.a_cada + ' min';
  if(a.freq === 'horas') return 'a cada ' + a.a_cada + ' h';
  if(a.freq === 'diario') return a.a_cada === 1 ? 'todo dia ' + a.hora : 'a cada ' + a.a_cada + ' dias ' + a.hora;
  if(a.freq === 'semanal') return (a.a_cada === 2 ? 'a cada 2 sem: ' : '') + (textoDias(a, CURTOS, false) || 'todo dia');
  if(a.freq === 'mensal') return a.dia_mes === 'ultimo' ? 'último dia do mês' : 'todo dia ' + a.dia_mes;
  return '';
}

// ---- períodos relativos, calculados a cada execução ----
const PERIODOS = [
  { id:'hoje', rotulo:'Hoje', faixa:false, so_hora:false },
  { id:'ontem', rotulo:'Ontem', faixa:false, so_hora:false },
  { id:'hora_atual', rotulo:'Hora atual', faixa:false, so_hora:true },
  { id:'ultimas_horas', rotulo:'Últimas X horas', faixa:false, so_hora:true },
  { id:'ultimos_dias', rotulo:'Últimos X dias', faixa:true, so_hora:false },
  { id:'esta_semana', rotulo:'Esta semana', faixa:true, so_hora:false },
  { id:'semana_passada', rotulo:'Semana passada', faixa:true, so_hora:false },
  { id:'este_mes', rotulo:'Este mês', faixa:true, so_hora:false },
  { id:'mes_passado', rotulo:'Mês passado', faixa:true, so_hora:false }
];
const SIGLAS = ['DOM', 'SEG', 'TER', 'QUA', 'QUI', 'SEX', 'SAB'];
const ERRO_PERIODO = ': período inválido para este parâmetro.';

// Períodos oferecidos para um parâmetro; `temPar` diz se há outro parâmetro de data para fechar a faixa.
function periodosPara(tipo, temPar){
  if(tipo !== 'data' && tipo !== 'datahora') return [];
  return PERIODOS.filter(p => !(tipo === 'data' && p.so_hora) && !(p.faixa && !temPar));
}

const fmtData = dt => dt.getFullYear() + '-' + pad(dt.getMonth() + 1) + '-' + pad(dt.getDate());
const fmtHora = dt => pad(dt.getHours()) + ':' + pad(dt.getMinutes()) + ':' + pad(dt.getSeconds());

// Valor do parâmetro em `ref`, ou null se a combinação não vale.
function valorDoParam(p, tipo, ref){
  if(!p) return null;
  if(p.modo === 'fixo') return p.valor == null ? null : String(p.valor);
  const y = ref.getFullYear(), m = ref.getMonth(), d = ref.getDate();
  if(p.modo === 'dia_semana'){
    if(tipo !== 'numero' && tipo !== 'texto') return null;
    const w = ref.getDay();
    if(p.formato === '0a6') return String(w);
    if(p.formato === '1a7') return String(w + 1);
    return p.formato === 'sigla' ? SIGLAS[w] : null;
  }
  if(p.modo !== 'periodo' || (tipo !== 'data' && tipo !== 'datahora')) return null;
  const per = PERIODOS.filter(x => x.id === p.periodo)[0];
  if(!per || (p.ponta !== 'inicio' && p.ponta !== 'fim')) return null;
  if(per.so_hora && tipo !== 'datahora') return null;
  const ini = p.ponta === 'inicio';
  if(per.id === 'hora_atual') return fmtData(ref) + ' ' + (ini ? pad(ref.getHours()) + ':00:00' : fmtHora(ref));
  if(per.id === 'ultimas_horas'){
    if(!inteiro(p.n) || p.n < 1) return null;
    const x = ini ? new Date(y, m, d, ref.getHours() - p.n, ref.getMinutes(), ref.getSeconds()) : ref;
    return fmtData(x) + ' ' + fmtHora(x);
  }
  let a, b; // primeiro e último dia do período
  if(per.id === 'hoje') a = b = new Date(y, m, d);
  else if(per.id === 'ontem') a = b = new Date(y, m, d - 1);
  else if(per.id === 'ultimos_dias'){
    if(!inteiro(p.n) || p.n < 1) return null;
    a = new Date(y, m, d - p.n); b = new Date(y, m, d);
  } else if(per.id === 'esta_semana'){ a = new Date(y, m, d - ref.getDay()); b = new Date(y, m, d); }
  else if(per.id === 'semana_passada'){ a = new Date(y, m, d - ref.getDay() - 7); b = new Date(y, m, d - ref.getDay() - 1); }
  else if(per.id === 'este_mes'){ a = new Date(y, m, 1); b = new Date(y, m + 1, 0); }
  else { a = new Date(y, m - 1, 1); b = new Date(y, m, 0); } // mes_passado
  const dia = fmtData(ini ? a : b);
  return tipo === 'datahora' ? dia + (ini ? ' 00:00:00' : ' 23:59:59') : dia;
}

// Valores dos parâmetros da consulta para esta execução; sem agenda.params[i] usa o valor salvo.
function valoresDaExecucao(consulta, ref){
  const ps = (consulta && consulta.params) || [], ag = (consulta && consulta.agenda && consulta.agenda.params) || [];
  const valores = [];
  for(let i = 0; i < ps.length; i++){
    const a = ag[i];
    const v = a ? valorDoParam(a, ps[i].tipo, ref) : (ps[i].valor == null ? '' : String(ps[i].valor));
    if(v == null) return { ok:false, erro:ps[i].nome + ERRO_PERIODO };
    valores.push(v);
  }
  return { ok:true, valores };
}

// Mesmo período, na ponta final: sugestão para o segundo parâmetro de data.
const parSugerido = p => Object.assign({}, p, { ponta:'fim' });

const api = { MAX_HORARIOS, validar, horarios, proximo, resumo, rotuloCurto, PERIODOS, periodosPara, valorDoParam, valoresDaExecucao, parSugerido };
if(isNode) module.exports = api; else root.CD2Agenda = api;
})(typeof window !== 'undefined' ? window : globalThis);
