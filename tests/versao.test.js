'use strict';
// A versão anda junto em todo lugar: o navegador guarda o front em cache (o llamafile não manda
// Cache-Control), então cada script leva ?v=<versão> e o launcher abre /?v=<versão>. Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs'), path = require('node:path');
const raiz = path.join(__dirname, '..'), ler = f => fs.readFileSync(path.join(raiz, f), 'utf8');
const V = ler('VERSION').trim();

test('index.html: todo script local leva ?v= com a versão de VERSION', () => {
  const srcs = [...ler('web/index.html').matchAll(/<script src="([^"]+)"/g)].map(m => m[1]);
  assert.ok(srcs.length >= 9, srcs.join(', '));
  assert.ok(srcs.some(s => s.startsWith('instalar.js')), 'instalar.js precisa estar no index.html');
  for(const s of srcs) assert.equal(s.split('?v=')[1], V, s);
});

test('mcp.js anuncia a mesma versão', () => {
  assert.match(ler('web/mcp.js'), new RegExp("version:'" + V.replace(/\./g, '\\.') + "'"));
});

test('launchers abrem a janela com ?v= lido de VERSION', () => {
  assert.match(ler('core/start-unix.sh'), /APP_URL="\$URL\/\?v=\$VER#k=\$CTL_KEY"/);
  assert.match(ler('core/start-windows.ps1'), /\$AppUrl = "\$Url\/\?v=\$Ver#k=\$CtlKey"/);
});
