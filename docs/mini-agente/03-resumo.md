# Mini agente · História 3 (CIDB-11): resumo pelo modelo quando a condição dispara

Quando a condição do agendamento dispara, o cd2-ctl pede à Qwen3 4B um resumo de até 3 linhas sobre o
resultado e a base de comparação. O código confere cada número; o resumo inválido é descartado e o aviso sai
como antes. Os itens marcados **(a confirmar)** são leituras nossas dos critérios, para o produto aprovar.

## Fluxo

```
consulta salva roda (SQL aprovado, sem modelo)
   └─ condição conferida pelo código (História 2)
        ├─ não disparou → grava no histórico, sem aviso, sem modelo
        └─ disparou
             ├─ web/resumo.js monta o pedido: frase do alerta, base de comparação, resultado (até 20 linhas,
             │    sempre com a linha que disparou nas regras de maior/menor valor)
             ├─ modelo responde em JSON schema {"linhas": [1 a 3 frases]}, thinking desligado
             ├─ web/resumo.js confere: formato e cada número/data
             │    └─ recusou → nova tentativa com o motivo (no máximo 2 tentativas)
             ├─ válido → aviso e card do histórico com o resumo
             └─ falhou, recusou 2 vezes ou estourou o prazo → aviso como antes; o card diz por quê
```

## Critérios e onde estão

| Critério | Como foi feito | Onde |
|---|---|---|
| Disparou: uma chamada gera até 3 linhas | `resumir` só com `Disparou && Aviso`, em execução agendada | `ctl/internal/agenda/resumo.go`, `agendador.go` |
| Saída restrita por JSON schema e validada | `response_format: json_schema` com `{linhas: 1..3 textos de até 160 caracteres}`; `conferir` valida de novo | `web/resumo.js` (`SCHEMA`, `conferir`) |
| Todo número existe no resultado ou na base | ver "Números aceitos" | `web/resumo.js` (`permitidos`, `conferir`) |
| Falha, validação após o limite ou timeout não bloqueiam | o aviso usa o texto da condição; o histórico grava `resumo.falha` (`prazo`, `modelo`, `formato`, `numero`) e `tentativas` | `resumo.go` |
| Sem disparo, não chama o modelo | conferido também em "Rodar agora" e sem condição | `resumo_test.go` |
| Thinking desligado | `chat_template_kwargs: {enable_thinking: false}` no pedido; o modelo do produto (Qwen3-4B-Instruct-2507) já não tem thinking; `<think>…</think>` é descartado | `resumo.go`, `web/resumo.js` |
| Notificação e card mostram o resumo válido | aviso: "Rodou às HH:MM · frase 1⏎frase 2⏎frase 3"; card: bloco "Resumo da IA · números conferidos com o resultado" | `agendador.go`, `web/index.html` (`blocoResumo`) |

## O que vai para o modelo

```
Alerta: A soma de faturamento caiu 15,5% (669.372,83 para 565.398,67) em relação à execução anterior.
Base de comparação: execução anterior = 669.372,83
Resultado (colunas: loja, faturamento):
loja: Loja 01 - Centro; faturamento: 201.268,45
…
```

- **Sem o nome da consulta:** o resumo sai "a partir do resultado atual e da base de comparação". O nome
  traz números que não são do resultado ("últimos **7** dias"): no teste, o modelo os copiava e a conferência
  recusava o resumo. Na bateria (30 casos), tirar o nome ou mantê-lo aceitando os números dele deu o mesmo
  resultado; ficou a versão sem o nome, que não abre exceção na conferência.
- Base de comparação: **valor fixo da regra** (maior/menor que), **execução anterior** ou **média das N
  execuções anteriores**; "nenhuma" quando não há.
- Até 20 linhas e 8 colunas do resultado, com datas em dd/mm/aaaa e números no formato brasileiro. Com mais
  linhas, o pedido diz "as 20 primeiras de N linhas". O corte em 20 é nosso, para o pedido não ficar lento em
  CPU sem placa de vídeo.
- **Linha que disparou o alerta:** nas regras "o maior valor de" e "o menor valor de", a linha com esse valor
  vai sempre no pedido. Se ela estiver depois da 20ª, entra no lugar da 20ª e o pedido diz "20 de N linhas,
  com a linha que disparou o alerta (a última)"; a data e os números dela passam a valer na conferência. Sem
  isso, o modelo não via a linha e apontava o maior ou o menor entre as 20 que recebeu (medido na bateria:
  "Faturamento por dia nos últimos 30 dias", menor valor no 30º dia; o resumo dizia que o pior dia era outro).
  Em "o valor de" a linha conferida é a primeira, que sempre vai; em soma e média o valor sai de todas as
  linhas e já está no alerta.
- Temperatura 0, até 300 tokens de saída.

## Números aceitos no resumo

- **Do resultado:** o valor de cada célula enviada, números escritos dentro de textos das células ("350ml",
  "Loja 04") e as datas das células.
- **Da base:** o valor fixo da regra, ou o valor da execução anterior ou da média.
- **Calculados pelo código a partir deles (a confirmar):** o valor conferido pela regra (a soma, a média, o
  maior...) e a variação %. Já estão no aviso de hoje; sem eles o resumo não poderia repetir o alerta.
