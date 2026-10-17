'use strict';
// Regra dos lancadores na gravacao (core/lancadores.sh) e o INICIAR do Mac rodando de core/ ou da raiz.
// Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const { execFileSync } = require('node:child_process');
const fs = require('node:fs'), os = require('node:os'), path = require('node:path');

const ROOT = path.join(__dirname, '..');
const tmp = () => fs.mkdtempSync(path.join(os.tmpdir(), 'cd2-lanc-'));
const touch = (base, rel, conteudo) => {
  const p = path.join(base, rel);
  fs.mkdirSync(path.dirname(p), { recursive: true });
  fs.writeFileSync(p, conteudo === undefined ? rel : conteudo);
};
const existe = (base, rel) => fs.existsSync(path.join(base, rel));
const arrumar = stage => execFileSync('sh', ['-c', '. "$ROOT/core/lancadores.sh"; arrumar_lancadores "$STAGE"'],
  { env: { PATH: process.env.PATH, ROOT, STAGE: stage }, encoding: 'utf8' });

test('lancadores.sh: carregar nao faz nada', () => {
  const out = execFileSync('sh', ['-c', '. "$ROOT/core/lancadores.sh"; echo ok'], { env: { PATH: process.env.PATH, ROOT }, encoding: 'utf8' });
  assert.equal(out, 'ok\n');
});

test('lancadores.sh com os dois lançadores', () => {
  const s = tmp();
  for (const f of ['IA de Bolso.exe', 'IA de Bolso.app/Contents/MacOS/IA de Bolso', 'INICIAR-CD2-Windows.bat',
    'INICIAR-CD2-Mac.command', 'iniciar-cd2-linux.sh', 'core/start-unix.sh']) touch(s, f);
  arrumar(s);
  // Ate o teste no Windows, o .bat fica na raiz ao lado do .exe (alternativa).
  assert.ok(existe(s, 'INICIAR-CD2-Windows.bat'));
  assert.ok(!existe(s, 'core/INICIAR-CD2-Windows.bat'));
  assert.ok(existe(s, 'core/INICIAR-CD2-Mac.command'));
  assert.ok(!existe(s, 'INICIAR-CD2-Mac.command'));
  assert.ok(existe(s, 'iniciar-cd2-linux.sh'));
  assert.ok(existe(s, 'IA de Bolso.exe'));
});

test('lancadores.sh sem lançador, nada muda', () => {
  const s = tmp();
  for (const f of ['INICIAR-CD2-Windows.bat', 'INICIAR-CD2-Mac.command', 'iniciar-cd2-linux.sh']) touch(s, f);
  arrumar(s);
  for (const f of ['INICIAR-CD2-Windows.bat', 'INICIAR-CD2-Mac.command', 'iniciar-cd2-linux.sh']) assert.ok(existe(s, f), f);
  assert.ok(!existe(s, 'core'));
});

test('lancadores.sh so o .app: o .command vai, o .bat fica', () => {
  const s = tmp();
  for (const f of ['IA de Bolso.app/Contents/MacOS/IA de Bolso', 'INICIAR-CD2-Windows.bat', 'INICIAR-CD2-Mac.command']) touch(s, f);
  arrumar(s);
  assert.ok(existe(s, 'core/INICIAR-CD2-Mac.command') && !existe(s, 'INICIAR-CD2-Mac.command'));
  assert.ok(existe(s, 'INICIAR-CD2-Windows.bat') && !existe(s, 'core/INICIAR-CD2-Windows.bat'));
});

test('lancadores.sh so o .exe: o .bat fica na raiz', () => {
  const s = tmp();
  for (const f of ['IA de Bolso.exe', 'INICIAR-CD2-Windows.bat']) touch(s, f);
  arrumar(s);
  assert.ok(existe(s, 'INICIAR-CD2-Windows.bat') && !existe(s, 'core/INICIAR-CD2-Windows.bat'));
});

test('lancadores.sh sobrescreve o de core/ e roda duas vezes sem erro', () => {
  const s = tmp();
  for (const f of ['IA de Bolso.app/Contents/MacOS/IA de Bolso']) touch(s, f);
  touch(s, 'INICIAR-CD2-Mac.command', 'novo');
  touch(s, 'core/INICIAR-CD2-Mac.command', 'velho');
  arrumar(s); arrumar(s);
  assert.equal(fs.readFileSync(path.join(s, 'core/INICIAR-CD2-Mac.command'), 'utf8'), 'novo');
  assert.ok(!existe(s, 'INICIAR-CD2-Mac.command'));
});

test('INICIAR-CD2-Mac.command roda de core/ e da raiz', () => {
  // Copia o .command e um start-unix.sh falso que grava o proprio caminho e o diretorio de onde rodou.
  const roda = (stage, onde) => {
    const cmd = path.join(stage, onde, 'INICIAR-CD2-Mac.command');
    return execFileSync('sh', [cmd], { encoding: 'utf8', env: { PATH: process.env.PATH } });
  };
  const monta = () => {
    const s = tmp();
    touch(s, 'core/start-unix.sh', 'echo "start:$0"\n');
    return s;
  };
  const orig = fs.readFileSync(path.join(ROOT, 'INICIAR-CD2-Mac.command'), 'utf8');

  const raiz = monta();
  touch(raiz, 'INICIAR-CD2-Mac.command', orig);
  assert.equal(roda(raiz, '.').trim(), 'start:core/start-unix.sh');

  const emCore = monta();
  touch(emCore, 'core/INICIAR-CD2-Mac.command', orig);
  assert.equal(roda(emCore, 'core').trim(), 'start:core/start-unix.sh');
});

test('INICIAR-CD2-Windows.bat acha o start-windows.ps1 em core\\ ou ao lado', () => {
  const bat = fs.readFileSync(path.join(ROOT, 'INICIAR-CD2-Windows.bat'), 'utf8');
  assert.match(bat, /%~dp0core\\start-windows\.ps1/);
  assert.match(bat, /if not exist "%CD2_PS1%" set "CD2_PS1=%~dp0start-windows\.ps1"/);
});
