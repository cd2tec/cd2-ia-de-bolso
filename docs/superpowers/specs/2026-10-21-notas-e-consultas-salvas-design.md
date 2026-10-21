# Notas da TI e consultas salvas

Data: 21/10/2026. Status: rascunho para aprovação.

## Por que

A IA de Bolso entende o banco sozinha: o perfil determinístico ([web/perfil.js](../../../web/perfil.js)) e a descrição feita pelo modelo ([web/descricao.js](../../../web/descricao.js)) dizem o papel e o conteúdo de cada tabela. Só que o modelo é pequeno (Qwen3 4B) e há coisas que o banco não mostra: qual de três tabelas de venda é a oficial, que `status = 9` é cancelado, que `vlr_aux` é o desconto. Quem sabe disso é a pessoa de TI do cliente.

Duas coisas resolvem isso com o uso:

1. **Notas da TI** por tabela e por campo, que entram no que o modelo recebe. A IA erra menos.
2. **Consultas salvas** com um joinha num resultado certo. Rodam de novo direto no banco, sem a IA. A pessoa usa menos a IA para descobrir de novo o que já descobriu, e a ferramenta vai ficando do jeito dela.

Há também um motivo comercial: **o trabalho acumulado é um empurrão para comprar a versão instalada.** No pen drive (degustação) a pessoa precisa conseguir testar tudo (notas, joinha, consultas salvas) para experimentar bem e decidir comprar, sem perder o que já fez. Mas o trabalho só passa para outro computador pela versão paga.

## Objetivos

- A TI escreve notas por tabela e por campo, aos poucos, enquanto usa.
- As notas melhoram as respostas: entram no prompt e na escolha das tabelas.
- Todo resultado pode ser salvo com 👍. As salvas ficam numa barra lateral com busca e rolagem e rodam sem o modelo.
- Consultas salvas aceitam parâmetros, pedidos antes de rodar.
- Exportar e importar o trabalho (notas e consultas), com as regras de versão abaixo.

## Fora do escopo

- **Anexar arquivos** (PDF, MD, TXT). Fica para depois.
- Sincronizar entre computadores, nuvem ou várias pessoas editando juntas.
- Guardar o trabalho no próprio pen drive. Ver anti-objetivos.

## Anti-objetivos (falhou, mesmo que funcione)

- **O pen drive virar o jeito de levar o trabalho de um computador para outro.** Nada de gravar notas no pen drive e nada de importar no pen drive. O pen drive pode exportar (para quem vai instalar não perder nada), mas só a versão instalada importa.
- **Perder o trabalho da TI sem aviso.** Uma importação que sobrescreve tem que mostrar o que muda e pedir confirmação.
- **Consulta salva rodando valor digitado sem tratamento dentro do SQL.**
- **Estourar o prompt do modelo** com notas longas e piorar as respostas.

## Regras por versão (decididas por ele)

| | Pen drive | Instalada |
|---|---|---|
| Escrever notas | sim | sim |
| 👍 e consultas salvas | sim | sim |
| Exportar | sim | sim |
| Importar | **não** (o botão não aparece) | sim |

- No pen drive, o trabalho fica no computador, na pasta de cache onde o cd2-ctl já grava o `db.env` (`~/Library/Caches/cd2-harness` no Mac, `$XDG_CACHE_HOME/cd2-harness` no Linux, equivalente no Windows).
- **Ao instalar no mesmo computador, o trabalho feito no pen drive tem que estar lá na versão instalada.** Pelo que vi no código, as duas versões usam a mesma pasta de cache, então deve acontecer sem cópia. O construtor confirma (inclusive no Windows) e, se não for assim, a instalação copia.
- Para outro computador: exportar, instalar lá e importar.

## Telas (aprovadas nos esboços)

Esboços em `.superpowers/brainstorm/39594-1792614092/content/` (`telas-v1.html`, `parametros-v2.html`, `exportar-importar-v1.html`). Seguir as cores e os componentes de [web/index.html](../../../web/index.html).

