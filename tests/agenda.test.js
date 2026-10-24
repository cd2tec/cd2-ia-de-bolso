'use strict';
// Testes da frequência dos agendamentos (web/agenda.js). Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const A = require('../web/agenda.js');

test('a cada 1 h das 08 às 20, seg a sáb', () => {
  const a = { ligado:true, freq:'horas', a_cada:1, janela:{ de:'08:00', ate:'20:00' }, dias:[1,2,3,4,5,6] };
  const h = A.horarios(a, new Date(2026,9,24,6,0), new Date(2026,9,26,9,0)); // sáb 24 → seg 26
  assert.equal(h.length, 13 + 2); // sáb 08..20, seg 08 e 09; domingo fora
});
test('a cada 2 h conta a partir do início da janela', () => {
  const a = { ligado:true, freq:'horas', a_cada:2, janela:{ de:'08:00', ate:'20:00' } };
  const h = A.horarios(a, new Date(2026,9,24,0,0), new Date(2026,9,24,23,59));
  assert.deepEqual(h.map(d => d.getHours()), [8,10,12,14,16,18,20]);
});
test('intervalo é aberto em de e fechado em ate', () => {
  const a = { ligado:true, freq:'horas', a_cada:1 };
  const h = A.horarios(a, new Date(2026,9,24,10,0), new Date(2026,9,24,12,0));
  assert.deepEqual(h.map(d => d.getHours()), [11,12]);
});
test('mensal dia 31 cai no último dia', () => {
  const a = { ligado:true, freq:'mensal', dia_mes:31, hora:'07:00' };
  assert.deepEqual(A.horarios(a, new Date(2026,1,1), new Date(2026,2,1)).map(d => d.getDate()), [28]);
});
test('mensal "ultimo" e dia fixo', () => {
  const u = { ligado:true, freq:'mensal', dia_mes:'ultimo', hora:'07:00' };
  assert.deepEqual(A.horarios(u, new Date(2026,0,1), new Date(2026,3,1)).map(d => d.getDate()), [31, 28, 31]);
  const f = { ligado:true, freq:'mensal', dia_mes:15, hora:'07:00' };
  assert.deepEqual(A.horarios(f, new Date(2026,0,1), new Date(2026,2,1)).map(d => d.getDate()), [15, 15]);
});
test('a cada 2 semanas ancorado no início', () => {
  const a = { ligado:true, freq:'semanal', a_cada:2, hora:'07:00', dias:[1], inicio:'2026-10-19' };
  const h = A.horarios(a, new Date(2026,9,18), new Date(2026,10,20));
  assert.deepEqual(h.map(d => [d.getMonth() + 1, d.getDate()]), [[10,19], [11,2], [11,16]]);
});
test('semanal toda semana nos dias escolhidos', () => {
  const a = { ligado:true, freq:'semanal', a_cada:1, hora:'07:00', dias:[1,3] };
  const h = A.horarios(a, new Date(2026,9,19,0,0), new Date(2026,9,25,23,59));
  assert.deepEqual(h.map(d => d.getDate()), [19, 21]);
});
test('a cada 3 dias ancorado no início', () => {
  const a = { ligado:true, freq:'diario', a_cada:3, hora:'07:00', inicio:'2026-10-24' };
  const h = A.horarios(a, new Date(2026,9,23), new Date(2026,9,31));
  assert.deepEqual(h.map(d => d.getDate()), [24, 27, 30]);
  assert.equal(h[0].getHours(), 7);
});
test('a cada X dias e a cada 2 semanas não rodam antes do início', () => {
  const a = { ligado:true, freq:'diario', a_cada:3, hora:'07:00', inicio:'2026-11-01' };
  const h = A.horarios(a, new Date(2026,9,20), new Date(2026,10,8));
  assert.deepEqual(h.map(d => [d.getMonth() + 1, d.getDate()]), [[11,1], [11,4], [11,7]]);
  assert.deepEqual(A.proximo(a, new Date(2026,9,24,12,0)), new Date(2026,10,1,7,0));
  assert.ok(A.resumo(a, new Date(2026,9,24,12,0)).endsWith('Próxima: 01/11, 07:00.'));
  const s = { ligado:true, freq:'semanal', a_cada:2, hora:'07:00', dias:[1], inicio:'2026-11-02' };
  assert.deepEqual(A.horarios(s, new Date(2026,9,1), new Date(2026,10,20)).map(d => [d.getMonth() + 1, d.getDate()]), [[11,2], [11,16]]);
  assert.deepEqual(A.proximo(s, new Date(2026,9,24,12,0)), new Date(2026,10,2,7,0));
  const t = { ligado:true, freq:'diario', a_cada:1, hora:'07:00', inicio:'2026-11-01' };
  assert.deepEqual(A.proximo(t, new Date(2026,9,24,12,0)), new Date(2026,10,1,7,0));
});
test('resumo semanal diz "toda segunda", "todo sábado" e mantém "a cada 2 semanas"', () => {
  const agora = new Date(2026,9,24,12,0);
  const r = a => A.resumo({ ligado:true, freq:'semanal', a_cada:1, hora:'07:00', ...a }, agora).split('.')[0];
  assert.equal(r({ dias:[1] }), 'Vai rodar toda segunda, às 07:00');
  assert.equal(r({ dias:[1,3] }), 'Vai rodar toda segunda e quarta, às 07:00');
  assert.equal(r({ dias:[1,3,5] }), 'Vai rodar toda segunda, quarta e sexta, às 07:00');
  assert.equal(r({ dias:[6] }), 'Vai rodar todo sábado, às 07:00');
  assert.equal(r({ dias:[0] }), 'Vai rodar todo domingo, às 07:00');
  assert.equal(r({ dias:[1,2,3,4,5] }), 'Vai rodar de segunda a sexta, às 07:00');
  assert.equal(r({ dias:[1], a_cada:2, inicio:'2026-10-19' }), 'Vai rodar a cada 2 semanas, toda segunda, às 07:00');
});
test('validar recusa 4 minutos', () => assert.equal(A.validar({ freq:'minutos', a_cada:4 }).erro, 'O intervalo mínimo é de 5 minutos.'));
test('validar: outros erros e aceitos', () => {
  assert.equal(A.validar({ freq:'semanal', a_cada:1, hora:'07:00', dias:[] }).erro, 'Escolha pelo menos um dia.');
  assert.equal(A.validar({ freq:'diario', a_cada:1, hora:'25:00' }).erro, 'Horário inválido.');
  assert.equal(A.validar({ freq:'horas', a_cada:1, janela:{ de:'20:00', ate:'08:00' } }).erro, 'Horário inválido.');
  assert.equal(A.validar({ freq:'mensal', hora:'07:00', dia_mes:32 }).erro, 'Dia do mês inválido.');
  assert.deepEqual(A.validar({ freq:'minutos', a_cada:5 }), { ok:true });
  assert.deepEqual(A.validar({ freq:'mensal', hora:'07:00', dia_mes:'ultimo' }), { ok:true });
});
test('proximo de agenda desligada é null', () => assert.equal(A.proximo({ ligado:false, freq:'horas', a_cada:1 }, new Date()), null));
test('proximo acha o horário seguinte, até no outro dia', () => {
  const a = { ligado:true, freq:'horas', a_cada:1, janela:{ de:'08:00', ate:'20:00' }, dias:[1,2,3,4,5,6] };
  assert.deepEqual(A.proximo(a, new Date(2026,9,24,14,0)), new Date(2026,9,24,15,0));
  assert.deepEqual(A.proximo(a, new Date(2026,9,24,20,0)), new Date(2026,9,26,8,0)); // domingo fora
});
test('resumo e rótulo', () => {
  const a = { ligado:true, freq:'horas', a_cada:1, janela:{ de:'08:00', ate:'20:00' }, dias:[1,2,3,4,5,6] };
  const agora = new Date(2026,9,24,14,0);
  assert.equal(A.resumo(a, agora), 'Vai rodar de segunda a sábado, de hora em hora, das 08:00 às 20:00. Próxima: hoje, 15:00.');
  assert.equal(A.resumo(a, new Date(2026,9,24,20,0)), 'Vai rodar de segunda a sábado, de hora em hora, das 08:00 às 20:00. Próxima: 26/10, 08:00.');
  assert.equal(A.resumo(a, new Date(2026,9,23,20,0)), 'Vai rodar de segunda a sábado, de hora em hora, das 08:00 às 20:00. Próxima: amanhã, 08:00.');
  assert.equal(A.resumo({ ...a, ligado:false }, agora), 'Desligado.');
  assert.equal(A.rotuloCurto(a), 'a cada 1 h');
  assert.equal(A.rotuloCurto({ freq:'minutos', a_cada:15 }), 'a cada 15 min');
  assert.equal(A.rotuloCurto({ freq:'diario', a_cada:1, hora:'07:30' }), 'todo dia 07:30');
  assert.equal(A.rotuloCurto({ freq:'semanal', a_cada:1, hora:'07:30', dias:[1,2,3,4,5,6] }), 'seg a sáb');
  assert.equal(A.rotuloCurto({ freq:'mensal', dia_mes:15, hora:'07:00' }), 'todo dia 15');
  assert.equal(A.rotuloCurto({ freq:'mensal', dia_mes:'ultimo', hora:'07:00' }), 'último dia do mês');
});
test('horarios corta em MAX_HORARIOS', () => {
  const a = { ligado:true, freq:'minutos', a_cada:5 };
  assert.equal(A.horarios(a, new Date(2026,0,1), new Date(2027,0,1)).length, A.MAX_HORARIOS);
});