- **Não entram:** números do título da consulta, dos nomes de coluna, contagens de linhas, o percentual-limite
  de uma regra de variação e qualquer conta nova.
- Arredondamento: vale só o que o próprio texto mostra ("135.979" para 135.979,13; "384,9 mil"); "135.980"
  não vale.
- **Limite conhecido:** a conferência garante que cada número existe, não o que a frase afirma. O critério
  pede só a existência. Ver "Evidências e medições".

## Tentativas e prazo

- Até **2 tentativas** (a confirmar): o mesmo limite do chat (`web/ask.js`). A segunda só acontece quando a
  conferência recusa a primeira, e leva a resposta recusada e o motivo ("número 12 não está nos dados").
  Erro do modelo (sem resposta) não tenta de novo. Leitura nossa de "uma única chamada" com "após o limite
  de tentativas": uma chamada por tentativa.
- **Prazo de 25 s para todas as tentativas** (a confirmar; `PrazoResumo` em `resumo.go`). O aviso espera o
  resumo até o prazo. O modelo atende um pedido por vez: com o chat em uso, o resumo espera na fila.

## Onde aparece

- **Notificação** (Windows e Mac, pelo lançador): o texto do aviso vira as frases do resumo. O balão do
  Windows mostra até 255 caracteres; o resumo inteiro fica no card.
- **Card do histórico:** bloco "Resumo da IA · números conferidos com o resultado" abaixo de "Avisou: …".
  Sem resumo válido: "Sem resumo da IA: o resumo citava um número que não estava nos dados (2 tentativas)."
- Não aparece em execução sem disparo, sem condição, em "Rodar agora" nem no chat.

## Evidências e medições

**Na tela (linux-dev, 24/11, "Faturamento nos últimos 7 dias" a cada 5 min, regra "a soma de faturamento for
maior que 1").** Prints e registros do histórico em `~/work/cd2/evidencias/h3-tela-2026-11-24/`.

| Execução | O que aparece | Critério |
|---|---|---|
| 01:55 | "Avisou: A soma de faturamento = 500.659,36, acima de 1." e o bloco "Resumo da IA" com 3 frases; 1ª tentativa, 9,9 s | resumo no card; números conferidos |
| 01:50 | "Avisou: …" e "Sem resumo da IA: o modelo demorou mais que o prazo." (modelo ocupado logo depois de abrir a IA) | falha não bloqueia o aviso |

O resumo das 01:55:

> Faturamento total acima de 1, com valor de 500.659,36.
> O dia 22/11/2026 teve o maior faturamento de 51.612,16.
> O dia 21/11/2026 teve o segundo maior faturamento de 117.479,24.

Todos os números existem no resultado, e a conferência aprovou. Mas a 2ª e a 3ª frases são falsas: 22/11 foi
o **menor** dia e 21/11, o **maior**, como diz a legenda do gráfico logo abaixo no mesmo card ("Maior valor em
21/11/2026: 117.479,24").

**Bateria (23/11, 30 casos do erp_pdv, modelo só para a bateria).** Arquivos em
`~/work/cd2/evidencias/h3-bateria-2026-11-23/`.

| Medida | Resultado |
|---|---|
| Aprovados pela conferência (até 2 tentativas) | 30 de 30 (24 na 1ª) |
| Aprovados com frase falsa (leitura humana contra os dados) | 10 de 30 |
| 1ª tentativa, casos 01–14 | média 26 s (16 a 49 s) |
| Prontos em até 25 s, casos 01–14 | 6 de 14 |

- Tipos de frase falsa: maior e menor trocados; "segundo maior" errado; limite da regra lido como valor
  anterior ("caiu de 2.220", que era o limite); conta nova que passa porque o número aparece em outro lugar
  ("3 vezes maior", com o "3" de "Loja 03"); tendência sem dado ("subiu", "permaneceu").
- A mesma bateria comparou o pedido com e sem o nome da consulta: deu o mesmo resultado (com o nome, 9 de 30
  com frase falsa). Ficou sem o nome.
- Do caso 15 em diante a CPU caiu para 0,4 GHz (modo de economia) e os tempos não valem; os acertos, sim.
- CPU de 14 núcleos, não o hardware mínimo do brief (8 GB, sem placa de vídeo): a latência no hardware mínimo
  ainda precisa ser medida (História 6).

**O que fica para o produto:** a conferência cumpre o critério (nenhum número fora dos dados), mas não impede
frase falsa com número verdadeiro. Liberar assim (o app já diz "pode errar"), mudar o critério ou segurar a
liberação é decisão do produto, junto com a História 6, assim como o prazo de 25 s.

## Dependências

- **História 6 (CIDB-26):** a bateria de avaliação libera esta história (resumos sem número inventado e
  latência no hardware mínimo). A História 3 pode ser revisada e integrada antes; a liberação aos usuários
  segue a decisão registrada na História 6.
- Decisões em aberto do épico: modelo (Qwen3 4B x Qwen3.5 4B) e se o resumo entra só na versão instalada (o
  agendamento já só roda nela).
