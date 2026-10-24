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
