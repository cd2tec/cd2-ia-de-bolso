# Planilhas do Excel como banco

Data: 23/10/2026. Status: rascunho para aprovação.

## Por que

Muito comércio pequeno não tem ERP: o controle de vendas, estoque, clientes e contas vive numa pasta de planilhas do Excel. Hoje a IA de Bolso só conversa com banco de sistema (PostgreSQL, Oracle, MariaDB, SQL Server, Firebird) ou com uma pasta de arquivos `.dbf`. Esse público fica de fora.

A ideia é a mesma do DBF: a pessoa aponta a pasta das planilhas em Configurações e sai perguntando, "quanto vendi em setembro", "quais clientes estão devendo". Para a IA, para as notas, para as consultas salvas e para os relatórios, é só mais um banco.

## Objetivos

- Um motor novo, **Planilhas Excel**, no mesmo formulário de Configurações, com um campo: a pasta das planilhas.
- Ler `.xlsx` e `.xlsm`. Cada aba vira uma tabela. Planilha "bem comportada" (cabeçalho na primeira linha) e planilha "viva" (título em cima, cabeçalho na linha 3, linha de total embaixo, abas ocultas) entram sem a pessoa arrumar nada.
- O **↻ Atualizar com o banco** que já existe passa a reler a pasta antes de comparar o catálogo, nas planilhas e também no DBF (decisão dele: hoje no DBF o botão nunca encontra arquivo novo, porque a cópia só é refeita ao reconectar). Não nasce botão novo.
- Tudo que já funciona em cima do catálogo (perfil, descrições, notas, consultas salvas, relatórios, PDF do entendimento) funciona igual.
- A pessoa vê em Configurações o que ficou de fora e por quê.

## Fora do escopo

- `.xls` antigo (binário do Excel 97-2003) e `.csv`. Quando a pasta só tiver esses, a tela diz para abrir no Excel e salvar como `.xlsx`.
- Perceber sozinha que um arquivo mudou e reler antes de responder. A pessoa clica em Atualizar.
- Interpretar número escrito como texto ("R$ 1.234,50" digitado numa célula de texto) ou data escrita como texto ("3/9/26"). Fica texto.
- Subpastas. Só os arquivos da pasta apontada, como no DBF.
- Planilhas do Google Sheets, OneDrive online ou qualquer coisa que não seja arquivo local.

## Anti-objetivos (falhou, mesmo que funcione)

- **Escrever qualquer coisa na planilha da pessoa.** Leitura só, como no DBF. Nem recalcular, nem salvar, nem criar arquivo na pasta dela.
- **Seguir link externo de pasta de trabalho** (planilha que puxa dado de outra na rede ou da internet). A leitura usa o valor que está gravado na célula e não busca nada fora.
- **Copiar uma aba "no chute" com colunas sem nome e deixar a IA se virar.** Decisão dele: aba sem cabeçalho reconhecível fica de fora, listada.
- **Perder nota ou consulta salva por causa de uma releitura.** A releitura que falha não mexe na cópia que está no ar.
- **Travar a tela ou o Excel da pessoa.** Arquivo aberto no Excel continua sendo lido; se não der para ler naquele instante, entra na lista dos ignorados e volta na próxima atualização.

## Como fica para a pessoa

### Configurações

No seletor de motor aparece **Planilhas Excel** (valor interno `excel`), depois de DBF. Ao escolher, o rótulo do campo da base vira **Pasta das planilhas**, com o exemplo `ex.: C:\Controle da loja`. Host, porta, usuário, senha e schema ficam como no DBF: "nao usado", porta 0, schema vazio.

**Salvar e conectar** lê a pasta e sobe o toolbox. Falhas de pasta aparecem no mesmo lugar e com o mesmo tom das do DBF:

- "Não achei a pasta das planilhas."
- "A base das planilhas é uma pasta com os arquivos .xlsx, não um arquivo."
- "Não achei planilha .xlsx ou .xlsm nessa pasta." Se houver `.xls` ou `.csv` na pasta, a frase ganha a dica: "Encontrei arquivos .xls/.csv, que ainda não leio. Abra cada um no Excel e salve como .xlsx."
- "Nenhuma aba das planilhas dessa pasta tem um cabeçalho que eu reconheça." (todas ignoradas)

Abaixo do status da conexão, quando o motor é Excel, entra uma linha **Planilhas lidas: 4 arquivos, 9 abas viraram tabelas** e, se houver, um `details` recolhido **Abas deixadas de fora (3)** com uma linha por aba: arquivo, aba e motivo em português. Motivos possíveis:

| Motivo mostrado | Quando |
|---|---|
| sem cabeçalho que eu reconheça | nenhuma linha passou no critério de cabeçalho |
| aba oculta | aba marcada como oculta no Excel |
| aba vazia | sem nenhuma célula preenchida |
| o arquivo não abriu | zip corrompido, meio gravado, bloqueado naquele instante |
| arquivo protegido por senha | pasta de trabalho criptografada |

Arquivos com nome começando em `~$` (a trava do Excel aberto) não aparecem nem na lista de ignorados.

### Perguntar

Nada muda na conversa. A IA recebe, na linha do motor que já descreve o dialeto, o aviso de que as tabelas vêm de planilhas: cada aba é uma tabela, o nome é `arquivo__aba`, as colunas têm os nomes que a pessoa escreveu no cabeçalho, pode não haver chave ligando uma aba à outra, e o dialeto é SQLite com `LIMIT` e data em texto `AAAA-MM-DD`.

O aviso "Este banco não parece ser de um ERP de varejo" **não é dado** quando o motor é Excel. Planilha de controle não é ERP, o aviso dispararia para todo mundo e não diria nada útil.

### ↻ Atualizar com o banco

Quando o motor é de arquivo (Excel ou DBF), o botão ganha o título "Reler os arquivos e trazer tabelas e colunas novas, sem perder as notas" (nas planilhas: "Reler as planilhas e trazer abas e colunas novas…"), e a janela ganha uma etapa antes de "lendo": **relendo**, com o texto "Relendo as planilhas…" ou "Relendo os arquivos .dbf…". Nessa etapa a tela pede ao `cd2-ctl` que releia a pasta. O toolbox reinicia (alguns segundos). Se a releitura falhar, a janela mostra o erro na etapa de erro que já existe, com a frase "Nenhuma nota foi apagada", e a cópia anterior continua no ar. Se der certo, o fluxo segue exatamente como hoje: lê o catálogo, compara com o entendimento guardado, mostra o resumo, confirma.

A linha "Planilhas lidas" e a lista de ignorados em Configurações refletem a última releitura. No DBF não há linha nem lista: o leitor de DBF continua sem relatório.

Reconectar (Salvar e conectar) e abrir a IA de Bolso de novo também releem tudo, como no DBF. O entendimento guardado é reaproveitado quando as abas e colunas são as mesmas; só aba nova é descrita de novo.

## Como fica por dentro

### O caminho é o do DBF

O DBF já faz o essencial: o `cd2-ctl` copia os arquivos para um SQLite na pasta de cache e o toolbox sobe com a receita de SQLite ([core/toolbox/dbf.yaml](../../../core/toolbox/dbf.yaml)) e as sete ferramentas de leitura. O Excel reaproveita isso:

- Pacote novo `ctl/internal/excel`, irmão de [ctl/internal/dbf](../../../ctl/internal/dbf/import.go), com `Import(pasta, destino) (Relatorio, error)`.
- `dbconf`: `excel` entra em `Engines`, `defaultPorts` (0), na mensagem de motor e no `check`, com a mesma regra do DBF (base obrigatória, sem caractere de controle, demais campos ignorados).
- Supervisor: `Start` chama `excel.Import` quando `cfg.Engine == "excel"`, a cópia fica em `excel.sqlite` na pasta de cache, e o ambiente leva `CD2_DB_SQLITE` como no DBF.
- Receita `core/toolbox/excel.yaml`, cópia da do DBF com as descrições falando em planilhas e abas e `engine` devolvendo `excel` em `db_connection_status`. Minha escolha: dois arquivos em vez de um gerado, porque a receita é texto lido por gente e o custo da duplicação é baixo. Se o construtor preferir gerar as duas de um molde no preparo do pen drive, pode, desde que o arquivo final tenha o nome `excel.yaml` e o supervisor continue escolhendo `engine + ".yaml"`.
- Preparo do pen drive e instalador: nada a fazer. Não há lista de receitas em lugar nenhum: o `cd2-ctl` recebe a pasta `core/toolbox` inteira como `RecipeDir`, e o instalador copia a pasta `core` inteira. Basta o arquivo `core/toolbox/excel.yaml` existir.

### Biblioteca

