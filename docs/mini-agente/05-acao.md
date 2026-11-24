# Mini agente · História 5 (CIDB-21): ação com confirmação humana e auditoria

O resumo da História 3 passa a sugerir uma ação, presa a um enum: copiar o resumo, abrir um rascunho de
e-mail com o resumo ou gerar o arquivo CSV com o resultado. Nada roda sozinho: o card do histórico mostra a
sugestão com **Aprovar** e **Recusar**, a ação só roda depois do clique e da aprovação gravada, e cada passo
fica na auditoria (o que foi proposto, a decisão, o resultado, quando e quem). Os itens marcados **(a
confirmar)** são leituras nossas dos critérios, para o produto aprovar.

## Fluxo

```
condição disparou (História 2)
   └─ resumo (História 3): o modelo devolve {"linhas": [...], "acao": <enum>}
        ├─ resumo recusado ou fora do prazo → aviso no formato atual, sem ação
        ├─ acao "nenhuma" (ou fora do enum: descartada) → resumo da História 3, sem ação
        └─ acao do enum
             ├─ histórico: resumo.acao + auditoria "proposta" (mesma transação da execução)
             ├─ notificação: "Ação sugerida: <ação>. Abra o histórico para aprovar ou recusar." + o resumo
             └─ card do histórico: a ação, Aprovar e Recusar
                  ├─ ninguém clica → nada acontece; a proposta fica pendente
                  ├─ Recusar → auditoria "recusada"; nada roda
                  └─ Aprovar → POST /agenda/acao "aprovada" (gravou?)
                       ├─ não gravou → nada roda; a tela diz por quê
                       └─ gravou → a página executa (cópia, download, mailto)
                            └─ auditoria "executada" (com o que foi feito) ou "falhou" (com o motivo)
```

## Critérios e onde estão

| Critério | Como foi feito | Onde |
|---|---|---|
| A notificação e o card exibem a ação proposta com botão para aprovar e para recusar | card: bloco "Ação sugerida pela IA" com Aprovar e Recusar; notificação: a linha da ação na frente do resumo, e o clique abre o card **(a confirmar: ver "Notificação sem botões")** | `web/index.html` (`blocoAcao`), `agendador.go`, `web/acao.js` (`textoAviso`) |
| Nenhuma ação é executada sem clique | a ação só roda em `decidirAcao`, chamada pelo botão; antes, a aprovação precisa estar gravada | `web/index.html` (`decidirAcao`) |
| Aprovar executa a ação | cópia (`navigator.clipboard`), CSV (o mesmo do botão CSV do card) e rascunho (`mailto:`) | `web/index.html` (`executarAcao`), `web/acao.js` |
| Recusar ou ignorar mantém só no histórico | recusar grava "recusada" e não roda nada; ignorar não grava nada além da proposta | `web/index.html`, `auditoria.go` |
| Auditoria com o que foi proposto, a decisão e quando | tabela `auditoria` no `historico.sqlite`, só de inclusão: evento, ação, data e hora, quem e detalhe | `ctl/internal/agenda/auditoria.go` |
| Enviar não envia e-mail: só abre o rascunho | `mailto:` sem destinatário; quem escolhe para quem e envia é a pessoa | `web/acao.js` (`rascunhoEmail`) |
| Falha ao executar é informada e registrada | "Não deu certo" com o motivo no card; auditoria "falhou" com o motivo | `web/index.html`, `auditoria.go` |

Subtarefas:

| Subtarefa | O que pediu | Onde |
|---|---|---|
| CIDB-22.1 JSON schema da proposta, enum, junto ao resumo | `acao: enum["copiar_resumo", "rascunho_email", "gerar_csv", "nenhuma"]` no schema do resumo, sem parâmetros: o conteúdo de cada ação sai do código (o resumo e o resultado), não do modelo | `web/resumo.js` (`SCHEMA`), `web/acao.js` |
| CIDB-22.2 executor, só após confirmação | na página, depois da aprovação gravada | `web/index.html` (`executarAcao`) |
| CIDB-22.3 tabela de auditoria: proposta, decisão, data e hora, usuário, resultado | `auditoria (chave, consulta, seq, exec_em, evento, acao, em, por, detalhe)` | `auditoria.go` |
| CIDB-22.4 nenhuma ação sem confirmação registrada | a rota recusa resultado sem "aprovada", decisão repetida, ação diferente da sugerida e execução que não sugeriu nada; a página só executa com a aprovação gravada | `auditoria.go` (`Auditar`), `web/index.html` |
| CIDB-22.5 falha com registro e retorno de erro | "falhou" com o motivo; erros da rota viram 404/409/400 com a mensagem | `auditoria.go`, `ctl/internal/api/agenda.go` |
| CIDB-23 card, retorno de sucesso e erro, auditoria por execução | bloco da ação; "Feito." / "Não deu certo: …"; "Auditoria" recolhida com os eventos | `web/index.html` (`blocoAcao`) |