### Tela principal

- Botão novo no topo: **"Entendimento do banco"**, ao lado de Nova conversa e Configurações. Só aparece com banco conectado (no modo Demonstração, não).
- **Barra lateral à esquerda: "Consultas salvas (N)"**, com campo de busca (por nome) e rolagem. Cada item mostra o nome, o tipo de visual e a data em que foi salva.
- No card de resposta ([renderAnswer](../../../web/index.html)), um botão **"👍 Salvar consulta"** ao lado de CSV e Imprimir/PDF. Depois de salva, vira **"👍 Salva"**.
- Clicar numa consulta salva roda o SQL direto no banco, pelo mesmo `db_read_only_query` e com as mesmas proteções que a pergunta usa hoje, **sem chamar o modelo**. O resultado entra na conversa como um card normal, com o título da consulta, os valores dos parâmetros logo abaixo, o mesmo visual, x e y, e a frase de molde (`frase()` de [web/ask.js](../../../web/ask.js) já é determinística). O card tem CSV e Imprimir/PDF.
- Dá para renomear e apagar uma consulta salva na barra lateral.

### Janela "Salvar consulta" (ao dar 👍)

- Campo **Nome**, preenchido com o título do resultado.
- O SQL aparece com os **valores fixos destacados**: literais de data, número e texto entre aspas (inclusive dentro de `LIKE '%...%'`).
- Tabela dos valores achados: caixa de marcar, valor, "Perguntar como" (nome amigável) e tipo (data, número ou texto, deduzido do valor). Os que devem mudar a cada vez a pessoa marca. Os demais ficam fixos.
- **"+ Adicionar parâmetro"** para o que a detecção não achou: selecionar um trecho do SQL com o mouse ou digitar o valor exatamente como aparece. Dar o nome e clicar em Adicionar. Se o valor não existir no SQL, avisar e não adicionar.
- Sem nenhum valor fixo, a parte de parâmetros não aparece.

### Janela de rodar consulta com parâmetros

- Um campo por parâmetro, já preenchido com os valores da última vez, e o botão **Rodar**. Sem parâmetros, o clique roda direto.
- Validar pelo tipo antes de rodar: data válida, número válido. O texto vai com aspas escapadas (`'` vira `''`). O valor substitui o literal original na posição dele no SQL. Nunca concatenar texto cru. Depois disso o SQL passa pela mesma checagem de leitura que já existe.

### Tela "Entendimento do banco" (opção A)

- Topo: "← Voltar para a conversa", título, resumo (motor, base, quantas tabelas, quantas com nota) e os botões **Exportar** e **Importar** (Importar só na instalada).
- Esquerda: busca por tabela ou campo, e a lista de tabelas com o papel (movimento, itens, cadastro...) e a marca "tem nota". Mesma ordem do perfil hoje.
- Direita, a tabela escolhida:
  - nome, papel e tamanho;
  - **"O que a IA entendeu"**: a descrição atual, só leitura (se ainda estiver sendo feita em segundo plano, dizer isso);
  - **Nota da TI sobre a tabela**: caixa de texto;
  - grade de campos: nome, tipo e **nota da TI** para cada campo;
  - **Salvar notas**, com o aviso "Salvo neste computador".

### Exportar e importar

- **Exportar** baixa um arquivo JSON (ex.: `ia-de-bolso-notas-loja01-2026-10-21.json`) com as notas das tabelas, as notas dos campos e as consultas salvas **do banco conectado**, mais a identificação do banco e a data.
- **Importar** (só na instalada) abre a escolha do arquivo e depois uma janela com:
  - resumo: notas novas, consultas novas, quantas vão ser sobrescritas;
  - se houver conflito, um aviso laranja e uma tabela com cada item em conflito, o texto atual riscado e o do arquivo embaixo;
  - **A importação junta:** o que só existe aqui continua aqui, o que só existe no arquivo entra.
  - **Conflito (o mesmo item preenchido dos dois lados): apagar e sobrescrever com o que vem do arquivo (decisão dele), com aviso em tela e confirmação.** O botão é "Sobrescrever e importar". Sem conflito, é só "Importar".
  - Nota vazia não é exportada, então nunca apaga uma nota daqui.