// ---- períodos calculados a cada execução ----
const ref = new Date(2026, 9, 24, 14, 37, 12); // sábado
const d = (periodo, ponta, tipo = 'data', n) => A.valorDoParam({ modo:'periodo', periodo, ponta, n }, tipo, ref);
test('dias', () => { assert.equal(d('hoje','inicio'), '2026-10-24'); assert.equal(d('ontem','fim'), '2026-10-23'); });
test('semana de domingo a sábado', () => {
  assert.equal(d('esta_semana','inicio'), '2026-10-18'); assert.equal(d('esta_semana','fim'), '2026-10-24');
  const qua = new Date(2026, 9, 21, 9, 0); // quarta: a semana ainda termina no sábado
  assert.equal(A.valorDoParam({ modo:'periodo', periodo:'esta_semana', ponta:'inicio' }, 'data', qua), '2026-10-18');
  assert.equal(A.valorDoParam({ modo:'periodo', periodo:'esta_semana', ponta:'fim' }, 'data', qua), '2026-10-24');
  assert.equal(A.valorDoParam({ modo:'periodo', periodo:'esta_semana', ponta:'fim' }, 'datahora', qua), '2026-10-24 23:59:59');
  assert.equal(d('semana_passada','inicio'), '2026-10-11'); assert.equal(d('semana_passada','fim'), '2026-10-17');
});
test('mês', () => { assert.equal(d('este_mes','fim'), '2026-10-31'); assert.equal(d('mes_passado','inicio'), '2026-09-01'); });
test('horas', () => {
  assert.equal(d('hora_atual','inicio','datahora'), '2026-10-24 14:00:00');
  assert.equal(d('hora_atual','fim','datahora'), '2026-10-24 14:37:12');
  assert.equal(d('ultimas_horas','inicio','datahora', 2), '2026-10-24 12:37:12');
  assert.equal(d('ultimos_dias','inicio','data', 7), '2026-10-17');
});
test('fim de datahora de período de dia vai até 23:59:59', () => {
  assert.equal(d('hoje','fim','datahora'), '2026-10-24 23:59:59');
  assert.equal(d('hoje','inicio','datahora'), '2026-10-24 00:00:00');
});
test('período de hora não vale para data; n inválido e tipo numero dão null', () => {
  assert.equal(d('hora_atual','inicio','data'), null);
  assert.equal(d('ultimos_dias','inicio','data', 0), null);
  assert.equal(d('hoje','inicio','numero'), null);
});
test('dia da semana', () => {
  const v = f => A.valorDoParam({ modo:'dia_semana', formato:f }, 'numero', ref);
  assert.equal(v('0a6'), '6'); assert.equal(v('1a7'), '7');
  assert.equal(A.valorDoParam({ modo:'dia_semana', formato:'sigla' }, 'texto', ref), 'SAB');
});
test('valoresDaExecucao mistura fixo e período', () => {
  const c = { params:[{ tipo:'data', nome:'ini', valor:'2020-01-01' }, { tipo:'data', nome:'fim', valor:'2020-01-01' }, { tipo:'numero', nome:'n', valor:'9' }],
    agenda:{ params:[{ modo:'periodo', periodo:'hoje', ponta:'inicio' }, { modo:'periodo', periodo:'hoje', ponta:'fim' }, { modo:'fixo', valor:'01' }] } };
  assert.deepEqual(A.valoresDaExecucao(c, ref), { ok:true, valores:['2026-10-24','2026-10-24','01'] });
});
test('valoresDaExecucao usa o valor salvo sem agenda e rejeita combinação inválida', () => {
  const c = { params:[{ tipo:'numero', nome:'n', valor:'9' }, { tipo:'texto', nome:'t', valor:'x' }] };
  assert.deepEqual(A.valoresDaExecucao(c, ref), { ok:true, valores:['9','x'] });
  c.agenda = { params:[{ modo:'periodo', periodo:'hoje', ponta:'inicio' }] };
  assert.deepEqual(A.valoresDaExecucao(c, ref), { ok:false, erro:'n: período inválido para este parâmetro.' });
});
test('periodosPara', () => {
  const ids = (t, par) => A.periodosPara(t, par).map(p => p.id);
  const sd = ids('data', false);
  assert.ok(sd.includes('hoje') && !sd.includes('esta_semana') && !sd.includes('hora_atual') && !sd.includes('ultimas_horas'));
  assert.deepEqual(ids('datahora', true), A.PERIODOS.map(p => p.id));
  assert.ok(!ids('datahora', false).includes('ultimos_dias') && ids('datahora', false).includes('hora_atual'));
  assert.deepEqual(ids('numero', true), []);
  assert.deepEqual(A.PERIODOS.map(p => p.rotulo), ['Hoje','Ontem','Hora atual','Últimas X horas','Últimos X dias','Esta semana','Semana passada','Este mês','Mês passado']);
});
test('parSugerido põe ponta fim sem mudar o original', () => {
  const p = { modo:'periodo', periodo:'ultimos_dias', n:3, ponta:'inicio' };
  assert.deepEqual(A.parSugerido(p), { modo:'periodo', periodo:'ultimos_dias', n:3, ponta:'fim' });
  assert.equal(p.ponta, 'inicio');
});

