/* As frases da janelinha: as de quem foi chamada de volta e as da despedida. As do carregamento
   ficam em mascote.js (CD2Mascote.PIADAS), iguais às do instalador. */
(function(root){
"use strict";

const CHAMADA = [
  'Tava aqui no cantinho do relógio, contando os segundos. Literalmente.',
  'Eu não dormi, só fechei os olhos para pensar.',
  'Já estava com saudade. Pode falar.',
  'Fiquei quietinha aqui, igual aluno de última fila.',
  'Dei uma cochilada de 4 bilhões de parâmetros.',
  'Chamou e eu vim. Sem notificação, sem reunião.'
];

const DESPEDIDA = [
  'Vou sonhar com planilhas. Até a próxima.',
  'Tchau! Eu fico de olho no pen drive por você.',
  'Boa noite. Prometo não apagar nada enquanto durmo.',
  'Desligando os neurônios, um por um.'
];

// Índice de uma frase diferente da última (ultima = -1 na primeira vez).
function proxima(lista, ultima, sorteio){
  const r = sorteio || Math.random;
  let i;
  do { i = Math.floor(r() * lista.length); } while(lista.length > 1 && i === ultima);
  return i;
}

const api = { CHAMADA, DESPEDIDA, proxima };
if(typeof module !== 'undefined' && module.exports) module.exports = api; else root.CD2Piadas = api;
})(typeof window !== 'undefined' ? window : globalThis);
