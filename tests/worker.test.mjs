import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { assinar, codigoValido } from '../worker/src/chave.js';

const v = JSON.parse(readFileSync(new URL('../instalacao/testdata/vetor.json', import.meta.url), 'utf8'));

test('assinar reproduz o vetor do Go', async () => {
  assert.equal(await assinar(v.codigo, v.privadaPKCS8Base64), v.chave);
});

test('codigoValido', () => {
  assert.ok(codigoValido('7F3K-Q9WD-2MXA'));
  assert.ok(!codigoValido('7F3K-Q9WD-2MXI')); // I não existe no Crockford
  assert.ok(!codigoValido('7F3KQ9WD2MXA'));
});