// ---- o que a janela Agendar e a barra lateral usam ----
const PS = [{ tipo:'data', nome:'Data inicial', valor:'2026-10-01' }, { tipo:'datahora', nome:'Data final', valor:'2026-10-02 00:00:00' },
  { tipo:'numero', nome:'Loja', valor:'1' }, { tipo:'texto', nome:'UF', valor:'PR' }];
test('agendaPadrao: de hora em hora, todos os dias, período de hoje nas datas e o valor salvo no resto', () => {
  const a = A.agendaPadrao(PS, '2026-10-24');
  assert.deepEqual(a, { ligado:true, freq:'horas', a_cada:1, janela:null, dias:[], inicio:'2026-10-24', params:[
    { modo:'periodo', periodo:'hoje', ponta:'inicio' }, { modo:'periodo', periodo:'hoje', ponta:'fim' },
    { modo:'fixo', valor:'1' }, { modo:'fixo', valor:'PR' }] });
  assert.ok(A.validar(a).ok);
  assert.deepEqual(A.agendaPadrao([], '2026-10-24').params, []);
});
test('pontaDe: o segundo parâmetro de data é o fim, os outros o início', () => {
  assert.deepEqual(PS.map((p, i) => A.pontaDe(PS, i)), ['inicio', 'fim', 'inicio', 'inicio']);
  const tres = [{ tipo:'data' }, { tipo:'data' }, { tipo:'data' }];
  assert.deepEqual(tres.map((p, i) => A.pontaDe(tres, i)), ['inicio', 'fim', 'inicio']);
});
test('textoProxima: HH:MM hoje, dd/mm HH:MM em outro dia, null desligada', () => {
  const a = { ligado:true, freq:'horas', a_cada:1, janela:{ de:'08:00', ate:'20:00' }, dias:[1,2,3,4,5,6] };
  assert.equal(A.textoProxima(a, new Date(2026,9,24,14,0)), '15:00');
  assert.equal(A.textoProxima(a, new Date(2026,9,24,20,0)), '26/10 08:00');
  assert.equal(A.textoProxima({ ...a, ligado:false }, new Date(2026,9,24,14,0)), null);
});
test('marcaBarra', () => {
  const a = { ligado:true, freq:'horas', a_cada:1 };
  assert.equal(A.marcaBarra(a, new Date(2026,9,24,14,10)), '⏰ a cada 1 h · próxima 15:00');
  assert.equal(A.marcaBarra({ ...a, ligado:false }, new Date(2026,9,24,14,10)), '⏰ a cada 1 h · desligado');
});
test('tela de ida e volta guarda a agenda', () => {
  const casos = [
    { ligado:true, freq:'horas', a_cada:2, janela:{ de:'08:00', ate:'20:00' }, dias:[1,2,3,4,5,6], inicio:'2026-10-24', params:[] },
    { ligado:false, freq:'minutos', a_cada:5, janela:null, dias:[], inicio:'2026-10-24', params:[] },
    { ligado:true, freq:'diario', a_cada:3, hora:'07:30', inicio:'2026-10-24', params:[] },
    { ligado:true, freq:'semanal', a_cada:2, hora:'07:30', dias:[1,3,5], inicio:'2026-10-19', params:[] },
    { ligado:true, freq:'mensal', dia_mes:'ultimo', hora:'06:00', inicio:'2026-10-24', params:[] },
    { ligado:true, freq:'mensal', dia_mes:15, hora:'06:00', inicio:'2026-10-24', params:[{ modo:'fixo', valor:'1' }] }
  ];
  for(const a of casos) assert.deepEqual(A.agendaDaTela(A.telaDaAgenda(a, '2026-10-24')), { ok:true, agenda:a });
});
test('tela: valores que vêm dos campos (texto) e troca de frequência', () => {
  const t = A.telaDaAgenda(A.agendaPadrao([], '2026-10-24'), '2026-10-24');
  assert.equal(t.dias.filter(Boolean).length, 7);
  t.freq = 'minutos'; t.n.minutos = '3';
  assert.deepEqual(A.agendaDaTela(t), { ok:false, erro:'O intervalo mínimo é de 5 minutos.' });
  t.n.minutos = '10';
  assert.equal(A.agendaDaTela(t).agenda.a_cada, 10);
  t.n.minutos = 'x';
  assert.deepEqual(A.agendaDaTela(t), { ok:false, erro:'Intervalo inválido.' });
  t.freq = 'semanal'; t.semanas = '1';
  const s = A.agendaDaTela(t).agenda;
  assert.deepEqual(s.dias, [0,1,2,3,4,5,6]); // semanal guarda os dias marcados, mesmo todos
  t.freq = 'mensal'; t.dia_mes = '31';
  assert.equal(A.agendaDaTela(t).agenda.dia_mes, 31);
});
test('tela sem nenhum dia marcado recusa, também de hora em hora', () => {
  const t = A.telaDaAgenda(A.agendaPadrao([], '2026-10-24'), '2026-10-24');
  t.dias = [false, false, false, false, false, false, false];
  assert.deepEqual(A.agendaDaTela(t), { ok:false, erro:'Escolha pelo menos um dia.' });
  t.freq = 'mensal'; // mensal não usa os dias
  assert.equal(A.agendaDaTela(t).ok, true);
});
test('tela de agenda semanal sem dias abre com segunda a sexta', () => {
  const t = A.telaDaAgenda({ ligado:true, freq:'semanal', a_cada:1, hora:'07:00', dias:[] }, '2026-10-24');
  assert.deepEqual(t.dias, [false, true, true, true, true, true, false]);
  assert.equal(t.inicio, '2026-10-24');
});

