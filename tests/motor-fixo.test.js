'use strict';
// Motor (llamafile) com versao e SHA-256 fixos no core/prepare-unix.sh, como no manifesto do instalador.
// Antes o script pegava a ultima release na hora de gravar, e com um bin/llamafile de outra versao o
// "curl -C -" emendava o arquivo novo no velho: motor corrompido, sem conferencia nenhuma. Rodar: node --test
const test = require('node:test');
const assert = require('node:assert/strict');
const { execFile } = require('node:child_process');
const crypto = require('node:crypto'), http = require('node:http');
const fs = require('node:fs'), os = require('node:os'), path = require('node:path');

const ler = f => fs.readFileSync(path.join(__dirname, '..', f), 'utf8');
const sh = ler('core/prepare-unix.sh'), ps1 = ler('core/prepare-windows.ps1');
const funcao = nome => { const i = sh.indexOf(nome + '() {'); assert.ok(i >= 0, 'sem ' + nome); return sh.slice(i, sh.indexOf('\n}\n', i) + 3); };
const sha = b => crypto.createHash('sha256').update(b).digest('hex');

// Servidor com Range, para o curl -C - poder retomar. /v1 e /v2 sao duas "versoes" do motor.
const versoes = { '/v1': Buffer.alloc(4096, 'a'), '/v2': Buffer.concat([Buffer.alloc(3000, 'b'), Buffer.alloc(3000, 'c')]) };
function servir(){
  const s = http.createServer((req, res) => {
    const corpo = versoes[req.url];
    if(!corpo){ res.writeHead(404); return res.end(); }
    const m = /bytes=(\d+)-/.exec(req.headers.range || '');
    const ini = m ? Number(m[1]) : 0;
    if(ini >= corpo.length){ res.writeHead(416, { 'Content-Range': 'bytes */' + corpo.length }); return res.end(); }
    res.writeHead(m ? 206 : 200, { 'Content-Length': corpo.length - ini, 'Accept-Ranges': 'bytes' });
    res.end(req.method === 'HEAD' ? undefined : corpo.subarray(ini));
  });
  return new Promise(r => s.listen(0, '127.0.0.1', () => r(s)));
}
function motor(raiz, url, soma){
  const script = funcao('remote_size') + funcao('baixar_motor') +
    'SUM="shasum -a 256"; ROOT="$1"; LLAMAFILE_URL="$2"; LLAMAFILE_SHA256="$3"; mkdir -p "$ROOT/bin"; baixar_motor';
  return new Promise(r => execFile('sh', ['-c', script, 'sh', raiz, url, soma], { timeout: 20000 }, (err, out, errOut) => r({ ok: !err, out: out + errOut })));
}
const pasta = () => fs.mkdtempSync(path.join(os.tmpdir(), 'cd2-motor-'));
const arq = (raiz, f) => path.join(raiz, 'bin', f);

test('os dois prepare fixam a mesma versao e o mesmo SHA do llamafile, sem consultar a ultima release', () => {
  const url = /^LLAMAFILE_URL="\$\{CD2_LLAMAFILE_URL:-([^}]+)\}"$/m.exec(sh), soma = /^else LLAMAFILE_SHA256="\$\{CD2_LLAMAFILE_SHA256:-([0-9a-f]{64})\}"/m.exec(sh);
  assert.ok(url && soma, 'prepare-unix.sh sem LLAMAFILE_URL/LLAMAFILE_SHA256 fixos');
  assert.match(url[1], /\/releases\/download\/[0-9.]+\/llamafile-[0-9.]+$/);
  assert.ok(ps1.includes("'" + url[1] + "'"), 'prepare-windows.ps1 com outra URL do llamafile');
  assert.ok(ps1.includes("'" + soma[1] + "'"), 'prepare-windows.ps1 com outro SHA do llamafile');
  for(const s of [sh, ps1]) assert.doesNotMatch(s, /releases\/latest/);
});