- Se o arquivo for de um banco com outra identificação, avisar antes do resumo, porque as tabelas podem não bater.

## Como as notas entram no modelo

Em [buildMessages](../../../web/ask.js) cada tabela vira uma linha com tamanho, papel, descrição e colunas. As notas entram aí:

- **Nota da tabela** ao lado da descrição. Quando há nota da TI, ela vem antes da descrição da IA e é marcada como nota da TI, para pesar mais.
- **Nota do campo** junto do campo, na lista de colunas.
- As notas também entram na **escolha das tabelas** em [rankTables](../../../web/catalog.js), do mesmo jeito que `descricaoTokens` entra hoje.
- **Coluna com nota da TI sempre entra** entre as colunas escolhidas por `pickColumns` para uma tabela que foi escolhida. Sem isso a nota nunca chegaria ao modelo.
- **Teto total de notas por pergunta:** cerca de 2.000 caracteres somando tudo. Se passar, corta primeiro as notas de campo das tabelas com pontuação menor, depois a descrição da IA das tabelas que têm nota da TI. A nota da TI sobre a tabela nunca é cortada. O número exato pode ser ajustado nos testes com o modelo.
- O mesmo tratamento de texto que a descrição já tem: sem quebra de linha e sem caractere de controle. A nota é dado, não instrução (o prompt de sistema já diz isso para os resultados das ferramentas).

## Onde e como guardar

- Arquivo por banco na pasta de cache, gravado e lido pelo cd2-ctl por rotas novas da API dele (o front não grava em `localStorage`, que perde tudo quando o cache do navegador é limpo ou a porta muda).
- O banco é identificado pela conexão: motor, servidor (em minúsculas, sem outra normalização), porta, base e schema efetivo (no Oracle, o usuário quando o schema está vazio; no PostgreSQL, `public`; no SQL Server, `dbo`). No DBF, a pasta. Sem senha nem nada secreto no arquivo nem na exportação. Trocar o servidor de IP para nome conta como outro banco, e quem quiser as notas de volta exporta e importa.
- Notas e consultas salvas de um banco não aparecem quando outro banco está conectado.

## O que fica com o construtor

- Formato exato do arquivo e das rotas do cd2-ctl. Proteger a escrita como as outras rotas (chave do ctl).
- Como o front sabe se está no pen drive ou na instalada (já existe o sinal que mostra a tag "rodando do pen drive").
- Detecção dos literais no SQL, inclusive datas em formatos de cada motor.
- Testes no padrão de [tests/](../../../tests/).

## Decisões minhas (para ele conferir)

- **Tamanho das notas:** até 300 caracteres na nota de tabela e 120 na de campo, com contador na caixa. O motivo é o prompt do modelo de 4B.
- **Consulta salva com o mesmo nome** na importação conta como conflito e é sobrescrita, igual às notas.
- **Importar junta**, e só o conflito é sobrescrito. Nota vazia não é exportada.
- **Coluna com nota sempre vai para o prompt** quando a tabela dela foi escolhida, e há um **teto de cerca de 2.000 caracteres** de notas por pergunta, com a ordem de corte descrita acima.
- **Identificação do banco:** motor, servidor, porta, base e schema efetivo. No DBF, a pasta.
- **Aviso a mais** quando o arquivo importado vem de outro banco.
- **Renomear e apagar** consultas salvas na barra lateral.
- O PDF "o que a IA de Bolso entende do banco" que já existe passa a incluir as notas da TI.
- O botão "Entendimento do banco" e a barra lateral não aparecem no modo Demonstração.
