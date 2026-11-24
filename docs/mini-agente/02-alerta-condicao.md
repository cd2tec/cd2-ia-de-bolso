# Mini agente · História 2 (CIDB-6): alerta por condição no agendamento

Como a condição do agendamento funciona hoje, regra por regra. A avaliação é só código
(`web/condicao.js`, `julgar`), rodando no cd2-ctl pelo motor goja; o modelo não entra. Os itens marcados
**(a confirmar)** são os que a história deixa para o produto confirmar: aqui está o comportamento
implementado, para o produto aprovar ou pedir mudança.

## Formulário (janela Agendar, "Quando avisar")

| Campo | Opções |
|---|---|
| Quando avisar | **Toda vez que rodar** (sem condição: igual a antes) ou **Só quando…** |
| Como juntar as linhas | **o valor de** (uma linha), **a soma de**, **a média de**, **o maior valor de**, **o menor valor de** |
| Coluna | lista das colunas numéricas do último resultado da consulta (a coluna do gráfico primeiro); sem resultado para ler, campo de texto |
| Operador | **for maior que**, **for menor que** (contra valor fixo); **subir mais de**, **cair mais de**, **variar mais de** (em %) |
| Valor | número; nos operadores de variação, percentual a partir de 0 |
| Base (só variação) | **execução anterior** ou **média das últimas N** (N de 2 a 30) |

A janela mostra o "Último resultado" do número que a regra conferiria e o resumo da regra ("Avisa só
quando a soma de faturamento for maior que 1.000.").

## Regras de avaliação

| Situação | Grava no histórico | Notifica e marca novidade | Texto gravado |
|---|---|---|---|
| Sem condição | sim | **sim, como antes** | (o de sempre) |
| Condição bate | sim | **sim** | "A soma de faturamento = 2.874.401,72, acima de 1." |
| Condição não bate | sim | **não** | "… não passou de 999.999.999." |
| Igual ao limite | sim | não (o limite é estrito: maior que, menor que) | "… não passou de …" |
| **Resultado vazio** (a confirmar) | sim | **não** | "A consulta não trouxe linhas. A condição não foi conferida." |
| **Sem execução anterior** para a variação ou média (a confirmar) | sim | **não** | "… Ainda não há execução anterior para comparar." |
| Erro do banco ou tempo esgotado | sim | **sim** (o alerta não pode parar calado) | "A consulta não rodou. A condição não foi conferida." |
| Coluna da regra não veio no resultado, ou sem número | sim | **sim** (para a pessoa arrumar a regra) | "A coluna da condição não veio no resultado. Confira a condição do agendamento." |
| Condição inválida (gravada errada) | sim | sim | "A condição do agendamento é inválida." |

"Rodar agora" (manual) grava no histórico e nunca notifica, como antes.

## Várias linhas (a confirmar)

- Uma linha: vale o número da coluna escolhida ("o valor de").
- Várias linhas: a pessoa escolhe **soma, média, maior ou menor** da coluna. A janela começa em **soma**.
- Valores nulos ou sem número na coluna ficam de fora da conta.
- Número vindo como texto do banco ("1234.50") conta como número; o nome da coluna não diferencia maiúsculas.

## Variação e base de comparação

- Variação em % = (valor atual − base) ÷ |base| × 100. "Subir mais de X%": variação > X; "cair mais de X%":
  −variação > X; "variar mais de X%": |variação| > X.
- **Execução anterior**: a última execução **agendada que deu certo** (manual, perda e erro não contam).
- **Média das últimas N**: média das N execuções agendadas mais novas que deram certo; com menos de N,
  usa as que houver (a partir de 1).
- Base zero: qualquer valor diferente de zero conta como variação (subiu ou caiu de 0); zero contra zero é
  "ficou igual".

## Onde está no código

| O quê | Arquivo |
|---|---|
| Regras (validar, julgar, textos) | `web/condicao.js` |
| Testes das regras | `tests/condicao.test.js` |
| Agendador: julgar antes de gravar, notificar só com `aviso` | `ctl/internal/agenda/agendador.go` (`julgar`, `rodar`) |
| Base: execuções anteriores agendadas que deram certo | `ctl/internal/agenda/historico.go` (`Anteriores`) |
| Testes no agendador | `ctl/internal/agenda/condicao_test.go` |
| Formulário "Quando avisar" e linha "Avisou/Sem aviso" no histórico | `web/index.html` |
