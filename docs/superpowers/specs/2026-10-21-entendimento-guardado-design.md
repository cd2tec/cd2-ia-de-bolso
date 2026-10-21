# Entendimento guardado e "Atualizar com o banco"

Data: 21/10/2026. Status: rascunho para aprovação. Continua [2026-10-21-notas-e-consultas-salvas-design.md](2026-10-21-notas-e-consultas-salvas-design.md), na mesma branch (`notas/notas-e-consultas-salvas`).

## Por que

Ele testou a tela "Entendimento do banco" e viu três problemas:

1. A tela abria antes da varredura terminar, com tabelas faltando e "A IA ainda está lendo esta tabela".
2. A caixa da nota da tabela vinha vazia. Ele quer que venha com o que a IA já entendeu, para a TI só corrigir.
3. O banco do cliente muda (tabelas e campos novos), e não havia como trazer isso sem perder as notas.

Por trás disso há um fato do código atual: **o entendimento não é guardado.** A cada abertura a IA relê o catálogo (`loadCatalog`), amostra todas as tabelas, refaz o perfil e o modelo descreve tudo de novo (`analisarBanco` em [web/index.html](../../../web/index.html)). Com a regra do item 1, o botão sumiria por muitos minutos em toda abertura. Por isso o entendimento passa a ser guardado.

## Decisões dele

- O botão "Entendimento do banco" **só aparece depois de o entendimento do banco estar completo.**
- A caixa "Nota da TI sobre a tabela" **vem preenchida com o texto da IA** e a TI pode alterar.
- Um botão para **atualizar com o banco** (tabelas e campos novos) **sem perder as notas**.
- Ao atualizar, **só as tabelas novas** são descritas pela IA. As que já tinham descrição ficam como estão.
- Tabela ou campo que **sumiu do banco: a nota é apagada, avisando o usuário.**
- Esboços aprovados ("nada, pode seguir"): `.superpowers/brainstorm/8514-1792621823/content/ressincronizar-v1.html`.

## Objetivos

- O entendimento (papel, colunas úteis e descrição de cada tabela, mais a lista de campos) é feito uma vez por banco e guardado.
- Nas aberturas seguintes, ele vem do arquivo guardado, sem amostrar nem chamar o modelo, e o botão aparece logo.
- "↻ Atualizar com o banco" compara o banco com o guardado, entende só o que é novo e limpa o que sumiu, com aviso.

## Fora do escopo

- Redescrever tabelas que já têm descrição (nem automático nem por botão).
- Descrição de campos pelo modelo (a IA continua descrevendo só tabelas).
- Levar o entendimento na exportação ou importação. O arquivo de exportação continua só com notas e consultas.

## Anti-objetivos

- Apagar nota sem a pessoa ver antes o que vai ser apagado.
- Refazer a varredura inteira a cada abertura.
- Uma atualização mexer em nota da TI ou em descrição que já existia.

## Telas (esboço aprovado)

### 1. Primeira vez num banco

- O botão "Entendimento do banco" fica escondido até a varredura terminar (perfil de todas as tabelas e a fila de descrições do modelo concluída, mesmo que o modelo tenha falhado em alguma).
- A faixa de progresso que já existe ganha a frase "O botão "Entendimento do banco" aparece quando terminar."
- Ao terminar, o entendimento é gravado e o botão aparece.
- Perguntas continuam funcionando durante a varredura, como hoje.

### 2. Tela de notas

- **A caixa "Nota da TI sobre a tabela" vem com a nota da TI, se existir; senão, com a descrição da IA**, e o rótulo ganha "(veio da IA, corrija se precisar)" quando o texto é o da IA.
- **O que estiver na caixa ao salvar é a nota da tabela** (decisão minha). Uma caixa que a pessoa não tocou não vira nota da TI: só o que foi alterado e salvo conta. Assim a lista e o prompt não passam a marcar como "nota da TI" o texto que a IA escreveu.
- "O que a IA entendeu" continua acima, só leitura, com a frase original da IA.
- Na lista, cada tabela mostra "texto da IA" ou "📝 nota da TI" (no lugar de "sem nota"/"tem nota").
- No topo: "↻ Atualizar com o banco" e, no resumo, "atualizado em DD/MM/AAAA".
- Os campos continuam com caixas vazias até a TI escrever (a IA não descreve campos).