Testes (CIDB-25):

| Caso | Onde |
|---|---|
| H5-CT01 com ação proposta, sem clique: nada roda | `TestPropostaGravadaComAExecucao`, `tests/historico-acao.test.js` |
| H5-CT02 aprovar "copiar resumo" | `TestAprovarEExecutar`, `tests/historico-acao.test.js` |
| H5-CT03 aprovar "rascunho de e-mail": abre sem enviar | `tests/historico-acao.test.js`, `tests/acao.test.js` |
| H5-CT04 aprovar "gerar CSV": conteúdo esperado e auditoria | `tests/historico-acao.test.js`, `tests/acao.test.js` |
| H5-CT05 recusar: nada roda, recusa registrada | `TestRecusarImpedeExecutar`, `tests/historico-acao.test.js` |
| H5-CT06 falha ao gerar o arquivo | `TestFalhaRegistrada`, `tests/historico-acao.test.js` (resultado sem linhas; cópia negada) |
| H5-CT07 auditoria de cada caso: proposta, decisão, data e hora | `TestAprovarEExecutar`, `TestRecusarImpedeExecutar`, `TestFalhaRegistrada`, `tests/acao.test.js` (`textoEvento`) |
| H5-CT08 ação fora do enum | `TestAcaoForaDoEnumOuNenhuma`, `tests/acao.test.js` (`lerAcao`) |
| H5-CT09 regressão sem ação proposta | `TestAcaoForaDoEnumOuNenhuma` ("nenhuma"), suítes das Histórias 2, 3 e 4 sem mudança de comportamento |
| Regras da auditoria (sem aprovar, ação trocada, outra execução) | `TestRegrasDaAuditoria`, `TestAcaoGravaNaAuditoria` (rota) |
| Auditoria sobrevive ao apagar o histórico; banco antigo ganha a tabela | `TestAuditoriaSobreviveAoApagar`, `TestEsquemaV1GanhaAuditoria` |

## O que vai para o modelo

O pedido da História 3 (e da 4, com aprofundamento) muda em três pontos: o schema ganha `acao`, a instrução
de formato passa a ser `{"linhas": [...], "acao": "..."}` e entra uma regra:

```
- Em "acao", sugira uma ação que a pessoa pode aprovar depois de ler o alerta: "copiar_resumo" (copiar
  este resumo), "rascunho_email" (abrir um rascunho de e-mail com este resumo, sem enviar), "gerar_csv"
  (baixar o resultado em CSV) ou "nenhuma".
```

## Leituras nossas e trade-offs (a confirmar)

- **Lista de ações.** A história diz que a lista final é decisão em aberto, e o critério de entrada dos testes
  pede a lista decidida. Implementamos as três candidatas do brief, como combinado; tirar uma é tirar do enum
  em `web/acao.js`.
- **Quando sugerir qual ação.** O brief não diz. A regra no pedido só descreve cada ação e deixa "nenhuma";
  quem escolhe é o modelo, sem regra de negócio nossa. Medir se a sugestão faz sentido pede um gabarito do
  produto (como o da escolha do aprofundamento, História 6).