test('motor de outra versao em bin/ e trocado, sem emendar, e confere o SHA', async () => {
  const s = await servir(), base = 'http://127.0.0.1:' + s.address().port, raiz = pasta();
  try{
    fs.mkdirSync(path.join(raiz, 'bin'));
    fs.writeFileSync(arq(raiz, 'llamafile'), versoes['/v1']);
    const r = await motor(raiz, base + '/v2', sha(versoes['/v2']));
    assert.ok(r.ok, r.out);
    assert.equal(sha(fs.readFileSync(arq(raiz, 'llamafile'))), sha(versoes['/v2']));
    assert.deepEqual(fs.readdirSync(path.join(raiz, 'bin')).sort(), ['llamafile', 'llamafile.exe']);
  }finally{ s.close(); }
});

test('pedaco baixado de outra versao nao e retomado', async () => {
  const s = await servir(), base = 'http://127.0.0.1:' + s.address().port, raiz = pasta();
  try{
    let r = await motor(raiz, base + '/v1', sha(versoes['/v1']));
    assert.ok(r.ok, r.out);
    // sobra um pedaco da v1 com o nome de pedaco que a v1 usaria; a v2 precisa ignorar
    const pedacos = () => fs.readdirSync(path.join(raiz, 'bin')).filter(f => f.startsWith('llamafile.part'));
    fs.writeFileSync(path.join(raiz, 'bin', 'llamafile.part-velho'), versoes['/v1'].subarray(0, 1000));
    fs.rmSync(arq(raiz, 'llamafile'));
    r = await motor(raiz, base + '/v2', sha(versoes['/v2']));
    assert.ok(r.ok, r.out);
    assert.equal(sha(fs.readFileSync(arq(raiz, 'llamafile'))), sha(versoes['/v2']));
    assert.deepEqual(pedacos(), []);
  }finally{ s.close(); }
});

test('pedaco da mesma versao e retomado e conferido', async () => {
  const s = await servir(), base = 'http://127.0.0.1:' + s.address().port, raiz = pasta(), url = base + '/v2';
  const ranges = []; s.on('request', req => { if(req.headers.range) ranges.push(req.headers.range); });
  try{
    fs.mkdirSync(path.join(raiz, 'bin'));
    // o nome do pedaco leva os 12 primeiros hex do SHA-256 da URL: so a mesma origem o reaproveita
    const pedaco = arq(raiz, 'llamafile.part-' + sha(url).slice(0, 12));
    fs.writeFileSync(pedaco, versoes['/v2'].subarray(0, 2500));
    const r = await motor(raiz, url, sha(versoes['/v2']));
    assert.ok(r.ok, r.out);
    assert.deepEqual(ranges, ['bytes=2500-']);
    assert.equal(sha(fs.readFileSync(arq(raiz, 'llamafile'))), sha(versoes['/v2']));
    assert.equal(fs.existsSync(pedaco), false);
  }finally{ s.close(); }
});

test('SHA diferente do fixo falha e nao deixa motor nem pedaco', async () => {
  const s = await servir(), base = 'http://127.0.0.1:' + s.address().port, raiz = pasta();
  try{
    const r = await motor(raiz, base + '/v2', sha(versoes['/v1']));
    assert.equal(r.ok, false);
    assert.match(r.out, /SHA-256 do motor nao confere/);
    assert.deepEqual(fs.readdirSync(path.join(raiz, 'bin')), []);
  }finally{ s.close(); }
});

test('motor ja conferido nao baixa de novo', async () => {
  const s = await servir(), base = 'http://127.0.0.1:' + s.address().port, raiz = pasta();
  try{
    fs.mkdirSync(path.join(raiz, 'bin'));
    fs.writeFileSync(arq(raiz, 'llamafile'), versoes['/v2']);
    s.close(); // sem servidor: se tentar baixar, falha
    const r = await motor(raiz, base + '/v2', sha(versoes['/v2']));
    assert.ok(r.ok, r.out);
    assert.match(r.out, /Ja conferido, pulando: bin\/llamafile/);
  }finally{ s.close(); }
});
