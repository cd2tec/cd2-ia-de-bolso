# Mini agente (CIDB-1) · Resumo das Histórias 1 a 6

Situação em 25/11/2026. Detalhes nos arquivos `01` a `06` desta pasta; evidências em
`~/work/cd2/evidencias/` (bateria: `h6-bateria-2026-11-24/`, um JSON e um log por rodada).

## 1. O que foi feito, trade-offs e por quê

| História | Entrega | Trade-off / limitação | Por quê |
|---|---|---|---|
| **H1 · CIDB-2** Mapeamento | Documento: agendador no cd2-ctl, execução agendada sem modelo, SQL salvo direto no banco | Só documento, sem código | A história pedia confirmar as hipóteses do épico antes de mexer no código |
| **H2 · CIDB-6** Alerta por condição | "Só quando…": limite, maior/menor e variação contra a execução anterior; conferido só por código | Limite estrito: igual ao limite não avisa | O brief não diz se "igual" dispara; "passou do limite" evita alerta na fronteira (a confirmar) |
| | | Com várias linhas, a pessoa escolhe soma, média, maior ou menor | A regra precisa de um número só, e o brief não diz qual usar |
| | | O modelo não decide se avisa | Por código o resultado é sempre o mesmo para os mesmos dados e não espera o modelo |
| **H3 · CIDB-11** Resumo | Até 3 linhas; cada número conferido contra os dados; até 2 tentativas; se for recusado, o aviso sai sem resumo | Garante o número, não a frase (cerca de 1 em 3 resumos com frase falsa) | A conferência só sabe se o número existe nos dados; conferir o sentido da frase pediria outro modelo ou regras de sentido, mais lento e fora do brief |
| | | Até 2 tentativas; a 2ª só quando o resumo é recusado | Mesmo limite do chat; mais tentativas atrasariam o aviso |
| | | Se falhar, o aviso sai sem resumo | Critério da história: falha do modelo não bloqueia o aviso |
| | | O pedido vai sem compactar o resultado (até 20 linhas) | Decisão do time: compactar perderia números que a conferência usa |
| **H4 · CIDB-16** Aprofundamento | Até 3 consultas vinculadas; o modelo escolhe só da lista; o resumo inclui o resultado | Se a escolha falhar, segue sem aprofundamento | Critério H4-CT04: escolha fora da lista ou erro do modelo não travam o aviso |
| | | O modelo vê só os nomes das consultas | Pedido curto, para caber nos 15 s; a qualidade depende de nomes claros |
| | | O resumo com aprofundamento fica mais lento | O pedido junta até 20 linhas do resultado principal e 10 do aprofundamento, para cruzar as duas; em CPU, ler o pedido é o que mais demora |
| **H5 · CIDB-21** Ação | Copiar, rascunho de e-mail ou CSV; só roda depois de Aprovar; tudo fica na auditoria | Nada roda sozinho | Critério da história: confirmação humana |
| | | A ação roda na página, não no cd2-ctl | Copiar, baixar e abrir o e-mail só funcionam a partir de um clique na tela |
| | | A notificação não tem Aprovar e Recusar; o clique abre o card | O balão do Windows não aceita botões; trocar seria uma mudança grande no lançador |
| | | O e-mail sai sem destinatário e sem anexo | O destinatário não deve vir do modelo; o link de e-mail não leva anexo (o CSV é outra ação) |
| **H6 · CIDB-26** Bateria | 60 casos + 2 controles; 6 rodadas (notebook, mínimo simulado, Qwen3.5) | Banco fictício | Sem acesso a resultados reais anonimizados |
| | | Gabarito e limites definidos por nós | O produto não forneceu; ficaram registrados antes de rodar |
| | | Hardware mínimo simulado | Sem máquina de 8 GB; o processador do notebook é mais rápido, então o tempo real tende a ser pior |

Agendamentos e alertas (H2 a H5) só rodam na **cópia instalada**, não no pen drive.

## 2. Bateria do modelo no mínimo simulado (Qwen3 4B, o do pen drive)

Mínimo simulado: 4 núcleos, 8 GB, sem GPU.

