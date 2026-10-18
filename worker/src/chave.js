// Assinatura da chave de instalação. Reproduz byte a byte o formato do lado Go
// (instalacao/chave): mensagem "cd2-instalacao-v1|<codigo>", assinatura Ed25519
// de 64 bytes em base32 Crockford maiúsculo, sem padding, prefixo "CD2-".

const ALFABETO = '0123456789ABCDEFGHJKMNPQRSTVWXYZ';
const FORMATO_CODIGO = /^[0-9A-HJKMNP-TV-Z]{4}-[0-9A-HJKMNP-TV-Z]{4}-[0-9A-HJKMNP-TV-Z]{4}$/;

export function codigoValido(s) {
  return typeof s === 'string' && FORMATO_CODIGO.test(s);
}

function base32(bytes) {
  let saida = '';
  let acc = 0;
  let bits = 0;
  for (const b of bytes) {
    acc = (acc << 8) | b;
    bits += 8;
    while (bits >= 5) {
      saida += ALFABETO[(acc >> (bits - 5)) & 31];
      bits -= 5;
    }
    acc &= (1 << bits) - 1;
  }
  if (bits > 0) saida += ALFABETO[(acc << (5 - bits)) & 31];
  return saida;
}

function deBase64(s) {
  const bin = atob(s);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
}

export async function assinar(codigo, privadaPKCS8Base64) {
  const chave = await crypto.subtle.importKey(
    'pkcs8', deBase64(privadaPKCS8Base64), { name: 'Ed25519' }, false, ['sign']);
  const msg = new TextEncoder().encode('cd2-instalacao-v1|' + codigo);
  const sig = new Uint8Array(await crypto.subtle.sign({ name: 'Ed25519' }, chave, msg));
  return 'CD2-' + base32(sig);
}
