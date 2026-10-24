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