| Métrica | Limite | Notebook | Mínimo simulado |
|---|---|---|---|
| Número inventado que chegou ao aviso | 0 | **0** ✓ | **0** ✓ |
| Resumos aprovados no fim | — | 60/60 | 60/60 |
| Acerto da escolha do aprofundamento | ≥ 80% | 73% ✗ | 73% ✗ |
| Tempo da escolha, 90% até | ≤ 15 s | 8,6 s ✓ | 13,8 s ✓ |
| Tempo do resumo, 90% até | ≤ 25 s | 48 s ✗ | 77 s ✗ |
| Resumos dentro do prazo | ≥ 80% | 40% ✗ | 10% ✗ |

- **Repetição (CT02):** duas rodadas iguais deram 90 de 90 respostas idênticas.
- **Controles:** o número inventado (CT05) e a escolha errada (CT06) plantados foram pegos nas 6 rodadas.
- **Gargalo:** o tempo é do modelo lendo o pedido; o banco responde em menos de 0,1 s.

## 3. Comparativo Qwen3 4B × Qwen3.5 4B

**Notebook**

| Métrica | Qwen3 4B | Qwen3.5, thinking ligado (como o produto chama hoje) | Qwen3.5, thinking desligado |
|---|---|---|---|
| Acerto da escolha | 22/30 (73%) | **0/30** (resposta vazia nas 30) | 22/30 (73%) |
| Tempo da escolha: média / 90% até | 7,4 / 8,6 s | 20,3 / 22,6 s | 6,8 / 8,6 s |
| Resumo sem número inventado na 1ª tentativa | 53/60 | n/c \* | **59/60** |
| Resumos aprovados no fim | 60/60 | n/c \* | 59/60 |
| Resumos dentro do prazo | 24/60 (40%) | n/c \* | 29/60 (48%) |
| Tempo do resumo: média / 90% até / pior | 30,2 / 48,0 / 77,9 s | n/c \* | 26,3 / 37,2 / 53,8 s |
| Frase com maior e menor trocados | 1 | n/c \* | 0 |

\* Com a escolha falhando, os resumos dessa rodada rodaram sem aprofundamento: não são comparáveis.

**Mínimo simulado**

| Métrica | Limite | Qwen3 4B | Qwen3.5, thinking desligado |
|---|---|---|---|
| Acerto da escolha | ≥ 80% | 73% ✗ | 73% ✗ |
| Tempo da escolha, 90% até | ≤ 15 s | 13,8 s ✓ | **9,8 s** ✓ |
| Número inventado que chegou ao aviso | 0 | 0 ✓ | 0 ✓ |
| Resumo sem número inventado na 1ª tentativa | — | 53/60 | **59/60** |
| Resumos dentro do prazo | ≥ 80% | 10% ✗ | 23% ✗ |
| Tempo do resumo: média / 90% até / pior | 90% ≤ 25 s | 47 / 77 / 129 s ✗ | **35 / 47 / 73 s** ✗ |
| Leitura do pedido / escrita da resposta | — | 17 / 4,1 tokens/s | 19 / 5,9 tokens/s |

- **Escolha:** os dois acertam 22 de 30, errando casos diferentes. A Qwen3 errou mais na ruptura (7 dos 8 erros).
- **Resumo:** a Qwen3.5 inventa menos (1 em 60, contra 7 em 60) e é cerca de 40% mais rápida; ainda fica fora do prazo.
- **Para trocar de modelo:** é preciso desligar o thinking na escolha (`aprofundar.go`).

## 4. Tempo até o aviso (Qwen3 4B)

| Situação | Notebook | Mínimo simulado |
|---|---|---|
| Regra não dispara | menos de 1 s | menos de 1 s |
| Dispara, só resumo | ~25 s | ~40 s |
| Dispara, com aprofundamento | ~42 s | ~67 s |

Hoje o prazo de 25 s corta o resumo: no mínimo, a maioria dos avisos sai **sem resumo**.

## 5. Recomendação (a decisão é do produto)

- **H3:** passa em qualidade e falha em tempo. Ajustar antes de liberar: aumentar o prazo, mandar um pedido menor, reaproveitar o pedido na 2ª tentativa ou trocar para a Qwen3.5 (com o thinking desligado).
- **H4:** não liberar como está. Primeiro revisar o gabarito (os erros se concentram na ruptura), depois melhorar a escolha e o tempo.
- **Em aberto:** máquina real de 8 GB, dados reais anonimizados, e a aprovação do piloto, do gabarito e dos limites.
