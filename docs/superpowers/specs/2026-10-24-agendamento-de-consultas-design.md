# Agendamento de consultas salvas

Data: 24/10/2026. Status: rascunho para aprovação.

## Por que

As consultas salvas ([2026-10-21-notas-e-consultas-salvas-design.md](2026-10-21-notas-e-consultas-salvas-design.md)) rodam direto no banco, sem o modelo, quando a pessoa clica. Há coisas que ela quer acompanhar ao longo do dia sem precisar lembrar de clicar:

- **Flash de vendas no horário de pico:** quanto vendeu hoje até agora, de hora em hora.
- **Monitoramento de estoque e ruptura:** o que zerou, toda manhã antes de a loja abrir.

O agendamento faz a consulta salva rodar sozinha em horários definidos. O resultado avisa a pessoa e fica guardado num histórico ligado à consulta.

Há também um motivo comercial, o mesmo das notas: **o agendamento é um recurso da versão instalada.** No pen drive ele aparece, e o clique leva para a compra.

## Objetivos

- Agendar qualquer consulta salva com uma frequência: a cada X minutos, toda hora, a cada X horas, todo dia (num horário), a cada X dias, semanal (dias marcados), a cada 2 semanas, mensal (todo dia X do mês).
- Parâmetros de período, calculados a cada execução: hoje, ontem, hora atual, últimas X horas, últimos X dias, esta semana, semana passada, este mês, mês passado, dia da semana de hoje. Os parâmetros que não são de período ficam com o valor fixo escolhido ao agendar.
- Cada execução vira um card de conversa (título, valores dos parâmetros, visual, frase, CSV, Imprimir/PDF), guardado junto da consulta salva. **O histórico é guardado para sempre.**
- Toda execução avisa a pessoa: notificação do sistema (Windows e Mac) e marca de "novo" no ícone da IA de Bolso na bandeja.
- Execução perdida (a IA estava fechada no horário) entra no histórico como "Não rodou às HH:MM", com o botão **Rodar agora**.
- Sempre dá para rodar qualquer consulta manualmente, a qualquer hora, agendada ou não. A execução manual de uma consulta agendada também entra no histórico.
- **Execução manual** (*minha*): o card aparece onde a pessoa clicou (na conversa, como hoje, ou no histórico) e também é gravado no histórico. Não gera notificação nem marca de NOVO, porque foi a própria pessoa que pediu. Passa pelo mesmo executor dos agendamentos, então respeita a regra de uma por vez.

## Fora do escopo

- Rodar com a IA de Bolso fechada: nada de serviço do sistema nem tarefa agendada do Windows ou do Mac.
- Mandar o resultado para fora do computador (e-mail, WhatsApp, Slack).
- Aviso condicional ("só avise se tiver linha" ou "se passar de X"). O aviso sai sempre.
- Agendar perguntas para o modelo. Só a consulta salva roda, e sempre sem o modelo.

## Anti-objetivos (falhou, mesmo que funcione)

- **Rodar sozinha, sem a pessoa pedir, as execuções que foram perdidas.** Quando a IA reabre, as perdidas viram aviso e só rodam pelo botão.
- **Pesar no banco do ERP do cliente no horário de pico.** Ver o intervalo mínimo e a regra de "uma por vez".
- **Perder o histórico sem aviso.** Apagar a consulta salva apaga o histórico, com confirmação que diz quantas execuções vão junto.
- **Afrouxar a segurança da consulta salva.** O valor do período calculado passa pelo mesmo caminho dos parâmetros digitados ([web/consultas.js](../../../web/consultas.js)): validação por tipo, substituição na posição do literal e depois a checagem de leitura do `db_read_only_query`.

## Onde funciona

| | Pen drive | Instalada |
|---|---|---|
| Ver "⏰ Agendar…" e "Ver histórico" | sim | sim |
| Agendar e rodar sozinha | **não**: o clique abre o convite de compra | sim |
| Rodar manualmente | sim (como hoje) | sim |

