# Mini agente · História 6 (CIDB-26): bateria de avaliação do modelo no hardware mínimo

Bateria que libera as Histórias 3 (resumo do alerta) e 4 (aprofundamento): cerca de 30 casos por agente, a
partir das consultas agendadas de flash de vendas e de ruptura, rodando contra o modelo do jeito que o
cd2-ctl roda. Mede acerto na escolha do aprofundamento, resumos sem número inventado e latência por etapa,
no ambiente de desenvolvimento e no hardware mínimo, e compara a Qwen3 4B com a Qwen3.5 4B. Os itens
marcados **(a confirmar)** são leituras nossas, para o produto aprovar.

## Limites de aprovação (definidos antes da execução, a confirmar)

O brief não fixa números. Estes limites foram escritos e versionados **antes** da primeira rodada (commit
deste arquivo); o produto pode trocar, e a decisão de liberação é refeita com os limites dele.

| Métrica | Limite | Por quê |
|---|---|---|
| Acerto na escolha do aprofundamento (escolha entre as aceitáveis) | **≥ 80%** (24 de 30) | um em cada cinco aprofundamentos menos útil ainda deixa o aviso melhor que sem aprofundamento |
| Resumo publicado com número inventado | **0** | critério das Histórias 3 e 4; a conferência descarta o resumo recusado |
| Resumos aprovados pela conferência, dentro do prazo (60 pedidos) | **≥ 80%** | o resumo é o valor da fase; abaixo disso a maioria dos avisos sai sem ele |
| Latência da escolha no hardware mínimo | **p90 ≤ 15 s** | prazo do cd2-ctl (`PrazoEscolha`) |
| Latência do resumo no hardware mínimo | **p90 ≤ 25 s** | prazo do cd2-ctl (`PrazoResumo`), para todas as tentativas |
| Frase falsa no resumo (número certo, afirmação errada) | **sem limite; só registrar** | a conferência garante o número, não a frase (limite conhecido da História 3) |

Regra de decisão proposta: libera a história cujas métricas passam nos limites do hardware mínimo. Se só a
latência falha, a recomendação é ajustar o prazo ou o modelo antes de liberar, não liberar como está.

## O que a bateria roda

| Agente | Casos | De onde | O que mede |
|---|---|---|---|
| Resumo do alerta (História 3) | 30 (`tests/eval/resumo-casos.json`, da História 3) | 10 consultas de vendas, estoque e perdas do `erp_pdv`, 3 regras cada | número inventado na 1ª tentativa, aprovados, latência |
| Escolha do aprofundamento (História 4) | 30 (`tests/eval/aprofundamento-casos.json`) | 15 de flash de vendas e 15 de ruptura; 3 consultas vinculadas por caso | acerto contra o gabarito, latência |
| Resumo com aprofundamento (História 4) | os mesmos 30 | o resumo com o resultado da consulta escolhida | número inventado, aprovados, latência |
| Controles (sem modelo) | 2 | resposta fixa com número inventado (H6-CT05) e escolha errada (H6-CT06) | se a conferência e a métrica pegam |

- **Piloto (a confirmar):** não há dado de uso das consultas agendadas; o piloto são as consultas de flash
  de vendas e de ruptura, os exemplos do brief.
- **Massa de dados:** o `erp_pdv` fictício (`~/work/cd2/erp-pdv-mock`), não resultados reais anonimizados
  como pede a CIDB-29. Trocar a massa é só gerar os casos de outro banco (`gerar_casos_aprofundamento.py`).
- **Gabarito do aprofundamento (a confirmar):** cada caso tem a escolha esperada, as aceitáveis quando mais
  de uma explica o alerta e o porquê (`porque`), para revisão. Montado assim: entre as 3 vinculadas, a que
  abre a métrica do alerta por outra dimensão ou mostra a causa direta (reposição, perda); as outras são de
  outro assunto ou não explicam o alerta. "Acerto" conta as aceitáveis; "estrito", só a esperada.
