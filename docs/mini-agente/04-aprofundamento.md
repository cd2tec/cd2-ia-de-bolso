# Mini agente · História 4 (CIDB-16): aprofundamento condicional com escolha pelo modelo

No agendamento, a pessoa vincula até 3 consultas salvas como "aprofundamentos". Quando a condição dispara, a
Qwen3 4B escolhe uma delas (saída presa a um enum) e o cd2-ctl roda a escolhida com o SQL salvo. O resumo da
História 3 passa a incluir o resultado dela. Os itens marcados **(a confirmar)** são leituras nossas dos
critérios, para o produto aprovar.

## Fluxo

```
consulta salva roda (SQL aprovado, sem modelo)
   └─ condição conferida pelo código (História 2)
        ├─ não disparou → grava no histórico, sem aviso, sem modelo, sem aprofundamento
        └─ disparou
             ├─ sem aprofundamento vinculado → resumo da História 3
             └─ com 1 a 3 vinculados
                  ├─ web/aprofundar.js monta a escolha: alerta, base e os nomes das vinculadas
                  ├─ modelo responde {"escolha": "<nome>"}, enum dos nomes; até 2 tentativas, prazo de 15 s
                  │    └─ falhou → segue sem aprofundamento (resumo da História 3)
                  ├─ a escolhida roda com o SQL salvo (o mesmo executor da consulta agendada)
                  │    └─ falhou → segue sem aprofundamento (resumo da História 3)
                  └─ resumo (História 3) com o resultado do aprofundamento no pedido
                       └─ falhou → aviso no formato atual
```

## Critérios e onde estão

| Critério | Como foi feito | Onde |
|---|---|---|
| Formulário vincula de 0 a 3 consultas salvas | "Aprofundar quando a regra bater" dentro de "Só quando…": as outras consultas salvas em caixas; com 3 marcadas, as demais travam e o limite aparece | `web/index.html` (`pintarAprofAgendar`, `agendaDaJanela`), `web/aprofundar.js` (`validar`) |
| O modelo escolhe exatamente uma, com enum das vinculadas | `response_format: json_schema` com `{escolha: enum dos nomes}`; a conferência aceita só um nome da lista; fora dela, nova tentativa com o motivo | `web/aprofundar.js` (`pedido`, `conferir`), `ctl/internal/agenda/aprofundar.go` (`escolher`) |
| A escolhida roda de forma determinística, sem o modelo escrever SQL | o modelo devolve só o nome; o código acha a consulta e roda o SQL salvo pelo mesmo executor da execução agendada | `agendador.go` (`consultar`), `aprofundar.go` |
| O resumo inclui o resultado do aprofundamento | bloco "Aprofundamento" no pedido do resumo; os números dele valem na conferência | `web/resumo.js` (`recorteAprof`, `mensagens`, `permitidos`) |
| No máximo 1 aprofundamento por execução | uma escolha e uma consulta; o agendador lê no máximo 3 vinculadas | `aprofundar.go`, `web/aprofundar.js` (`ids`) |
| Sem vinculadas, comportamento da História 3 | sem escolha; o pedido do resumo é idêntico ao da História 3 | `aprofundar.go`, `web/resumo.js` |
| Falha na escolha ou na execução não bloqueia | o histórico grava `aprofundamento.falha`; o aviso segue com o resumo da História 3 ou no formato atual | `aprofundar.go`, `agendador.go` |
| Sem disparo, nem modelo nem aprofundamento | conferido também em "Rodar agora" | `aprofundar_test.go` |
| Histórico grava a escolhida e o resultado (CIDB-17) | `aprofundamento {id, nome, valores, resultado, falha, tentativas, ms}` na execução | `historico.go` |
| Card mostra o aprofundamento (CIDB-18) | "Aprofundou em: <consulta>" com o resultado recolhido, ou "Sem aprofundamento: <motivo>" | `web/index.html` (`blocoAprofundamento`) |

Testes (CIDB-20):