[excelize](https://github.com/xuri/excelize) v2, em Go puro, para `.xlsx` e `.xlsm`. Mesma lógica da go-dbase no DBF: sem dependência de sistema, um binário por plataforma. Macros são ignoradas. Fórmulas entram pelo valor calculado que o Excel gravou na célula; célula de fórmula sem valor gravado (arquivo nunca recalculado) vira vazia.

### Arquivos e abas

- Entram os arquivos da pasta com extensão `.xlsx` ou `.xlsm`, sem distinguir maiúsculas. Ficam de fora: subpastas, nomes começando em `~$` ou `.`, qualquer outra extensão. Ordem alfabética, como no DBF.
- De cada arquivo entram as abas **visíveis**, na ordem do arquivo. Aba oculta vai para a lista de ignorados com o motivo "aba oculta".
- Arquivo que não abre (corrompido, meio gravado, com senha) **não derruba a importação**: vai para a lista de ignorados com uma linha por arquivo e a importação segue. Isso é diferente do DBF, que para no primeiro arquivo ruim. Motivo: numa pasta de controle é normal ter um arquivo estragado ou sendo salvo naquele segundo, e a pessoa não pode ficar sem nada por causa de um.
- A importação só falha por inteiro quando não sobra nenhuma tabela.

### Nome da tabela

- Arquivo com **uma** aba visível: `identificador(nome do arquivo sem extensão)`. Ex.: `clientes.xlsx` → `clientes`.
- Arquivo com **duas ou mais** abas visíveis: `identificador(arquivo) + "__" + identificador(aba)`. Ex.: `vendas 2026.xlsx`, aba `Setembro` → `vendas_2026__Setembro`.
- Caixa preservada como a pessoa escreveu. Colisão resolvida com `_2`, `_3`, como hoje.
- Consequência aceita: criar a segunda aba num arquivo que tinha uma só renomeia a tabela (`clientes` vira `clientes__Plan1`), e as notas dela seguem a regra de tabela que sumiu, com o aviso que a janela de Atualizar já dá. Preferi isso a dar sempre o nome composto, que deixaria `clientes__Plan1` em todo arquivo simples.

### Identificador com acento

O `identificador` de hoje apaga letra acentuada ("Descrição" vira "Descrio"). Para planilha, onde o cabeçalho é texto de gente, isso não serve. O identificador passa a **transliterar**: remove o acento e mantém a letra (`Descrição` → `Descricao`, `Preço (R$)` → `Preco_R`, `Nº` → `N`), usando `golang.org/x/text` (já é dependência) para decompor (NFD, não NFKD: `Nº` vira `N`, não `No`) e tirar as marcas. Espaço, hífen, ponto e barra viram `_`; parêntese, `$`, `%` e o resto somem; underscores repetidos colapsam; nome vazio ou começando com dígito ganha `T_`; limite de 60 caracteres. A função vira compartilhada entre `dbf` e `excel` (pacote pequeno, por exemplo `ctl/internal/ident`). No DBF o efeito é nulo na prática: nome de campo DBF é ASCII.

### Cabeçalho, linhas e colunas de uma aba

Regra determinística, sem modelo:

1. **Procura do cabeçalho**, nas primeiras 30 linhas da aba. Uma linha é candidata quando: tem pelo menos 2 células preenchidas; **pelo menos 2** delas são texto; e a próxima linha preenchida, a no máximo 2 linhas de distância, tem pelo menos uma célula preenchida dentro das colunas do cabeçalho. A primeira candidata é o cabeçalho. Célula de cabeçalho que é número ou data (comum em planilha de comparação: "Produto | 2024 | 2025", "Vendedor | 01/09 | 02/09") vira nome de coluna pelo texto formatado que o Excel mostra, passado pelo identificador (`2024` → `T_2024`, `01/09` → `T_01_09`). Escolha minha, depois da revisão: a regra "só texto" perderia essas abas inteiras, e preferi errar para o lado de ler demais. Título mesclado em cima ("Controle de Vendas 2026") costuma ter uma célula só e não passa; uma linha de rótulos tipo "Loja: Centro | Mês: Setembro" pode passar, e aí vira cabeçalho errado. Aceito o risco nesta versão: é raro e a pessoa vê o resultado em Configurações e nas colunas da IA.
2. **Largura da tabela:** da primeira à última coluna preenchida da linha de cabeçalho. Célula de cabeçalho vazia dentro desse intervalo (comum em cabeçalho mesclado) vira `Coluna_<letra do Excel>`, ex.: `Coluna_D`, para o nome não mudar quando a pessoa insere linha.
3. **Linhas de dados:** todas as linhas abaixo do cabeçalho até a última preenchida, exceto: linha vazia dentro da largura; linha em que a primeira célula preenchida é texto que, sem acento e em minúsculas, começa com `total` ou `subtotal` (minha escolha para a linha de soma do fim, afinável); e célula fora da largura, que é ignorada. Não para no primeiro buraco: gente deixa linha em branco entre blocos.
4. **Tipo da coluna**, olhando todas as células preenchidas da coluna:
   - todas número → `NUMERIC`, ou `INTEGER` se todas inteiras;
   - todas data (número com formato de data ou data/hora do Excel) → `DATE`, gravada como texto `AAAA-MM-DD`, ou `AAAA-MM-DD HH:MM:SS` quando houver hora, igual ao DBF;
   - todas booleano → `INTEGER` 0/1;
   - qualquer mistura → `TEXT`, com número e data escritos como o Excel mostra (o texto formatado da célula).
   - Percentual fica como fração (15% → 0.15). Moeda fica número sem símbolo. Célula de erro (`#N/A`, `#DIV/0!`) vira vazia.
5. Célula mesclada: só a célula de cima à esquerda tem valor; as outras ficam vazias, como o Excel guarda.
6. Aba sem nenhuma candidata a cabeçalho → ignorada com "sem cabeçalho que eu reconheça". Aba sem célula preenchida → "aba vazia".

### Relatório da importação

`Import` devolve `Relatorio{Arquivos int, Tabelas int, Ignoradas []Ignorada{Arquivo, Aba, Motivo string}, LidoEm time.Time}` além do erro. O controlador guarda o último relatório em memória e o `GET /status` o expõe num campo novo `planilhas` (omitido nos outros motores). A tela monta a linha "Planilhas lidas" e o `details` a partir dele. Nada disso vai para o `db.env` nem para disco.

O relatório **também vem quando a importação falha por inteiro** (todas as abas ignoradas): é justamente aí que a pessoa precisa saber qual aba caiu por quê. Então o `Start` devolve o relatório junto do `StartError`, o controlador o guarda no estado `falhou` (e no `rodando` com a conexão anterior, depois do rollback, o relatório é o da tentativa que falhou, como já acontece com `erro` e `log`), e `renderToolbox` desenha a linha e o `details` sempre que `planilhas` vier no status, em qualquer estado. Escolha minha, depois da revisão.

### Releitura sem derrubar a cópia que está no ar

Hoje o `dbf.Import` apaga o destino e grava por cima. Para o Atualizar não deixar a pessoa sem banco quando a releitura falha, a gravação passa a ser em duas etapas, para os dois motores de arquivo:

1. Importar para `<destino>.novo` (apagando um `.novo` esquecido de antes).
2. Só com a importação concluída: parar o toolbox, renomear `.novo` por cima do destino (Windows não deixa renomear com o SQLite aberto pelo toolbox, por isso parar antes), subir o toolbox.

Endpoint novo no `cd2-ctl`: `POST /reler`, sem corpo, com a mesma chave de sessão e as mesmas proteções de origem dos demais. Só responde para motor de arquivo (`excel` e `dbf`). Para outro motor ou sem conexão: 409 com "Esta conexão não é de arquivos." Devolve o `Status` como o `POST /config`. Falha de importação: mantém o toolbox anterior rodando e devolve o status com `erro` preenchido. Falha ao subir o toolbox de novo: segue o `rollback` que já existe. Um pedido por vez, pelo mesmo mutex `op` do `Apply`.

Na tela, `abrirAtualizar` para motor de arquivo (`excel` ou `dbf`): etapa `relendo` → `ctlRequest('POST', '/reler')` com tempo de espera folgado (o construtor mede; o `Start` do DBF já leva a cópia inteira dentro do prazo do `/config`) → se `erro`, etapa de erro com a frase do `cd2-ctl` e "Nenhuma nota foi apagada." → senão, o toolbox é um processo novo: a página reabre a sessão MCP com ele **sem descartar** `st.catalog`, `st.entendimento` e as notas em memória, e segue para a etapa `lendo` de hoje. O construtor localiza em [web/index.html](../../../web/index.html) o ponto em que a sessão MCP é refeita depois do `saveDb` e separa "reabrir sessão" de "recomeçar o entendimento", que hoje andam juntos.

### Front

- [web/toolbox.js](../../../web/toolbox.js): `excel` em `ENGINES` (rótulo "Planilhas Excel", porta 0), placeholders (`base: 'ex.: C:\\Controle da loja'`, host e usuário "nao usado"), `defaultsFor` (schema vazio), `configPayload` igual ao DBF.
- [web/ask.js](../../../web/ask.js): entrada `excel` no mapa de dialetos, igual à do DBF mais a frase sobre abas e nomes `arquivo__aba`; mesma exclusão de schema que `dbf`.
- [web/index.html](../../../web/index.html): `<option value="excel">Planilhas Excel</option>`, rótulo "Pasta das planilhas" em `syncDbForm`, a linha e o `details` de planilhas lidas em `renderToolbox`, a etapa `relendo` na janela de Atualizar, o título do botão, e a verificação de varejo pulada quando `st.ctlStatus.motor === 'excel'` (gravando no entendimento que o aviso não se aplica, para não ser refeito).
- [web/entendimento.js](../../../web/entendimento.js): `pareceVarejo` não muda; quem pula é a página. A identificação do banco para notas e entendimento (motor + base) já serve: motor `excel` e a pasta.
- Modo Demonstração: sem mudança.

### README, site e versão

- README, seção 3: na linha do DBF e no parágrafo do Atualizar, dizer que o botão relê a pasta. Linha da tabela de motores ("Planilhas Excel | não usa | não usa | sem servidor: no campo da base vai a pasta com os arquivos .xlsx/.xlsm; cada aba vira uma tabela; a leitura copia e não altera a planilha") e um parágrafo curto sobre o que entra e o que fica de fora (cabeçalho, total, abas ocultas, .xls e .csv) e sobre o Atualizar reler as planilhas.
- [docs/index.html](../../../docs/index.html), card "Os bancos mais comuns": acrescentar "e planilhas do Excel (.xlsx), para quem ainda não tem sistema". Print, se houver um novo, segue a regra de dados fictícios.
- `VERSION`: 0.21.0 (funcionalidade nova).

## Testes que o construtor deve cobrir

Go, em `ctl/internal/excel`, com arquivos gerados pela própria excelize no teste:

- Planilha bem comportada: cabeçalho na linha 1, 3 colunas (texto, número, data), 2 linhas. Tabela com o nome do arquivo, tipos `TEXT`/`NUMERIC`/`DATE`, data em `AAAA-MM-DD`.
- Planilha viva: título mesclado na linha 1, linha em branco, cabeçalho na linha 3 com uma célula vazia no meio, 4 linhas de dados com uma em branco no meio, linha "Total" no fim. Cabeçalho achado, `Coluna_C` no buraco, 4 linhas, total fora.
- Arquivo com duas abas visíveis e uma oculta: nomes `arquivo__aba`, a oculta na lista de ignorados.
- Aba só com gráfico ou só com texto solto (nenhuma linha candidata): ignorada com o motivo certo.
- Cabeçalho com acento e símbolo: `Descrição` → `Descricao`, `Preço (R$)` → `Preco_R`.
- Coluna mista (número e texto) vira `TEXT` com o número como texto.
- Fórmula com valor gravado entra pelo valor.
- Arquivo `~$teste.xlsx` e `.xls` na pasta: ignorados; pasta só com `.xls` dá a mensagem com a dica de salvar como `.xlsx`.
- Cabeçalho com ano nas colunas ("Produto | 2024 | 2025"): aba lida, colunas `Produto`, `T_2024`, `T_2025`.
- Arquivo corrompido (bytes aleatórios com extensão `.xlsx`) não derruba a importação.
- Pasta em que todas as abas são ignoradas: erro na subida e o relatório com os motivos chega ao `/status`.
- Gravação em duas etapas: destino anterior intacto quando a importação falha.

Go, em `control` e `api`: `POST /reler` com motor excel (sucesso e falha de importação mantendo o toolbox anterior), com motor dbf (sucesso), com motor postgres (409), sem conexão (409), sem chave (401).

JS, em `tests/`: `toolbox.test.js` (defaults, placeholders e payload do motor `excel`), `ask.test.js` (dialeto `excel` sem schema, com a frase de abas) e, se a janela de Atualizar tiver lógica testável fora do DOM, a etapa `relendo`.

## O que fica para o construtor

- Como a excelize informa formato de data da célula e visibilidade de aba na versão escolhida, e o tratamento de data/hora com fuso (a planilha não tem fuso; gravar como está).
- O tempo de espera do `/reler` na tela e se vale uma mensagem de progresso por arquivo numa pasta grande.
- Limite de linhas por aba, se algum. Minha sugestão: nenhum limite duro nesta versão; a cópia é feita em memória como no DBF, e uma planilha de 200 mil linhas cabe.
- Organização do código do leitor (um arquivo ou vários) e nomes internos.
