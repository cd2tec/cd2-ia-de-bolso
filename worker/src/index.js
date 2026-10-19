import { assinar, codigoValido } from './chave.js';
import { paginaCancelou, paginaChave, paginaErro } from './paginas.js';

const STRIPE = 'https://api.stripe.com/v1/checkout/sessions';
const SESSAO_VALIDA = /^cs_(test|live)_[A-Za-z0-9]+$/;

function portaValida(s) {
  if (typeof s !== 'string' || !/^\d{1,5}$/.test(s)) return false;
  const n = Number(s);
  return n >= 1024 && n <= 65535;
}

function html(corpo, status = 200) {
  return new Response(corpo, {
    status,
    headers: { 'Content-Type': 'text/html; charset=utf-8', 'Cache-Control': 'no-store' },
  });
}

export function criar({ fetch: buscar }) {
  async function comprar(url, env) {
    const codigo = url.searchParams.get('codigo');
    const porta = url.searchParams.get('porta');
    if (!codigoValido(codigo) || !portaValida(porta)) {
      return html(paginaErro('Link inválido', 'Este link de compra não está completo. Volte para a IA de Bolso ou para o instalador e tente de novo.'), 400);
    }
    const chave = await assinar(codigo, env.CHAVE_PRIVADA);
    const doInstalador = url.searchParams.get('de') === 'instalador';
    const corpo = new URLSearchParams({
      mode: 'payment',
      'line_items[0][price]': env.STRIPE_PRICE_ID,
      'line_items[0][quantity]': '1',
      client_reference_id: codigo,
      'metadata[porta]': porta,
      success_url: env.BASE_URL + '/voltou?session_id={CHECKOUT_SESSION_ID}',
      cancel_url: env.BASE_URL + (doInstalador ? '/cancelou?de=instalador' : '/cancelou'),
      locale: 'pt-BR',
      'invoice_creation[enabled]': 'true',
      'invoice_creation[invoice_data][custom_fields][0][name]': 'Chave de instalação',
      'invoice_creation[invoice_data][custom_fields][0][value]': chave,
      'consent_collection[terms_of_service]': 'required',
    });
    if (doInstalador) corpo.set('metadata[de]', 'instalador');
    const corpoTexto = corpo.toString();
    const falha = () => html(paginaErro('Pagamento indisponível', 'Não consegui abrir o pagamento agora. Tente de novo em alguns minutos.'), 502);
    let resp;
    try {
      resp = await buscar(STRIPE, {
        method: 'POST',
        headers: { Authorization: 'Bearer ' + env.STRIPE_SECRET_KEY, 'Content-Type': 'application/x-www-form-urlencoded' },
        body: corpoTexto,
      });
    } catch {
      return falha();
    }
    if (!resp.ok) {
      // Só para o log do Worker (wrangler tail / painel): o motivo do Stripe, sem chaves.
      try {
        const e = ((await resp.json()) || {}).error || {};
        console.error('stripe recusou o checkout', resp.status, e.type, e.code, e.param, e.message);
      } catch {
        console.error('stripe recusou o checkout', resp.status);
      }
      return falha();
    }
    let sessao;
    try { sessao = await resp.json(); } catch { return falha(); }
    if (!sessao || typeof sessao.url !== 'string' || !sessao.url.startsWith('https://')) return falha();
    return new Response(null, { status: 302, headers: { Location: sessao.url, 'Cache-Control': 'no-store' } });
  }

  async function voltou(url, env) {
    const naoConfirmado = () => html(paginaErro('Pagamento pendente', 'O pagamento ainda não foi confirmado. Se você acabou de pagar, espere alguns segundos e recarregue esta página.'), 402);
    const id = url.searchParams.get('session_id') || '';
    if (!SESSAO_VALIDA.test(id)) return naoConfirmado();
    let sessao;
    try {
      const resp = await buscar(STRIPE + '/' + id, { headers: { Authorization: 'Bearer ' + env.STRIPE_SECRET_KEY } });
      if (!resp.ok) return naoConfirmado();
      sessao = await resp.json();
    } catch {
      return naoConfirmado();
    }
    const codigo = sessao && sessao.client_reference_id;
    if (!sessao || sessao.payment_status !== 'paid' || !codigoValido(codigo)) return naoConfirmado();
    const porta = sessao.metadata && sessao.metadata.porta;
    const chave = await assinar(codigo, env.CHAVE_PRIVADA);
    return html(paginaChave(chave, portaValida(porta) ? porta : null, sessao.metadata && sessao.metadata.de === 'instalador'));
  }

  return {
    async fetch(req, env) {
      const url = new URL(req.url);
      if (req.method !== 'GET' && req.method !== 'HEAD') return html(paginaErro('Não encontrado', 'Esta página não existe.'), 405);
      switch (url.pathname) {
        case '/comprar': return comprar(url, env);
        case '/voltou': return voltou(url, env);
        case '/cancelou': return html(paginaCancelou(url.searchParams.get('de') === 'instalador'));
        default: return html(paginaErro('Não encontrado', 'Esta página não existe.'), 404);
      }
    },
  };
}

export default criar({ fetch: (...args) => fetch(...args) });