- **Notificação sem botões.** O balão do Windows (`Shell_NotifyIcon`) não aceita botões; ter Aprovar e Recusar
  na notificação pediria trocar para o toast do Windows (WinRT) e as ações do Mac (`UNNotificationAction`),
  uma mudança grande no lançador. A notificação **mostra** a ação ("Ação sugerida: …. Abra o histórico para
  aprovar ou recusar."), na frente do resumo porque o balão corta o fim, e o clique abre o card, onde estão os
  botões. No Linux de desenvolvimento não há notificação.
- **A ação roda na página, não no cd2-ctl.** Copiar, baixar arquivo e abrir o `mailto:` só funcionam a partir
  do clique na tela; isso também garante "nada sem clique". O cd2-ctl guarda a auditoria e recusa o que fura a
  ordem (resultado sem aprovação, decidir duas vezes, ação trocada).
- **Rascunho de e-mail sem destinatário e sem anexo.** Nada no produto diz "para quem avisa", e o destinatário
  não deve vir do modelo; quem escolhe é a pessoa, no rascunho. O `mailto:` não leva anexo: o CSV é outra
  ação. Corpo cortado em 1.800 caracteres (URL longa demais não abre).
- **A página não sabe se o programa de e-mail abriu.** O navegador não avisa. Sem programa de e-mail
  configurado (massa de dados da CIDB-25), o `mailto:` não abre nada e a auditoria registra "executada:
  rascunho pedido ao programa de e-mail (sem envio)". Não há como detectar esse caso na página.
- **CSV é o resultado da execução.** O brief cita "lista de reposição em CSV": a lista é o resultado da consulta
  agendada (por exemplo, a de ruptura). Não montamos outro conteúdo, para não inventar regra; é o mesmo CSV do
  botão "CSV" do card.
- **Usuário na auditoria.** O app não tem login: grava o usuário do sistema operacional. A proposta é gravada
  como "IA de Bolso".
- **Ignorar não expira.** A proposta fica pendente no card até alguém decidir.
- **Uma decisão por proposta; falha não tem "tentar de novo".** Depois de "falhou", o card mostra o motivo; o
  botão "CSV" do card e a cópia manual continuam lá. Repetir seria uma nova aprovação, que o critério não pede.
- **Sem resumo, sem ação.** A ação vem junto ao resumo (CIDB-22.1): resumo recusado ou fora do prazo deixa a
  execução sem ação, como na História 3. Com aprofundamento, o resumo já não coube nos 25 s nesta máquina
  (História 4), então na prática a ação aparece só quando o resumo sai a tempo.
- **A auditoria não some ao apagar o histórico.** É registro: "Apagar o histórico" tira as execuções, não os
  eventos. A execução é identificada pelo seq e pelo horário em que rodou, para a execução nova com o mesmo seq
  não herdar eventos antigos.
- **Demonstração sem ação.** O modo demonstração não tem cd2-ctl para gravar a auditoria.
- **Windows (WebView2).** Cópia, download e `mailto:` dentro da janela do lançador no Windows não foram testados
  nesta máquina (Linux).

## Evidências (linux-dev, 24/11, CPU de 14 núcleos)

Arquivos em `~/work/cd2/evidencias/h5-acao-2026-11-24/`: print do card, a execução das 12:10 com a
auditoria, o script e a saída da reprodução.

**Execução agendada das 12:10** ("Faturamento nos últimos 7 dias", regra "soma de faturamento maior que 1",
3 aprofundamentos): escolheu "Faturamento por loja nos últimos 7 dias", o resumo passou na 1ª tentativa em
18,9 s com `"acao": "copiar_resumo"`, e a auditoria gravou `proposta · copiar_resumo · IA de Bolso · 12:10:53`.
No card (`01-card-acao-pendente-1210.jpg`): "Ação sugerida pela IA: Copiar o resumo", "Nada é feito sem o seu
clique.", Aprovar, Recusar e a auditoria com "Sugerida pela IA em 24/11/2026 às 12:10:53". Aprovar e recusar
ficaram para o teste do time.

**Reprodução do pedido com a ação, sem aprofundamento** (3 rodadas, mesma execução):

| Rodada | Tempo | Tokens gerados | Ação | Conferência |
|---|---|---|---|---|
| 1 | 32,8 s (pedido inteiro processado: o texto do sistema mudou, cache frio) | 107 | copiar_resumo | aprovado |
| 2 | 18,8 s | 106 | copiar_resumo | aprovado |
| 3 | 18,0 s | 106 | copiar_resumo | aprovado |

- A ação custa cerca de 8 tokens a mais na resposta. A primeira chamada depois de abrir a IA (ou de mudar o
  texto do sistema) processa o pedido inteiro e pode passar dos 25 s; as seguintes usam o cache.
- O modelo sugeriu "copiar_resumo" em todas: com a regra só descritiva, ele tende à ação mais simples. Se o
  produto quiser outra preferência, é uma regra a definir (ver "Quando sugerir qual ação").
- O resumo das 12:10 ainda traz o erro conhecido da História 3 ("O dia 22/11/2026 teve o maior faturamento":
  foi o menor).

## Dependências

- **História 3 (CIDB-11):** a ação vem no mesmo pedido do resumo; sem resumo, sem ação.
- **Decisão em aberto:** a lista final de ações.
- **História 6 (CIDB-26):** se a bateria passar a medir a ação sugerida, precisa de gabarito do produto.