- O convite no pen drive é uma janela curta ("Agendamento é da versão instalada", com dois exemplos de uso) com os botões **Agora não** e **Instalar no computador**. O segundo abre o fluxo de compra que já existe (`abrirInstalar` em [web/index.html](../../../web/index.html)).
- Como as consultas salvas, o agendamento não aparece no modo Demonstração.

## Quando roda

- **Só com a IA de Bolso aberta.** Não precisa de janela de conversa na frente: basta o programa rodando, com o ícone na bandeja (a janela fechada pelo X continua viva, ver [launcher/janela.go](../../../launcher/janela.go)). A opção "Acordar quando o computador ligar", que já existe, é o jeito de a pessoa garantir que a IA esteja aberta.
- **Quem dispara não pode depender da janela.** Hoje a página da conversa pode nem existir (a IA acordou com o computador e ninguém abriu a janela). O relógio dos agendamentos fica no processo que está sempre vivo enquanto a IA está aberta. Ver "O que fica com o construtor".
- **Execuções perdidas:** quando a IA abre, ela confere, para cada agendamento ligado, os horários que passaram desde a última vez que esteve aberta. **Cada intervalo fechado vira uma linha só no histórico (decisão dele):** "Não rodou às 12:00" quando foi um horário, ou "Não rodou 812 vezes entre sex 20:00 e seg 08:00 — a IA de Bolso estava fechada". O botão **Rodar agora** roda o último horário perdido do intervalo. Nada roda sozinho.
- **Horário que passou com a IA aberta mas sem conseguir rodar também é perdido, nunca roda atrasado sozinho (decisão dele).** Vale para o computador que dormiu com a IA aberta (o notebook fechado às 7h e aberto às 9h não roda o das 7h30) e para um agendamento que esperou na fila mais que o próprio intervalo. A regra é uma só: o horário é a hora certa ou não roda. *Minha:* uma folga de 1 minuto conta como "na hora".
- **"Colocar a IA para dormir" para os agendamentos, igual a Fechar (decisão dele).** O item do menu avisa: "Colocar a IA para dormir (os agendamentos param)". Os horários do sono viram perdidos quando ela acorda.
- **Motivo da perda, na linha do histórico** (*minha*): "a IA de Bolso estava fechada" (inclui dormindo e o pen drive aberto em vez da instalada), "o computador estava dormindo", "outro banco estava conectado", "a anterior ainda estava rodando". Banco fora do ar não é perda: é execução com erro.
- **"Rodar agora" numa execução perdida usa o horário perdido** para calcular os períodos. Um "Hoje" das 12h de 24/10, rodado às 15h ou no dia seguinte, continua sendo 24/10. "Hora atual" vira 12h. O card diz "Rodou às 15:04 (referente a 12:00)". A execução entra como nova no histórico, e a linha da perda ganha "rodada às 15:04" (*minha*).
- Só os agendamentos do banco conectado rodam. As consultas salvas já são separadas por banco. A "última vez aberta" é guardada por banco (*minha*).

## Frequência

Na janela "Agendar consulta":

- **A cada X minutos**, **a cada X horas** e **toda hora**, com uma janela de horário opcional (das 08:00 às 20:00) e os dias da semana marcados.
- **Todo dia** num horário, ou **a cada X dias** a partir de uma data.
- **Semanal:** dias marcados e horário, toda semana ou a cada 2 semanas.
- **Mensal:** todo dia X do mês num horário, ou o último dia do mês.
- Um resumo em português no rodapé ("Vai rodar de segunda a sábado, de hora em hora, das 08:00 às 20:00, com o período de hoje. Próxima: hoje, 15:00.") e a caixa **Agendamento ligado**, para pausar sem perder a configuração.
- Os horários seguem o relógio do computador. Mês sem o dia X (31 em fevereiro, por exemplo) roda no último dia do mês.

