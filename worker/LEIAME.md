# Worker da instalação paga

Cloudflare Worker que abre o Checkout do Stripe e assina a chave de instalação.
Rotas: `/comprar?codigo=...&porta=...`, `/voltou?session_id=...` e `/cancelou`.

## Passo a passo (CD2)

1. No Stripe, em modo de teste, crie o produto da instalação com o preço de R$ 25 (pagamento único). Guarde o ID do preço (`price_...`).
2. No painel do Stripe, cadastre a URL das condições da cópia, `https://cd2tec.github.io/cd2-ia-de-bolso/condicoes-instalacao.html` (Configurações, Empresa, Informações públicas). O Checkout exige isso por causa de `consent_collection[terms_of_service]`.
3. Gere o par de chaves de assinatura, uma única vez: `node worker/gerar-chaves.mjs`. Guarde a privada fora do repositório e cole a pública em `instalacao/chave/publica.go`.
4. Cadastre os três segredos, dentro de `worker/`:
   - `wrangler secret put STRIPE_SECRET_KEY` (chave secreta do Stripe, `sk_test_...` no teste)
   - `wrangler secret put STRIPE_PRICE_ID` (o `price_...` do passo 1)
   - `wrangler secret put CHAVE_PRIVADA` (a privada do passo 3)
5. Publique: `wrangler deploy`.
6. Teste de ponta a ponta no modo de teste: abra `/comprar?codigo=7F3K-Q9WD-2MXA&porta=8080`, pague com o cartão `4242 4242 4242 4242` e confira a página da chave, o e-mail de teste e a fatura com o campo "Chave de instalação".

Para ir ao ar de verdade, troque os segredos do Stripe pelos do modo real e crie o preço também no modo real.

## Worker de teste (sandbox)

Para testar sem cobrar de verdade há um segundo Worker, `cd2-instalacao-sandbox`, em `https://instalariadebolso-teste.cd2.com.br` (`[env.sandbox]` no `wrangler.toml`).

- Publicar: `npx wrangler@4 deploy --env sandbox`.
- Segredos, todos com `--env sandbox`: `STRIPE_SECRET_KEY` e `STRIPE_PRICE_ID` da sandbox do Stripe, e `CHAVE_PRIVADA` com a chave de TESTE (`instalacao/testdata/vetor.json`), nunca a de produção.
- O pen drive de teste usa o build com `-tags sandbox`: `CD2_GO_TAGS=sandbox sh ctl/build.sh "..."` e `wails build -tags sandbox ...`. Esse build compra no Worker de teste, confere com a chave de teste e mostra "modo de teste" no modal.
- Uma chave da sandbox não abre o build oficial, e uma de produção não abre o de teste.
- O Worker de produção só deve ter as chaves do Stripe no modo real. Com chaves da sandbox nele, um pagamento de mentira geraria uma chave de produção válida.