### 3. "↻ Atualizar com o banco"

- Relê o catálogo e compara com o guardado.
- Janela com resumo: tabelas novas, campos novos, o que sumiu.
- Se algo **com nota** sumiu: aviso laranja ("estas tabelas e campos não existem mais no banco. As notas deles vão ser apagadas. Não dá para desfazer."), tabela com o que sumiu e a nota riscada, e o botão "Apagar notas e atualizar". Sem nota afetada, o botão é "Atualizar".
- Lista as consultas salvas que leem uma tabela que sumiu ("continuam salvas, mas podem dar erro"). Elas **não** são apagadas.
- Se nada mudou: "O entendimento já está igual ao banco."
- Confirmado: grava o entendimento novo, apaga as notas do que sumiu (pela fila única de gravação das notas) e descreve em segundo plano **só as tabelas novas** (e as que ficaram sem descrição porque o modelo falhou antes, decisão minha). A faixa de progresso aparece de novo, **mas o botão e a tela continuam disponíveis** (decisão minha, aprovada junto com o esboço).
- Campo novo numa tabela que já existe entra no catálogo e na grade de notas, mas a descrição da tabela não é refeita.

## Abertura com o banco já mudado (decisão minha)

Ao abrir, o catálogo vem do banco ao vivo, como hoje, e o entendimento guardado é aplicado às tabelas que existem.

- Tabelas novas entram no catálogo para as perguntas (sem descrição) e a tela mostra "O banco tem N tabelas que o entendimento ainda não conhece. Clique em Atualizar com o banco." Nada é descrito sozinho.
- Tabelas que sumiram não aparecem no catálogo. As notas delas só são apagadas pelo botão Atualizar, com o aviso.

## Onde guardar

- **Documento separado das notas**, por banco, gravado pelo cd2-ctl ao lado do arquivo de notas (decisão minha). O entendimento de um banco grande (milhares de tabelas, com a lista de campos) passaria do limite de 1 MB do documento de notas e deixaria pesado cada "Salvar notas".
- Rotas novas no mesmo padrão das de notas (`GET/POST /entendimento`), com a mesma checagem de Host, Origin, chave da sessão e **chave do banco** (409 se o banco conectado mudou). Limite próprio, generoso (o construtor escolhe; algo como 16 MB).
- Conteúdo por tabela: nome, papel, colunas úteis (data, valor, quantidade), descrição e a lista de nomes de campos; mais a data da última atualização. Nada de amostra de dados (linhas do cliente não vão para o disco).
- A importação e a exportação de notas não tocam nesse documento.

## Regras de detalhe (decisões minhas, depois da revisão)

- **Quando a caixa vira nota da TI:** só quando o texto, sem espaços nas pontas, é diferente da descrição da IA. Devolver exatamente o texto da IA, ou apagar tudo e salvar, remove a nota da TI, e a caixa volta a mostrar o texto da IA. Isso mantém o formato do documento de notas e as regras de exportação e importação como estão.
- **Prompt, ranking e PDF:** continuam como na feature anterior. A nota da TI vem marcada e antes da descrição da IA, as duas convivem, e o corte do teto de 2000 segue a mesma ordem.
- **"Completo":** a varredura terminou (amostras, perfil e fila de descrições), mesmo que o modelo tenha falhado ou não esteja disponível. O documento guarda, por tabela, se ela tem descrição. O botão Atualizar tenta descrever as que ficaram sem, e "O entendimento já está igual ao banco." só aparece quando não há mudança no banco **e** não há tabela sem descrição para tentar de novo.
- **Comparação de tabelas e campos:** pelo nome, sem diferenciar maiúsculas de minúsculas. Mudança só de tipo não conta como campo novo nem como sumiço, e a nota fica.
- **Tabela nova vista na abertura:** entra no catálogo sem perfil (sem papel), como uma tabela "indefinida" hoje, até o Atualizar. Ela continua podendo ser escolhida nas perguntas.
- **Sem expiração:** não há aviso de "faz tempo". A data "atualizado em" fica visível no topo da tela.
- **Falha ao gravar o entendimento no fim da varredura** (ctl fora, 409, 413): o botão aparece mesmo assim, usando o que está na memória da página, com uma nota curta na conversa ("Não consegui guardar o entendimento; ele vai ser refeito na próxima abertura."). Na próxima abertura, a varredura é refeita.
- **Bancos que já têm notas e nenhum entendimento** passam pelo fluxo de primeira vez, sem mexer nas notas.
- **Versão:** sobe para 0.18.0 em todos os lugares que andam juntos.

