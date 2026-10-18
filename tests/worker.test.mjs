import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { assinar, codigoValido } from '../worker/src/chave.js';
import { criar } from '../worker/src/index.js';

const v = JSON.parse(readFileSync(new URL('../instalacao/testdata/vetor.json', import.meta.url), 'utf8'));

test('assinar reproduz o vetor do Go', async () => {
  assert.equal(await assinar(v.codigo, v.privadaPKCS8Base64), v.chave);
});

test('codigoValido', () => {
  assert.ok(codigoValido('7F3K-Q9WD-2MXA'));
  assert.ok(!codigoValido('7F3K-Q9WD-2MXI')); // I não existe no Crockford
  assert.ok(!codigoValido('7F3KQ9WD2MXA'));
});

const BASE = 'https://instalariadebolso.cd2.com.br';
const SESSAO = 'cs_test_a1B2c3';

function montar(sessao, { falhaCriar = false } = {}) {
  const pedidos = [];
  const fetchFalso = async (url, init = {}) => {
    pedidos.push({ url: String(url), method: init.method || 'GET', body: init.body, headers: init.headers });
    if ((init.method || 'GET') === 'POST') {
      if (falhaCriar) return new Response('{"error":{"message":"segredo do stripe"}}', { status: 400 });
      return new Response(JSON.stringify({ id: SESSAO, url: 'https://checkout.stripe.com/c/pay/' + SESSAO }), { status: 200 });
    }
    if (sessao && String(url).endsWith('/' + SESSAO)) return new Response(JSON.stringify(sessao), { status: 200 });
    return new Response('{"error":{}}', { status: 404 });
  };
  const env = { STRIPE_SECRET_KEY: 'sk_test_x', STRIPE_PRICE_ID: 'price_1', CHAVE_PRIVADA: v.privadaPKCS8Base64, BASE_URL: BASE };
  return { app: criar({ fetch: fetchFalso }), pedidos, env };
}
const paga = (porta) => ({
  id: SESSAO, payment_status: 'paid', client_reference_id: v.codigo,
  metadata: porta === undefined ? {} : { porta: String(porta) },
});

test('comprarCriaCheckoutComCodigoEChave', async () => {
  const { app, pedidos, env } = montar();
  const r = await app.fetch(new Request(BASE + '/comprar?codigo=' + v.codigo + '&porta=8080'), env);
  assert.equal(r.status, 302);
  assert.equal(r.headers.get('location'), 'https://checkout.stripe.com/c/pay/' + SESSAO);
  assert.equal(pedidos[0].url, 'https://api.stripe.com/v1/checkout/sessions');
  assert.equal(pedidos[0].headers.Authorization, 'Bearer sk_test_x');
  const corpo = new URLSearchParams(pedidos[0].body);
  assert.equal(corpo.get('mode'), 'payment');
  assert.equal(corpo.get('line_items[0][price]'), 'price_1');
  assert.equal(corpo.get('line_items[0][quantity]'), '1');
  assert.equal(corpo.get('client_reference_id'), v.codigo);
  assert.equal(corpo.get('invoice_creation[invoice_data][custom_fields][0][value]'), v.chave);
  assert.equal(corpo.get('invoice_creation[invoice_data][custom_fields][0][name]'), 'Chave de instalação');
  assert.equal(corpo.get('invoice_creation[enabled]'), 'true');
  assert.equal(corpo.get('consent_collection[terms_of_service]'), 'required');
  assert.equal(corpo.get('locale'), 'pt-BR');
  assert.equal(corpo.get('metadata[porta]'), '8080');
  assert.equal(corpo.get('success_url'), BASE + '/voltou?session_id={CHECKOUT_SESSION_ID}');
  assert.equal(corpo.get('cancel_url'), BASE + '/cancelou');
  assert.equal(r.headers.get('cache-control'), 'no-store');
});