| Caso | Onde |
|---|---|
| H4-CT01 3 vinculadas: escolhe 1, roda, entra no resumo | `TestAprofundamentoEscolhidoRodaEEntraNoResumo` |
| H4-CT02 sem vinculadas: História 3 | `TestSemAprofundamentoEhAHistoria3`, `tests/aprofundar.test.js` |
| H4-CT03 mais de 3: formulário impede e informa | `tests/agendar-aprof.test.js`, `tests/aprofundar.test.js` |
| H4-CT04 fora do enum: recusa; depois do limite, sem aprofundamento | `TestEscolhaForaDaListaSegueSemAprofundamento` |
| H4-CT05 a consulta falha | `TestConsultaDoAprofundamentoFalhaNaoBloqueia` |
| H4-CT06 timeout da escolha | `TestPrazoDaEscolhaNaoBloqueia` |
| H4-CT07 não disparou | `TestSemDisparoSemAprofundamento` |
| H4-CT08 no máximo 1 | `TestAprofundamentoEscolhidoRodaEEntraNoResumo` (duas consultas rodam: a agendada e uma de aprofundamento) |
| H4-CT09 SQL idêntico ao salvo | idem (o SQL que rodou é o salvo da escolhida) |
| H4-CT10 regressão das Histórias 2 e 3 | suítes `condicao_test.go`, `resumo_test.go`, `tests/condicao.test.js`, `tests/resumo.test.js` sem mudança |

## O que vai para o modelo

**Escolha:**

```
Alerta: A soma de faturamento caiu 15,5% (669.372,83 para 565.398,67) em relação à execução anterior.
Base de comparação: execução anterior = 669.372,83
Consultas de aprofundamento:
- Faturamento por loja nos últimos 7 dias
- Faturamento por departamento nos últimos 7 dias
- Faturamento por fornecedor nos últimos 7 dias
```

Resposta presa a `{"escolha": <um dos nomes>}`, temperatura 0, até 60 tokens.

**Resumo:** o pedido da História 3 e mais

```
Aprofundamento (resultado de outra consulta, para explicar o alerta; colunas: loja, faturamento):
loja: Loja 01 - Centro; faturamento: 201.268,45
…
Use também o Aprofundamento para explicar o alerta; os números e datas dele também valem, escritos exatamente como estão.
```

## Leituras nossas e trade-offs (a confirmar)

- **Thinking "ligado só na escolha" (CIDB-17).** O pedido da escolha vai com `enable_thinking: true` e o do
  resumo com `false`. O modelo do produto, Qwen3-4B-**Instruct**-2507, não tem modo thinking: a chave não muda
  nada nele. Para haver thinking de fato seria preciso trocar o modelo, que é decisão em aberto do épico.
  Mesmo com um modelo com thinking, a saída presa ao JSON schema do llama.cpp não deixa espaço para o raciocínio
  antes do JSON: isso também precisaria ser revisto nessa troca.
- **O que o modelo vê para escolher.** A frase do alerta, a base de comparação e os **nomes** das vinculadas,
  sem o resultado inteiro, para a escolha sair rápida (pedido curto, resposta de poucos tokens). A qualidade da
  escolha depende de nomes claros ("Faturamento por loja", não "Consulta 3").
- **Enum com os nomes.** O valor que o modelo devolve é o nome da consulta, que ele entende; o código traduz
  para o id. Nomes repetidos ganham " (2)", " (3)".
- **Com uma só vinculada, o modelo é chamado mesmo assim.** O critério diz que o modelo escolhe; com uma opção o
  enum só aceita ela. Custa alguns segundos; pular a chamada seria uma regra nossa.
- **A consulta de aprofundamento não é filtrada pela anomalia.** Ela roda como foi salva, com o período dela
  recalculado no horário da execução (o mesmo `valoresDaExecucao` da agenda). Não vira "a loja que caiu": quem
  cruza as duas informações é o resumo.
- **Nome da consulta de aprofundamento fora do pedido do resumo.** Pelo mesmo motivo da História 3 (números do
  nome que não estão nos dados). As colunas explicam o que é.
- **Até 10 linhas do aprofundamento** no pedido do resumo (o resultado principal segue com até 20), para o pedido
  não ficar lento em CPU. O card mostra as mesmas 10.