## Aviso de banco que não parece ERP de varejo (pedido dele)

A IA de Bolso foi pensada para ERP de varejo. No fim do **primeiro** entendimento de um banco, a IA tenta reconhecer se é um ERP de varejo. Se não parecer, mostra **uma única vez** uma mensagem na conversa avisando que o banco não parece um ERP de varejo e que a IA de Bolso foi feita para funcionar melhor com ERP de varejo.

Decisões minhas:

- **Texto:** "Este banco não parece ser de um ERP de varejo. A IA de Bolso foi feita para funcionar melhor com ERPs de varejo: ela vai tentar ajudar mesmo assim, mas pode errar mais." Aparece como aviso na conversa, sem bloquear nada.
- **Uma vez por banco:** o documento de entendimento guarda que a verificação já foi feita (com o resultado). Ela não roda de novo nas aberturas seguintes nem no "Atualizar com o banco". Se o entendimento não puder ser gravado e for refeito na próxima abertura, o aviso pode aparecer de novo; isso é aceito.
- **Como reconhecer:** lógica determinística, sem chamada extra ao modelo, sobre nomes de tabelas, nomes de campos e **as descrições que o modelo já fez**. As descrições contam porque há ERPs de varejo com nomes de tabela em código (ex.: `XYZ_M45`), em que só a descrição revela "itens vendidos no cupom". A lógica procura grupos de sinais do varejo (produto/item/EAN/SKU; venda/cupom/PDV/NFC-e; estoque; loja/filial; cliente/fornecedor; preço/custo; caixa/operador/finalizadora/forma de pagamento), em português e inglês, e conta também se há tabela de papel `movimento`. Sem nome de ERP específico, como em [web/perfil.js](../../../web/perfil.js).
- **Na dúvida, não avisa.** O aviso só aparece quando quase nenhum grupo de sinais aparece. Um aviso errado para um cliente que usa ERP de varejo é pior que deixar de avisar quem não usa. O limite exato fica com o construtor, com testes de catálogos de varejo (com nomes claros e com nomes em código mais descrições) que **não** podem gerar aviso, e de catálogos claramente de outro ramo (por exemplo um sistema de clínica ou de RH) que geram.

## O que fica com o construtor

- Formato exato do documento e das rotas; reaproveitar o pacote `ctl/internal/notas` (Store por pasta) se fizer sentido.
- Como reaplicar o perfil guardado no catálogo para que ranking, sugestões, prompt e PDF funcionem como hoje.
- Como a fila de descrições (`CD2Descricao.executarFila`) recebe só as tabelas novas.
- Testes no padrão de [tests/](../../../tests/) e de `ctl/internal/api`.

## Decisões minhas (para ele conferir)

- Só o que a TI alterou e salvou vira "nota da TI"; o texto da IA na caixa, intocado, não.
- Botão e tela continuam disponíveis enquanto as tabelas novas são descritas depois de uma atualização.
- Atualizar também tenta descrever tabelas que ficaram sem descrição porque o modelo falhou antes.
- Ao abrir com o banco mudado, nada é descrito nem apagado sozinho: aparece o convite para atualizar.
- Entendimento num documento separado das notas, com rotas próprias, fora da exportação.
- Consultas salvas que usam tabela que sumiu só são listadas no aviso, nunca apagadas.