test('comprarRecusaCodigoInvalido', async () => {
  for (const q of ['codigo=7F3K-Q9WD-2MXI&porta=8080', 'porta=8080', 'codigo=' + v.codigo + '&porta=80',
    'codigo=' + v.codigo + '&porta=70000', 'codigo=' + v.codigo + '&porta=abc', 'codigo=' + v.codigo]) {
    const { app, pedidos, env } = montar();
    const r = await app.fetch(new Request(BASE + '/comprar?' + q), env);
    assert.equal(r.status, 400, q);
    assert.equal(pedidos.length, 0, q);
  }
});

test('comprarStripeComErroDa502SemVazar', async () => {
  const { app, env } = montar(null, { falhaCriar: true });
  const r = await app.fetch(new Request(BASE + '/comprar?codigo=' + v.codigo + '&porta=8080'), env);
  assert.equal(r.status, 502);
  const t = await r.text();
  assert.match(t, /Não consegui abrir o pagamento agora\. Tente de novo em alguns minutos\./);
  assert.ok(!t.includes('segredo do stripe'));
});

test('voltouPagaMostraChaveEVolta', async () => {
  const { app, pedidos, env } = montar(paga(8080));
  const r = await app.fetch(new Request(BASE + '/voltou?session_id=' + SESSAO), env);
  assert.equal(r.status, 200);
  const t = await r.text();
  assert.ok(t.includes(v.chave));
  assert.ok(t.includes('http://127.0.0.1:8080/?chave=' + encodeURIComponent(v.chave)));
  assert.match(t, /Voltar para a IA/);
  // Só o botão volta: nada de redirecionar sozinho, a página sempre fica com a chave.
  assert.ok(!t.includes('setTimeout'));
  assert.ok(!t.includes('location.href'));
  assert.ok(!/http-equiv="?refresh/i.test(t));
  assert.ok(!t.includes('3 segundos'));
  assert.equal(pedidos[0].method, 'GET');
  assert.equal(pedidos[0].url, 'https://api.stripe.com/v1/checkout/sessions/' + SESSAO);
  assert.equal(pedidos[0].headers.Authorization, 'Bearer sk_test_x');
  assert.equal(r.headers.get('cache-control'), 'no-store');
});

test('voltouMostraChaveMesmoSemPorta', async () => {
  const { app, env } = montar(paga());
  const r = await app.fetch(new Request(BASE + '/voltou?session_id=' + SESSAO), env);
  assert.equal(r.status, 200);
  const t = await r.text();
  assert.ok(t.includes(v.chave));
  assert.ok(!t.includes('127.0.0.1'));
});

test('voltouIgnoraPortaForaDaFaixa', async () => {
  const { app, env } = montar(paga(22));
  const t = await (await app.fetch(new Request(BASE + '/voltou?session_id=' + SESSAO), env)).text();
  assert.ok(t.includes(v.chave));
  assert.ok(!t.includes('127.0.0.1'));
});

test('voltouNaoPagaNaoMostraChave', async () => {
  const { app, env } = montar({ ...paga(8080), payment_status: 'unpaid' });
  const r = await app.fetch(new Request(BASE + '/voltou?session_id=' + SESSAO), env);
  assert.equal(r.status, 402);
  const t = await r.text();
  assert.match(t, /O pagamento ainda não foi confirmado/);
  assert.ok(!t.includes('CD2-'));
});

test('voltouSessaoDesconhecidaOuMalFormada', async () => {
  const { app, pedidos, env } = montar(null);
  let r = await app.fetch(new Request(BASE + '/voltou?session_id=cs_test_zzz'), env);
  assert.equal(r.status, 402);
  r = await app.fetch(new Request(BASE + '/voltou?session_id=../x'), env);
  assert.equal(r.status, 402);
  assert.equal(pedidos.length, 1); // a malformada nem chega ao Stripe
});

test('cancelou e rota desconhecida', async () => {
  const { app, env } = montar();
  const r = await app.fetch(new Request(BASE + '/cancelou'), env);
  assert.equal(r.status, 200);
  assert.match(await r.text(), /Tudo bem, a IA continua no pen drive\./);
  assert.equal(r.headers.get('cache-control'), 'no-store');
  assert.equal((await app.fetch(new Request(BASE + '/outra'), env)).status, 404);
});