// ---- histórico: dias, horários e textos dos cards ----
// RFC3339 em hora local, como o cd2-ctl manda (time.Time no JSON), com ou sem nanossegundos.
function iso(y, m, d, h, mi, frac){
  const dt = new Date(y, m, d, h, mi), off = -dt.getTimezoneOffset(), p = n => String(n).padStart(2, '0');
  const z = off === 0 ? 'Z' : (off > 0 ? '+' : '-') + p(Math.floor(Math.abs(off) / 60)) + ':' + p(Math.abs(off) % 60);
  return y + '-' + p(m + 1) + '-' + p(d) + 'T' + p(h) + ':' + p(mi) + ':00' + (frac || '') + z;
}
test('dataDe lê o RFC3339 do cd2-ctl, com nanossegundos, Z e fuso', () => {
  assert.deepEqual(A.dataDe(iso(2026,9,24,14,0,'.123456789')), new Date(2026,9,24,14,0,0,123));
  assert.equal(A.dataDe('2026-10-24T17:00:00Z').getTime(), Date.UTC(2026,9,24,17,0));
  assert.equal(A.dataDe('2026-10-24T14:00:00-03:00').getTime(), Date.UTC(2026,9,24,17,0));
  assert.equal(A.dataDe('ontem'), null);
  assert.equal(A.dataDe(null), null);
});
test('rotuloDia: Hoje, Ontem e o dia da semana', () => {
  const agora = new Date(2026,9,24,15,0); // sábado
  assert.equal(A.rotuloDia(new Date(2026,9,24,0,5), agora), 'Hoje, 24/10');
  assert.equal(A.rotuloDia(new Date(2026,9,23,23,59), agora), 'Ontem, 23/10');
  assert.equal(A.rotuloDia(new Date(2026,9,16,8,0), agora), 'sex, 16/10');
  assert.equal(A.rotuloDia(new Date(2025,11,31,8,0), new Date(2026,0,2,8,0)), 'qua, 31/12/2025');
});
test('agruparPorDia junta as seguidas do mesmo dia, na ordem que vieram', () => {
  const agora = new Date(2026,9,24,15,0);
  const itens = [
    { seq:5, tipo:'manual', ref:iso(2026,9,24,15,4), em:iso(2026,9,24,15,4) },
    { seq:4, tipo:'agendada', ref:iso(2026,9,24,14,0), em:iso(2026,9,24,14,0) },
    { seq:3, tipo:'perda', ref:iso(2026,9,23,20,0), em:iso(2026,9,24,8,0), de:iso(2026,9,23,20,0), ate:iso(2026,9,23,20,0), n:1 },
    { seq:2, tipo:'agendada', ref:iso(2026,9,23,19,0), em:iso(2026,9,23,19,0) },
    { seq:1, tipo:'agendada', ref:iso(2026,9,16,19,0), em:iso(2026,9,16,19,0) }
  ];
  const g = A.agruparPorDia(itens, agora);
  assert.deepEqual(g.map(x => [x.rotulo, x.itens.map(e => e.seq)]), [['Hoje, 24/10', [5,4]], ['Ontem, 23/10', [3,2]], ['sex, 16/10', [1]]]);
  assert.deepEqual(A.agruparPorDia([], agora), []);
});
test('textoExecucao: agendada, manual e manual de uma perda', () => {
  assert.equal(A.textoExecucao({ tipo:'agendada', ref:iso(2026,9,24,14,0), em:iso(2026,9,24,14,0,'.5') }), 'Rodou às 14:00 (agendada)');
  assert.equal(A.textoExecucao({ tipo:'manual', ref:iso(2026,9,24,13,12), em:iso(2026,9,24,13,12) }), 'Rodou às 13:12 (manual)');
  assert.equal(A.textoExecucao({ tipo:'manual', ref:iso(2026,9,24,12,0), em:iso(2026,9,24,15,4) }), 'Rodou às 15:04 (referente a 12:00)');
  assert.equal(A.textoExecucao({ tipo:'manual', ref:iso(2026,9,23,20,0), em:iso(2026,9,24,8,1) }), 'Rodou às 08:01 (referente a sex 20:00)');
});
test('perda: título de um horário e de vários, motivo e rodada', () => {
  const uma = { tipo:'perda', motivo:'fechada', de:iso(2026,9,24,12,0), ate:iso(2026,9,24,12,0), ref:iso(2026,9,24,12,0), em:iso(2026,9,24,15,0), n:1 };
  assert.equal(A.tituloPerda(uma), 'Não rodou às 12:00');
  assert.equal(A.textoMotivo(uma), 'A IA de Bolso estava fechada nesse horário.');
  assert.equal(A.textoRodada(uma), null);
  assert.equal(A.textoRodada({ ...uma, rodada:iso(2026,9,24,15,4) }), 'rodada às 15:04');
  assert.equal(A.textoRodada({ ...uma, rodada:iso(2026,9,26,9,30) }), 'rodada às seg 09:30');
  const varias = { tipo:'perda', motivo:'dormindo', de:iso(2026,9,23,20,0), ate:iso(2026,9,26,7,55), ref:iso(2026,9,26,7,55), em:iso(2026,9,26,8,0), n:812 };
  assert.equal(A.tituloPerda(varias), 'Não rodou 812 vezes entre sex 20:00 e seg 07:55');
  assert.equal(A.textoMotivo(varias), 'O computador estava dormindo nesses horários.');
  assert.equal(A.textoMotivo({ motivo:'outro_banco', n:1 }), 'Outro banco estava conectado nesse horário.');
  assert.equal(A.textoMotivo({ motivo:'anterior', n:1 }), 'A anterior ainda estava rodando nesse horário.');
  assert.equal(A.textoMotivo({ motivo:'xyz', n:1 }), '');
});
test('a perda fica no dia do horário perdido', () => {
  const p = { tipo:'perda', de:iso(2026,9,23,20,0), ate:iso(2026,9,23,21,0), ref:iso(2026,9,23,21,0), em:iso(2026,9,24,8,0), n:2 };
  assert.deepEqual(A.diaDaExecucao(p), new Date(2026,9,23,21,0));
});
test('textoErroExecucao: erro do banco, motivo conhecido ou null quando deu certo', () => {
  const M = { sql:'Não consegui montar essa consulta.', vazio:'A consulta não trouxe nenhuma linha.', timeout_front:'Passou de 15 s.' };
  assert.equal(A.textoErroExecucao({ ok:true }, M), null);
  assert.equal(A.textoErroExecucao({ ok:false, motivo:'sql', erro:'relation "x" does not exist' }, M), 'relation "x" does not exist');
  assert.equal(A.textoErroExecucao({ ok:false, erro:'dial tcp: connection refused' }, M), 'dial tcp: connection refused');
  assert.equal(A.textoErroExecucao({ ok:false, motivo:'vazio' }, M), 'A consulta não trouxe nenhuma linha.');
  assert.equal(A.textoErroExecucao({ ok:false, motivo:'timeout_front' }, M), 'Passou de 15 s.');
  assert.equal(A.textoErroExecucao({ ok:false }, M), 'Não consegui montar essa consulta.');
  assert.equal(A.textoErroExecucao(null, M), 'Não consegui montar essa consulta.');
});
test('textoApagar: aviso do histórico só quando há execuções', () => {
  assert.equal(A.textoApagar(0), '');
  assert.equal(A.textoApagar(1), 'O histórico de 1 execução também será apagado.');
  assert.equal(A.textoApagar(812), 'O histórico de 812 execuções também será apagado.');
});