- **Igual ao produto:** as funções de `web/condicao.js`, `web/aprofundar.js` e `web/resumo.js`, temperatura
  0, JSON schema, thinking ligado só na escolha (CIDB-17), até 2 tentativas, a segunda só na recusa.
- **Latência por etapa:** escolha e resumo, somando as tentativas, contra os prazos do cd2-ctl (15 s e
  25 s). A consulta de aprofundamento não roda na bateria (o resultado de cada opção está no caso): o tempo
  dela é do servidor do banco, não do computador da pessoa.
- **Frase com maior e menor trocados:** verificação automática parcial (a frase diz "maior" e cita o menor
  valor, ou o contrário), o erro mais comum da bateria da História 3. As outras frases falsas pedem leitura.

## Como rodar

```sh
docker compose -f ~/work/cd2/erp-pdv-mock/docker-compose.yml up -d     # só para gerar os casos
python3 tests/eval/gerar_casos_aprofundamento.py > tests/eval/aprofundamento-casos.json
node --test tests/eval/metricas.test.js                                 # H6-CT04, CT05, CT06, casos (CT01)
CD2_EVAL_ROTULO=qwen3-dev-1 CD2_EVAL_SAIDA=r.json sh tests/eval/bateria.sh
CD2_EVAL_SIMULACAO=1 CD2_EVAL_ROTULO=qwen3-min sh tests/eval/bateria.sh # hardware mínimo simulado (Linux)
CD2_EVAL_MODEL=~/work/cd2/modelos-comparacao/Qwen3.5-4B-Q4_K_M.gguf sh tests/eval/bateria.sh
```

Na máquina de hardware mínimo de verdade: abrir a IA de Bolso sem agendamentos e rodar
`CD2_EVAL_MODEL_URL=http://127.0.0.1:8080/v1/chat/completions node tests/eval/bateria.js` (precisa do Node).

## Resultados (24/11/2026)

Seis rodadas, as mesmas 90 chamadas (30 escolhas, 30 resumos, 30 resumos com aprofundamento) e os 2
controles em cada uma. Evidências (JSON por caso, log da bateria e do motor) em
`~/work/cd2/evidencias/h6-bateria-2026-11-24/`. Máquina de desenvolvimento: Intel Core Ultra 5 135U
(14 núcleos), 15 GB, sem GPU, energia `powersave`/`balanced`. "Mínimo simulado": o mesmo motor preso a
4 núcleos econômicos, 8 GB e sem swap (ver "Como rodar"). **Não houve rodada em máquina de hardware mínimo
real.**

| Rodada | Modelo | Ambiente | Thinking na escolha | Duração |
|---|---|---|---|---|
| qwen3-dev-1 | Qwen3 4B Instruct 2507 Q4_K_M (a do pen drive) | desenvolvimento | ligado (como o produto) | 33,9 min |
| qwen3-dev-2 | idem | desenvolvimento | ligado | 33,2 min |
| qwen3-min | idem | mínimo simulado | ligado | 53,0 min |
| qwen35-dev | Qwen3.5 4B Q4_K_M | desenvolvimento | ligado (como o produto) | 28,5 min |
| qwen35-dev-sem-thinking | Qwen3.5 4B Q4_K_M | desenvolvimento | desligado | 29,7 min |
| qwen35-min-sem-thinking | Qwen3.5 4B Q4_K_M | mínimo simulado | desligado | 38,9 min |

A Qwen3 4B Instruct 2507 não tem modo thinking: o `enable_thinking` da escolha não muda nada nela.

### Contra os limites, no mínimo simulado