- **Prazos.** Escolha: **15 s** (todas as tentativas, `PrazoEscolha`). Consulta: os 15 s de qualquer consulta.
  Resumo: os **25 s** da História 3. No pior caso o aviso espera a soma. Medido na tela (ver "Evidências"): a
  escolha leva 3 a 6 s, mas o resumo com o aprofundamento ficou maior e **não coube nos 25 s** em nenhuma das 3
  execuções. Aumentar o prazo do resumo quando há aprofundamento é **decisão do produto**; a latência no
  hardware mínimo é medida na História 6.
- **Formulário.** Aprofundamento só com "Só quando…" (com "Toda vez que rodar" a lista fica escondida e não é
  gravada); a própria consulta não aparece; vinculada que foi apagada aparece como "(consulta apagada)" e
  impede salvar até ser desmarcada. No agendador, vinculada apagada é "sem_consulta" e segue sem aprofundamento.
- **A execução do aprofundamento não vira execução da consulta vinculada.** Ela fica dentro da execução que
  disparou, no histórico dela; o histórico da consulta vinculada não muda.

## Evidências (linux-dev, 24/11, CPU de 14 núcleos)

Registros do histórico e o script da reprodução em `~/work/cd2/evidencias/h4-tela-2026-11-24/` (os prints do navegador falharam nesta sessão; o texto dos cards foi lido da página).

**Formulário.** "Faturamento nos últimos 7 dias", regra "a soma de faturamento for maior que 1", com 3
aprofundamentos marcados: "Faturamento por loja nos últimos 7 dias", "Faturamento por forma de pagamento nos
últimos 7 dias" e "Produto com mais saída nos últimos 7 dias" (as duas primeiras salvas pelo chat, com 👍). Com 3
marcadas, a 4ª ("Faturamento nos últimos 30 dias") ficou travada e apareceu "Limite de 3 aprofundamentos:
desmarque uma para escolher outra." (H4-CT03). Gravado: `agenda.aprofundamentos` com os 3 ids.

**Execuções agendadas (03:30, 03:35, 03:40):**

| Execução | Escolha | Consulta escolhida | Resumo | Card |
|---|---|---|---|---|
| 03:30 | "Faturamento por loja…", 1ª tentativa, 6,0 s (modelo frio) | rodou, 4 lojas | prazo (1 tentativa) | "Aprofundou em: Faturamento por loja nos últimos 7 dias" e "Sem resumo da IA: o modelo demorou mais que o prazo." |
| 03:35 | idem, 3,4 s | rodou | prazo (2 tentativas) | idem, "(2 tentativas)" |
| 03:40 | idem, 4,2 s | rodou | prazo (2 tentativas) | idem |

O aviso saiu nas três ("Avisou: A soma de faturamento = 500.659,36, acima de 1."): a falha do resumo não
bloqueou a notificação.

**O resumo da execução das 03:40, reproduzido fora do prazo** (o mesmo pedido, com o aprofundamento):

| Tentativa | Tempo | Resposta | Conferência |
|---|---|---|---|
| 1ª | ~12 s | "…As lojas Loja 01 - Centro e Loja 02 - Taquaral contribuíram com mais de 300 mil reais cada." | recusada: "número 300 não está nos dados" (e a frase é falsa: a Loja 02 faturou 146.331,59) |
| 2ª | ~19 s | "…Loja 01 - Centro teve faturamento de 179.243,49, o maior entre todas as lojas." | aprovada; a frase do aprofundamento é verdadeira |

A 2ª resposta ainda repete o erro da História 3 ("O dia 22/11/2026 teve o maior faturamento de 51.612,16": foi o
menor). As duas tentativas somam ~31 s, acima dos 25 s do resumo: por isso, na tela, o card ficou sem resumo.

## Dependências

- **História 6 (CIDB-26):** a bateria mede o acerto na escolha do aprofundamento no hardware mínimo. Medir
  acerto pede a resposta certa de cada caso (gabarito): quem define é o produto. A bateria atual
  (`tests/eval/resumo.js`) ainda não tem casos de aprofundamento.
- **Limite herdado da História 3:** a conferência garante que cada número existe nos dados (agora também no
  aprofundamento), não o que a frase afirma. Ver `03-resumo.md`, "Evidências e medições".
