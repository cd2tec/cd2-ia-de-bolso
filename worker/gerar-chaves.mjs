// Gera o par Ed25519 de produção da instalação paga. Rode UMA vez:
//   node worker/gerar-chaves.mjs
// Nada é gravado em disco; copie a saída na hora.

const par = await crypto.subtle.generateKey({ name: 'Ed25519' }, true, ['sign', 'verify']);
const privada = Buffer.from(await crypto.subtle.exportKey('pkcs8', par.privateKey)).toString('base64');
const publica = Buffer.from(await crypto.subtle.exportKey('raw', par.publicKey)).toString('base64');

console.log('CHAVE PRIVADA (PKCS8, base64). Guarde fora do repositório e entregue ao Worker:');
console.log('  ' + privada);
console.log('  Comando: npx wrangler secret put CHAVE_PRIVADA   (cole o valor acima quando pedir)');
console.log('');
console.log('CHAVE PÚBLICA (base64). Cole em instalacao/chave/publica.go, com um comentário sobre a origem:');
console.log('  ' + publica);
console.log('');
console.log('Atenção: trocar o par invalida todas as chaves já emitidas. Não rode de novo sem necessidade.');
