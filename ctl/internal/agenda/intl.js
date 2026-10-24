/* Intl mínimo para o goja, que não tem Intl: só o que o chart.js usa,
   new Intl.NumberFormat('pt-BR', { maximumFractionDigits }).format(n), com o mesmo texto do Node
   (milhar '.', decimal ',', arredonda a metade para longe do zero sobre a forma decimal curta). */
(function(g){
"use strict";
if(typeof g.Intl !== 'undefined') return;

// Dígitos e posição da vírgula de |x| a partir de String(x), que já é a forma decimal mais curta.
function decimal(a){
  const m = /^(\d+)(?:\.(\d+))?(?:e([+-]\d+))?$/.exec(String(a));
  const frac = m[2] || '';
  return { digitos:m[1] + frac, ponto:m[1].length + (m[3] ? +m[3] : 0) };
}
function arredonda(d, casas){
  const corte = d.ponto + casas;
  if(corte < 0) return { digitos:'0', ponto:1 };
  if(corte >= d.digitos.length) return d;
  const sobe = d.digitos.charCodeAt(corte) >= 53; // '5'
  const ds = d.digitos.slice(0, corte).split('').map(Number);
  let ponto = d.ponto;
  if(sobe){
    let i = ds.length - 1;
    while(i >= 0 && ds[i] === 9){ ds[i] = 0; i--; }
    if(i >= 0) ds[i]++; else { ds.unshift(1); ponto++; }
  }
  return { digitos:ds.join('') || '0', ponto };
}
function NumberFormat(locale, opcoes){
  if(!(this instanceof NumberFormat)) return new NumberFormat(locale, opcoes);
  if(locale !== 'pt-BR') throw new RangeError('Intl: só pt-BR');
  const md = opcoes && opcoes.maximumFractionDigits;
  this.casas = md == null ? 3 : md;
}
NumberFormat.prototype.format = function(n){
  const x = Number(n);
  if(x !== x) return 'NaN';
  const neg = x < 0 || (x === 0 && 1 / x < 0);
  if(!isFinite(x)) return (neg ? '-' : '') + '∞';
  const d = arredonda(decimal(Math.abs(x)), this.casas);
  let inteiro, frac;
  if(d.ponto <= 0){ inteiro = '0'; frac = '0'.repeat(-d.ponto) + d.digitos; }
  else if(d.ponto >= d.digitos.length){ inteiro = d.digitos + '0'.repeat(d.ponto - d.digitos.length); frac = ''; }
  else { inteiro = d.digitos.slice(0, d.ponto); frac = d.digitos.slice(d.ponto); }
  inteiro = inteiro.replace(/^0+(?=\d)/, '');
  frac = frac.replace(/0+$/, '');
  inteiro = inteiro.replace(/\B(?=(\d{3})+$)/g, '.');
  return (neg ? '-' : '') + inteiro + (frac ? ',' + frac : '');
};
g.Intl = { NumberFormat };
})(globalThis);