## Parâmetros

- Cada parâmetro da consulta salva aparece na janela de agendar com o valor que vai receber em cada execução.
- **Parâmetro de data:** escolhe um período (lista do 📅) ou uma data fixa.
- **Parâmetro de número ou texto:** valor fixo, preenchido com o da última vez.
- Período que é uma faixa ("esta semana", "últimos 7 dias") precisa de dois parâmetros de data na consulta, o inicial e o final. A janela sugere o par: escolhendo "Esta semana" no inicial, o final vira "fim desta semana". Com um só parâmetro de data, a lista só oferece os períodos que são um dia ("hoje", "ontem").
- "Hora atual" e "últimas X horas" só valem quando o literal da consulta é data com hora. Hoje o parâmetro de data com hora cobre só a data ([web/consultas.js](../../../web/consultas.js)). **Minha:** entra um tipo novo de parâmetro, "data e hora", no formato da consulta salva, nas janelas de salvar e de rodar e na validação da importação. É o mesmo caminho para todos, sem um atalho só do agendamento.
- **Como cada período vira valor** (*minha*): todo período é um par início/fim. Com um parâmetro só, usa o início. "Hora atual" vai do início da hora até agora. "Últimas X horas" vai de agora menos X até agora. A semana vai de domingo a sábado, como no calendário brasileiro. "Esta semana" e "este mês" terminam no último dia do período, e "semana passada" e "mês passado" são o período inteiro anterior.
- "Dia da semana de hoje" é um parâmetro de número ou texto, conforme o banco grava o dia da semana. O construtor decide como a pessoa diz qual é o formato.

## Histórico

- Abre clicando numa consulta agendada na barra lateral, ou em "Ver histórico" no menu "⋯". Aparece na área da conversa, com "← Voltar para a conversa".
- No topo: o nome, a frequência, **Editar agendamento** e **▶ Rodar agora**.
- Execuções da mais nova para a mais antiga, separadas por dia. Cada uma diz o horário, se foi **agendada** ou **manual**, e os valores dos parâmetros usados.
- Só a execução mais nova abre com o resultado à mostra. As outras ficam fechadas ("▾ Mostrar resultado").
- As execuções agendadas ainda não vistas têm a marca **NOVO**. A marca some quando a pessoa abre o histórico.
- O histórico carrega aos poucos conforme a pessoa rola, porque é guardado para sempre.
- Execução que deu erro (banco fora do ar, SQL recusado) entra no histórico com a mensagem de erro e o botão **Rodar agora**, e avisa como as outras.
- **O que se guarda de cada execução:** o resultado (as linhas), para redesenhar o card igual a qualquer momento, com CSV e Imprimir/PDF. Não basta guardar o SQL: os dados mudam.

## Barra lateral

- A consulta agendada mostra ⏰ com a frequência e a próxima execução ("a cada 1 h · próxima 15:00"), além de uma bolinha com quantas execuções ainda não foram vistas.
- Filtro **Todas / ⏰ Só agendadas (N)** acima da lista.
- O menu "⋯" ganha **▶ Rodar agora**, **⏰ Agendar…** e **🗂 Ver histórico**, junto de Renomear e Apagar.
- Apagar uma consulta agendada pede confirmação: "O histórico de N execuções também será apagado."

## Avisos

- **Notificação do sistema** a cada execução: "IA de Bolso · Flash de vendas de hoje", com a frase do card (ou "N linhas" quando não há frase, ou a mensagem de erro). Clicar abre a janela no histórico, já na execução nova.
- **Marca no ícone da bandeja** com o total de execuções não vistas. O menu do ícone ganha, no topo, "N resultados novos" e uma linha por agendamento com novidade ("⏰ Flash de vendas de hoje (2)"). Clicar numa delas abre o histórico daquele agendamento.
- "Fechar" e "Colocar a IA para dormir", no menu do ícone, avisam que os agendamentos param: "Fechar (os agendamentos param)".
- Execuções perdidas não geram notificação, uma por uma, quando a IA reabre. Gera uma só: "N execuções não rodaram enquanto a IA estava fechada".