| Métrica | Limite | Qwen3 4B (`qwen3-min`) | Qwen3.5 4B sem thinking (`qwen35-min-sem-thinking`) |
|---|---|---|---|
| Acerto na escolha | ≥ 80% | **73,3%** (22/30) ✗ | **73,3%** (22/30) ✗ |
| Resumo publicado com número inventado | 0 | 0 ✓ | 0 ✓ |
| Resumos aprovados dentro do prazo (60) | ≥ 80% | **10%** (6/60) ✗ | **23,3%** (14/60) ✗ |
| Latência da escolha, p90 | ≤ 15 s | 13,8 s ✓ (pior 14,6 s) | 9,8 s ✓ |
| Latência do resumo, p90 | ≤ 25 s | **77,1 s** ✗ (pior 129 s) | **47,1 s** ✗ (pior 73 s) |
| Frase com maior/menor trocados (aprovados) | só registrar | 1 (caso 15) | 0 |

### Todas as rodadas

| Rodada | Escolha: acerto (estrito) | Escolha: média / p90 | Resumo: 1ª tentativa sem número inventado | Aprovados | Aprovados no prazo | Resumo: média / p90 / pior |
|---|---|---|---|---|---|---|
| qwen3-dev-1 | 22/30 (19) | 7,4 / 8,6 s | 53/60 | 60/60 | 24/60 | 30,2 / 48,0 / 77,9 s |
| qwen3-dev-2 | 22/30 (19) | 7,6 / 8,8 s | 53/60 | 60/60 | 26/60 | 29,4 / 47,7 / 74,9 s |
| qwen3-min | 22/30 (19) | 11,5 / 13,8 s | 53/60 | 60/60 | 6/60 | 47,3 / 77,1 / 129,2 s |
| qwen35-dev | **0/30** (30 sem resposta) | 20,3 / 22,6 s | 60/60 \* | 60/60 \* | 55/60 \* | 18,3 / 24,7 / 44,1 s \* |
| qwen35-dev-sem-thinking | 22/30 (19) | 6,8 / 8,6 s | 59/60 | 59/60 | 29/60 | 26,3 / 37,2 / 53,8 s |
| qwen35-min-sem-thinking | 22/30 (19) | 8,4 / 9,8 s | 59/60 | 59/60 | 14/60 | 34,7 / 47,1 / 72,9 s |

\* Em `qwen35-dev` a escolha falhou nos 30 casos, então os 30 "resumos com aprofundamento" rodaram **sem**
aprofundamento (como o cd2-ctl faz quando a escolha falha): prompts menores e mais rápidos. Esses números de
resumo não são comparáveis com as outras rodadas.

Separando os dois tipos de resumo, a latência vem do resumo com aprofundamento: no mínimo simulado, com a
Qwen3, o resumo sozinho teve p90 de 66,6 s (6/30 no prazo) e o resumo com aprofundamento teve p90 de 77,1 s
(0/30 no prazo). No desenvolvimento, foram 20/30 e 4/30 no prazo.

### O que os números dizem

- **Número inventado:** nenhum resumo publicado tem número fora dos dados, em nenhuma rodada. A conferência
  recusou o que o modelo inventou: Qwen3, 7 de 60 na 1ª tentativa, todos corrigidos na 2ª; Qwen3.5, 1 de
  60 (A24, "os 10 itens", nas duas tentativas), e o aviso sai sem resumo.
- **Escolha do aprofundamento:** os dois modelos acertam 22 de 30, abaixo dos 80%. A Qwen3 errou A05, A19,
  A20, A21, A22, A27, A28 e A30, e a Qwen3.5 errou A05, A06, A12, A18, A19, A20, A21 e A22. Os erros da
  Qwen3 se concentram na ruptura: 7 de 8, contra 5 de 8 na Qwen3.5. Em ruptura, os modelos preferem uma consulta de
  faturamento ou de produto mais vendido a compras pendentes ou ruptura por fornecedor. Em A05 e A27, a
  Qwen3 escolheu "Clientes cadastrados por cidade", que não tem relação com o alerta. O gabarito é nosso
  (a confirmar): se o produto aceitar outras escolhas, o acerto muda.
