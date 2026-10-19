'use strict';
// Download fácil: o ZIP cd2tech-iadebolso.zip sai de uma release fixa, atualizada a cada push no main.
// O README aponta para esse link, nunca para o ZIP automático do GitHub. O site é para quem é do
// negócio: oferece só os instaladores, sem o ZIP técnico. Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs'), path = require('node:path');
const ler = f => fs.readFileSync(path.join(__dirname, '..', f), 'utf8');
const LINK = 'https://github.com/cd2tec/cd2-ia-de-bolso/releases/download/iadebolso/cd2tech-iadebolso.zip';

test('README usa o link do cd2tech-iadebolso.zip; nem README nem site usam o ZIP automático', () => {
  assert.ok(ler('README.md').includes(LINK));
  for(const f of ['README.md', 'docs/index.html']) assert.doesNotMatch(ler(f), /archive\/refs\/heads\/main\.zip/, f);
});

test('workflow monta o ZIP com a pasta cd2tech-iadebolso e publica na release iadebolso', () => {
  const w = ler('.github/workflows/zip-iadebolso.yml');
  assert.match(w, /branches: \[main\]/);
  assert.match(w, /--prefix=cd2tech-iadebolso\//);
  assert.match(w, /-o cd2tech-iadebolso\.zip/);
  assert.match(w, /gh release upload iadebolso cd2tech-iadebolso\.zip --clobber/);
  assert.match(w, /--latest=false/);
});