## Proteções

- **Intervalo mínimo de 5 minutos.**
- **Uma execução por vez em cada agendamento.** Se a anterior ainda estiver rodando quando chegar a hora, a nova é pulada e registrada como "Não rodou: a anterior ainda estava rodando".
- **Agendamentos diferentes no mesmo horário rodam um depois do outro**, não ao mesmo tempo, para não abrir várias conexões no banco do ERP.
- A mesma proteção de tempo e de linhas que a consulta salva já tem.

## Exportar e importar

- A exportação de notas e consultas ganha o agendamento de cada consulta (frequência, parâmetros e ligado/desligado). **O histórico não é exportado.**
- Ao importar, o agendamento vem **desligado**, para não sair rodando num computador novo sem a pessoa conferir.

## Esboços

[.superpowers/brainstorm/2298-1792864395/content/agendamento-v1.html](../../../.superpowers/brainstorm/2298-1792864395/content/agendamento-v1.html): barra lateral, janela de agendar, histórico, avisos e o convite do pen drive. Seguir as cores e os componentes de [web/index.html](../../../web/index.html).

## O que fica com o construtor

- **Onde fica o relógio dos agendamentos.** Precisa estar vivo com a janela fechada e rodar o SQL sem a página. O caminho natural é o cd2-ctl, que já guarda as consultas salvas e fala com o banco. Nesse caso a lógica de montar o SQL com os parâmetros ([web/consultas.js](../../../web/consultas.js)) precisa existir também lá, ou o construtor acha outro jeito de ter uma regra só.
- **Como o lançador recebe "rodou"** para disparar a notificação e atualizar a marca da bandeja. A notificação do Windows já tem base em [launcher/notificacao.go](../../../launcher/notificacao.go). A do Mac é nova.
- Formato do arquivo do histórico e das rotas. Guardar na mesma pasta de cache das consultas salvas, por banco. Pensar no tamanho: guardado para sempre, a cada 5 minutos.
  - Decidido: o histórico fica num SQLite só (`historico.sqlite`, separado por banco dentro dele) em `~/Library/Application Support/cd2-harness` no Mac (o `Caches` pode ser limpo) e na pasta do cache no Windows; `--dados` no cd2-ctl troca a pasta.
- Como saber "desde quando a IA estava fechada" para achar as perdidas.
- Sinal de pen drive x instalada: o mesmo que já esconde o Importar.
- Testes no padrão de [tests/](../../../tests/) e dos `_test.go`.

## Decisões minhas (para ele conferir)

- Intervalo mínimo de 5 minutos.
- Uma execução por vez em cada agendamento, e agendamentos diferentes um depois do outro.
- Execução manual de uma consulta agendada também entra no histórico.
- Execução com erro entra no histórico e avisa.
- Execuções perdidas viram uma só notificação ao reabrir.
- A frase do card vai na notificação.
- Só a execução mais nova abre com o resultado à mostra. O histórico carrega aos poucos.
- Mês sem o dia X roda no último dia do mês.
- O histórico não é exportado, e o agendamento importado vem desligado.
- Apagar a consulta apaga o histórico, com confirmação.
- Só os agendamentos do banco conectado rodam.
- Folga de 1 minuto para contar como "na hora". Os motivos de perda listados em "Quando roda".
- "Rodar agora" numa perda cria uma execução nova e marca a perda como rodada.
- Tipo novo "data e hora" no formato da consulta salva, valendo para todos os caminhos.
- Períodos como par início/fim, semana de domingo a sábado, "esta semana" e "este mês" até o fim do período.
- Execução manual: aparece onde foi clicada, entra no histórico, sem aviso e sem NOVO, pelo mesmo executor.