- **Latência:** a escolha cabe nos 15 s, e o resumo não cabe nos 25 s, nem no desenvolvimento. Quase todo o
  tempo vai para a **leitura do prompt** pelo modelo, não para escrever a resposta. No mínimo simulado, a
  Qwen3 lê cerca de 17 tokens/s e escreve cerca de 4 tokens/s. O resumo com aprofundamento tem de 500 a
  900 tokens de entrada, o que dá de 30 a 50 s só lendo. Na 2ª tentativa, o prompt é lido de novo. Na
  Qwen3.5, a leitura é de cerca de 19 tokens/s e a escrita de cerca de 6 tokens/s no mínimo simulado. Velocidades medidas
  pela média dos `print_timing` do log do motor.
- **Qwen3.5 do jeito que o produto chama:** com thinking ligado e 60 tokens de saída, ela gasta os 60
  tokens pensando e devolve resposta vazia nos 30 casos, nas duas tentativas (≈ 20 s). Para usá-la, a
  escolha precisa de thinking desligado ou de mais tokens (CIDB-17). Com thinking desligado, ela empata na
  escolha, é melhor no resumo (59/60 sem número inventado na 1ª, nenhuma frase trocada) e é mais rápida
  no mínimo simulado (p90 do resumo 47 s × 77 s). Ainda fica fora do prazo.
- **Repetição (H6-CT02):** `qwen3-dev-1` × `qwen3-dev-2`: 90 de 90 resultados iguais (mesma escolha, mesmo
  texto de resumo), só a latência varia (±3%). O mesmo vale entre desenvolvimento e mínimo simulado de cada
  modelo (90/90 iguais): com temperatura 0, o ambiente muda o tempo, não a resposta.
- **Controles:** H6-CT05 (número inventado plantado) foi recusado pela conferência, e H6-CT06 (escolha
  errada plantada) contou como erro, nas 6 rodadas.
- **Frase com maior/menor trocados:** 1, no caso 15 com a Qwen3: "O valor em Cartão de Débito caiu para
  164.741,66, o menor entre todas as formas." O menor era o PIX (142.405,34), citado na frase seguinte. A
  conferência aprova, porque o número existe nos dados. A leitura humana das outras frases não foi feita.

### Recomendação de liberação (a decisão é do produto)

Pela regra definida antes da execução, **nenhuma das duas histórias passa no mínimo simulado como está**:

- **História 3 (resumo do alerta):** passa em qualidade, com 0 número inventado publicado e 100% aprovados
  com a Qwen3. Falha só em latência: 6/30 no prazo, p90 66,6 s. A regra diz para ajustar antes de
  liberar. As opções, sem escolher por vocês:
  - aumentar o `PrazoResumo`, com o aviso saindo primeiro e o resumo depois, se a tela já suporta isso;
  - diminuir o prompt, com menos linhas do resultado;
  - reaproveitar o prompt já lido na 2ª tentativa (cache);
  - trocar para a Qwen3.5, que é cerca de 40% mais rápida no resumo, mas ainda fica fora do prazo.
- **História 4 (aprofundamento):** falha no acerto (73% < 80%) e na latência do resumo com aprofundamento
  (0/30 no prazo no mínimo simulado). Recomendação: não liberar como está. Antes, o produto revisa o
  gabarito, principalmente o da ruptura. Depois é preciso melhorar a escolha (instrução ou descrição das
  consultas) e resolver a latência como na História 3.
- **Modelo:** a Qwen3.5 4B é candidata, com resumo melhor e mais rápido e escolha igual. A troca pede
  desligar o thinking na escolha (mudança no `aprofundar.go`) e trocar o arquivo no pen drive. Os GGUF dela
  são de terceiros (unsloth), como o da Qwen3 atual.

### O que continua em aberto

- **Hardware mínimo real (H6-CT03, critério 3):** as rodadas "mín." são simulação. O processador de 2024
  tende a dar tempos **menores** que um PC básico, e os 8 GB valem só para o motor. Como nem a simulação
  cabe no prazo, o resultado real deve ser pior.
- **Massa (H6-CT01):** foi usado o `erp_pdv` fictício, não resultados reais anonimizados.
- **Pendências do produto:** a consulta piloto, o gabarito e os limites estão (a confirmar).
