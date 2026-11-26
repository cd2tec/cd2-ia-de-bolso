'use strict';
// Compara duas rodadas da bateria (H6-CT02): caso a caso, o que mudou de uma para a outra.
//   node tests/eval/comparar-rodadas.js rodada-1.json rodada-2.json
const fs = require('fs');
const M = require('./metricas.js');
const [a, b] = process.argv.slice(2).map(f => JSON.parse(fs.readFileSync(f, 'utf8')));
if(!a || !b){ console.error('uso: node tests/eval/comparar-rodadas.js a.json b.json'); process.exit(2); }
const c = M.comparar(a.itens, b.itens);
console.log(a.ambiente.rotulo + ' x ' + b.ambiente.rotulo + ': ' + c.iguais + ' de ' + c.total + ' iguais, ' + c.diferentes + ' diferentes');
c.difs.forEach(d => console.log('  ' + d.id + ' ' + d.etapa + ': ' + d.mudou.join('; ')));
for(const k of ['escolha', 'resumoTodos']){
  const x = a.metricas[k], y = b.metricas[k];
  console.log(k + ': acertos/aprovados ' + (x.acertos != null ? x.acertos + ' x ' + y.acertos : x.aprovados + ' x ' + y.aprovados)
    + ' · latência média ' + x.latencia.media + ' x ' + y.latencia.media + ' ms');
}
