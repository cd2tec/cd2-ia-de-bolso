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

## Resultados

(preenchido depois das rodadas)
