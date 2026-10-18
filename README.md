<p>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/assets/logos/cd2-logo-horizontal-negativo.svg">
    <img src="docs/assets/logos/cd2-logo-horizontal.svg" alt="CD2 Tech" width="200">
  </picture>
</p>

# CD2 IA de Bolso

**A IA da CD2 rodando 100% no seu computador, direto do pen drive.**

Plugou, clicou, conversou. A IA de Bolso abre uma janela com a identidade da CD2 e conversa com um modelo de IA rodando no próprio computador, sem instalar nada e sem mandar dados para a nuvem. Quando houver rede, conecta a um MCP do Archeo para consultar dados reais.

[Site do projeto](https://cd2tec.github.io/cd2-ia-de-bolso/) · [Passo a passo do pen drive](docs/criar-pendrive.md) · [Baixar cd2tech-iadebolso.zip](https://github.com/cd2tec/cd2-ia-de-bolso/releases/download/iadebolso/cd2tech-iadebolso.zip) · [Dar uma estrela](https://github.com/cd2tec/cd2-ia-de-bolso)

> **Experimental.** É um modelo pequeno, roda no seu computador e erra. Não é indicado para uso comercial. Montamos este conjunto para mostrar na prática como uma IA específica, pequena e perto do seu dado, já consegue ser útil. Pode baixar, gravar, testar e passar adiante. Se acender uma ideia aí, a [CD2](https://cd2.com.br) pode ajudar no seu projeto de IA.
>
> Siga a CD2: [LinkedIn](https://br.linkedin.com/company/cd2tech) · [Instagram](https://www.instagram.com/cd2tech/) · [GitHub](https://github.com/cd2tec) · [Falar com a CD2](https://wa.me/19982355172)

## Crie o seu pen drive em 4 passos

O jeito mais fácil é o instalador para Windows e Mac, com quatro telas: veja [O jeito mais fácil: o instalador](docs/criar-pendrive.md#o-jeito-mais-fácil-o-instalador). Os passos abaixo são o jeito técnico, com os scripts do ZIP.

Feito para Windows e Mac, sem precisar entender de tecnologia. Você só precisa de um pen drive de 16 GB ou mais (tudo nele será apagado) e de internet na primeira vez.

### [Baixar cd2tech-iadebolso.zip](https://github.com/cd2tec/cd2-ia-de-bolso/releases/download/iadebolso/cd2tech-iadebolso.zip)

1. **Baixe** o arquivo [cd2tech-iadebolso.zip](https://github.com/cd2tec/cd2-ia-de-bolso/releases/download/iadebolso/cd2tech-iadebolso.zip).
2. **Descompacte:** clique com o botão direito no arquivo baixado e escolha **Extrair tudo**. Vai aparecer a pasta `cd2tech-iadebolso`.
3. **Espete o pen drive** no computador.
4. **Grave:** abra a pasta extraída e dê dois cliques em **`GRAVAR-PENDRIVE-Windows.bat`**. Aceite o pedido de permissão do Windows, escolha o pen drive na lista e digite `APAGAR` para confirmar.

Na primeira vez ele baixa uns 3,9 GB, então leva alguns minutos, conforme a sua internet. No fim aparece "Pronto. Pode tirar o pen drive." Para usar, espete o pen drive em qualquer computador e dê dois cliques em **`IA de Bolso`**.

> Se aparecer "O Windows protegeu o computador", clique em **Mais informações** e depois em **Executar assim mesmo**. Se a janelinha da IA de Bolso não abrir, use a alternativa que fica na pasta `core`: dois cliques em `INICIAR-CD2-Windows.bat`.

No Mac, o passo 4 é dar dois cliques em `GRAVAR-PENDRIVE-Mac.command`; no Linux, rodar `sh gravar-pendrive-linux.sh`. Detalhes e problemas conhecidos em [docs/criar-pendrive.md](docs/criar-pendrive.md).

## O que tem aqui

```
raiz do pen drive (CD2)/
  IA de Bolso.exe             <- duplo clique no Windows (o lançador)
  IA de Bolso.app             <- duplo clique no Mac (o lançador)
  ia-de-bolso.sh              <- no Linux: sh ia-de-bolso.sh
  web/                        <- a interface (index.html, config.json e fontes Manrope)
  core/                       <- scripts de preparo, de gravação e de inicialização
                                 (aqui fica também INICIAR-CD2-Mac.command, o plano B do Mac)
  bin/    (criado no preparo) <- llamafile
  bin/toolbox/ (criado no preparo) <- MCP Toolbox, um por sistema
  bin/ctl/ (criado no preparo) <- cd2-ctl, o controle do banco, um por sistema
  core/toolbox/               <- receitas SQL por motor
  models/ (criado no preparo) <- modelo .gguf

Só no repositório (não vão para o pen drive):
  GRAVAR-PENDRIVE-Windows.bat <- grava um pen drive pronto, no Windows
  GRAVAR-PENDRIVE-Mac.command <- idem, no Mac
  gravar-pendrive-linux.sh    <- idem, no Linux
  docs/criar-pendrive.md      <- o passo a passo completo
  docs/index.html             <- o site do projeto (GitHub Pages)
  ctl/                        <- código do cd2-ctl (Go), compilado pelo GitHub Actions
  launcher/                   <- código do lançador IA de Bolso (Go + Wails), compilado pelo GitHub Actions
  worker/                     <- Worker da Cloudflare que abre o pagamento e assina a chave de instalação
```

## Configuração recomendada

| | Mínimo | Recomendado |
|---|---|---|
| Memória RAM | 8 GB | 16 GB |
| Processador | 64 bits: Intel/AMD de 2015 para cá (com AVX2) ou Mac M1 em diante | 8 núcleos ou mais, ou Apple Silicon |
| Placa de vídeo | Não precisa | Com GPU ou Apple Silicon fica bem mais rápido; o launcher tenta usar sozinho |
| Sistema | Windows 10/11, macOS ou Linux, sempre 64 bits | |
| Porta USB | USB 3 | USB 3 |
| Espaço no computador | Uns 600 MB livres no Mac e no Linux (cópia do motor em cache) | |
| Navegador | Edge ou Chrome, para abrir como janela de aplicativo | |
| Internet | Só no modo Archeo MCP | |

De onde vêm os números: o modelo ocupa uns 2,4 GB de memória e o contexto padrão (8.192 tokens) mais uns 1,2 GB; somando motor, toolbox e a janela do navegador, a IA de Bolso usa de 4 a 5 GB. Em 8 GB funciona, mas apertado se houver outras coisas abertas. Em notebook só com CPU, espere uns 9 segundos por pergunta. É estimativa a partir do tamanho do modelo e da avaliação em `tests/eval/`; ainda não foi medido numa máquina de 8 GB nem num processador sem AVX2.

## 1. Criar o pen drive (uma vez, com internet)

Plugue um pen drive de 16 GB (USB 3) e rode o gravador do seu sistema:

| Sistema | Como gravar |
|---|---|
| Windows | duplo clique em `GRAVAR-PENDRIVE-Windows.bat` (pede administrador) |
| Mac | duplo clique em `GRAVAR-PENDRIVE-Mac.command` |
| Linux | `sh gravar-pendrive-linux.sh` |

Ele lista os pen drives, pede confirmação (digite `APAGAR`), formata em exFAT, baixa o llamafile e o modelo para um cache no seu computador, copia tudo, confere os checksums e ejeta. O segundo pen drive não baixa nada de novo. O passo a passo, o jeito manual e a lista de problemas conhecidos estão em [docs/criar-pendrive.md](docs/criar-pendrive.md).

Se preferir fazer na mão: formate em exFAT, copie o conteúdo desta pasta para a raiz do pen drive (sem `.git`, `docs/`, `ctl/`, `tests/` e os `GRAVAR-*`) e rode o preparo (`core\prepare-windows.bat` no Windows, `sh core/prepare-unix.sh` no Mac/Linux). O preparo retoma downloads interrompidos e pula o que já estiver completo. Ele grava `CHECKSUMS.sha256`; guarde uma cópia fora do pen drive.

Depois de gravado, o pen drive **não precisa de internet**. Só o modo Archeo MCP precisa.

## 2. Usar

| Sistema | Como abrir |
|---|---|
| Windows | duplo clique em `IA de Bolso.exe` |
| Mac | duplo clique em `IA de Bolso.app` |
| Linux | `sh ia-de-bolso.sh` |

A **IA de Bolso** (o lançador) abre uma janelinha com um robô carregando o modelo e uma barra de progresso de verdade. Por trás, ela escolhe uma porta livre, sobe o servidor só em `127.0.0.1` (não fica exposto na rede) e tenta GPU, caindo para CPU sozinha. Quando a IA acorda, a conversa abre como janela de aplicativo no navegador (Edge ou Chrome no Windows; Chrome, Edge ou Brave no Mac).

Fechar a janelinha **não** encerra a IA: ela continua acordada no ícone perto do relógio (Windows) ou na barra de menus (Mac), e dali dá para trazer a janelinha de volta ou abrir outra janela da conversa. Para encerrar, use **Colocar a IA para dormir**, na janelinha ou no menu do ícone. Quando a IA dorme, a conversa mostra uma faixa avisando. Só depois de "Boa noite! Pode tirar o pen drive." é seguro ejetar. Há uma IA por computador: abrir o lançador de novo traz a janelinha da que já está rodando.

**Plano B técnico.** O `INICIAR-CD2-Windows.bat` e o `INICIAR-CD2-Mac.command` ficam em `core/`. Os dois fazem a mesma coisa pelo terminal (para encerrar: ENTER na janela do Windows, Ctrl+C ou fechar o terminal no Mac). Se o pen drive foi gravado sem o lançador de um sistema, o INICIAR dele fica na raiz. No Linux não há lançador: use `sh ia-de-bolso.sh`.

Windows e Mac modernos **não executam nada automaticamente** ao plugar o pen drive. O "plugou e abriu" é, na prática, um duplo clique. Por isso o lançador fica na raiz, com nome grande e visível; o resto fica nas subpastas.

## Instalar no computador

Gostou e quer usar a IA sem o pen drive? Com a IA aberta pelo pen drive, clique em **Instalar no computador** (na barra de cima da conversa ou na janelinha do lançador).

- **Quanto custa:** R$ 25 por computador, em pagamento único, pelo Stripe. É uma cópia avulsa para este computador, não é assinatura nem licença.
- **O que você leva:** a IA copiada do pen drive para o computador, que abre sem o pen drive, com ícone nos programas e atalho na área de trabalho se você quiser. Se quiser, ela acorda junto com o computador (ocupa de 4 a 5 GB de memória desde que ele liga, e dá para desligar). E dá para desinstalar quando quiser.
- **A chave:** chega por e-mail e na tela, depois do pagamento. Guarde. Ela reinstala no mesmo computador, sem pagar de novo.
- **As regras:** vale para um computador (todas as contas de usuário dele). Trocou de computador ou perdeu a chave, é uma nova instalação, com novo pagamento.

O teste pelo pen drive continua livre e completo. As condições completas estão em [condicoes-instalacao.html](https://cd2tec.github.io/cd2-ia-de-bolso/condicoes-instalacao.html).

## 3. Conectar a um banco de dados

O pen drive leva o [MCP Toolbox for Databases](https://github.com/googleapis/mcp-toolbox) (v1.14.0, um binário por sistema em `bin/toolbox/`) e o `cd2-ctl`, um controle pequeno feito aqui (código em `ctl/`, um binário por sistema em `bin/ctl/`). A conexão é feita pela tela: botão **Configurações**, modo **MCP Nativo para Banco de dados**, preencha motor, host, porta, base (service name no Oracle, caminho do arquivo `.fdb` no Firebird), usuário, senha e schema, e clique em **Salvar e conectar**. O toolbox reinicia sozinho com a conexão nova. Se ela falhar, a anterior continua valendo e a tela mostra o motivo e o fim do log. **Desconectar do banco** apaga a conexão deste computador.

| Motor | Porta padrão | Schema padrão | Conta de leitura recomendada |
|---|---|---|---|
| PostgreSQL | 5432 | `public` | membro de `pg_read_all_data`, sem GRANT de escrita |
| Oracle | 1521 | usuário em maiúsculas | `CREATE SESSION` + `SELECT` nas tabelas (ou `SELECT ANY TABLE`) |
| MariaDB / MySQL | 3306 | nome da base | `GRANT SELECT ON base.* TO ...` |
| SQL Server | 1433 | `dbo` | membro só de `db_datareader` |
| Firebird | 3050 | não usa | conta com só `SELECT` nas tabelas; no campo da base vai o caminho do arquivo `.fdb` |

A IA ganha sete ferramentas somente leitura: `db_connection_status`, `db_catalog` (o mapa do banco para os relatórios), `db_list_tables`, `db_describe_table`, `db_search_columns`, `db_sample_table` (até 20 linhas) e `db_read_only_query` (SELECT avulso, até 200 linhas). O botão **Configurações** mostra a conexão ativa e exporta em PDF o que a IA de Bolso entende do banco.

- **A conexão fica no computador, não no pen drive.** Arquivo `db.env` em `~/Library/Caches/cd2-harness` (Mac), `${XDG_CACHE_HOME:-~/.cache}/cd2-harness` (Linux) ou `%LOCALAPPDATA%\cd2-harness` (Windows), legível só pelo usuário. O formato é o mesmo da v0.5.0: quem já tinha uma conexão continua conectado.
- **A senha passa pelo navegador só para ser gravada**, sempre em `127.0.0.1`, protegida pela chave da sessão. Ela nunca volta para a tela (o campo mostra "Senha salva") e nunca vai para log: o `cd2-ctl` tira do `toolbox.log` toda linha que cita a senha ou a palavra password.
- **Chave da sessão.** A cada abertura o launcher sorteia 32 bytes, entrega ao `cd2-ctl` pelo ambiente e à janela pelo endereço (`#k=`). A página guarda a chave só naquela aba e limpa o endereço. Sem a chave certa, o `cd2-ctl` responde 401; ele também recusa outra origem, outro `Host` e corpo acima de 8 KB. Risco aceito: a chave aparece na linha de comando do navegador, visível para outros usuários do mesmo computador. O pen drive é pensado para notebook de uma pessoa, e a chave muda a cada abertura.
- **Use uma conta de leitura.** O toolbox não valida SQL. A IA de Bolso recusa qualquer coisa que não seja um único SELECT (no front e no próprio toolbox, com lista de palavras e funções proibidas por motor), e no PostgreSQL a sessão do toolbox já abre com `default_transaction_read_only=on`. Mesmo assim, a garantia de verdade é a permissão da conta.
- **Portas.** O launcher usa um trio que avança junto: P para a IA de Bolso, P+1000 para o toolbox e P+3000 para o `cd2-ctl` (8080, 9080 e 11080 por padrão). Todos escutam só em `127.0.0.1`.
- Se o banco não responder em 15 segundos (senha errada, firewall), a tela mostra Atenção com o motivo e as últimas linhas do log, já filtradas.
- É preciso a variante certa em `bin/toolbox/` e `bin/ctl/` (`windows-amd64`, `darwin-arm64`, `darwin-amd64`, `linux-amd64`). O preparo baixa as quatro; `CD2_TOOLBOX_PLATFORMS` limita as duas pastas. Sem o `cd2-ctl` da plataforma, o launcher avisa e segue sem banco (Demonstração e Archeo continuam).
- **Tempo máximo de 15 segundos por consulta.** PostgreSQL corta no servidor (`statement_timeout`), MariaDB/MySQL pelo `queryTimeout` da receita. No SQL Server, no Oracle e no Firebird, a IA de Bolso para de esperar aos 15 s, mas a consulta pode continuar no servidor: crie a conta de leitura com limite de recurso (Resource Governor no SQL Server, perfil com `CPU_PER_CALL` no Oracle).

Ao conectar, a IA de Bolso lê até 10 linhas de cada tabela (no máximo 2 consultas ao mesmo tempo, views por último e uma de cada vez) para entender o papel de cada uma: movimento, itens de movimento, cadastro, parâmetro ou vazia. Isso leva segundos e já melhora a escolha da tabela. Depois, em segundo plano, o modelo escreve uma frase sobre cada tabela, e uma faixa com o robô mostra o progresso; dá para ir perguntando enquanto isso. Nada disso é gravado: amostras, papéis e descrições ficam só na memória da janela e são refeitos a cada conexão. Em Configurações, o link **Exportar o que a IA de Bolso entende do banco (PDF)** gera um relatório com o papel, as colunas úteis e a descrição de cada tabela, pela janela de impressão do navegador.

## 3b. Conectar ao Archeo MCP

Funciona como configurar qualquer servidor MCP. É preciso ter uma conta no Archeo: [archeo.cd2.com.br](https://archeo.cd2.com.br).

1. Clique em **Configurações** e escolha **Archeo MCP**.
2. Informe o link do servidor MCP: `https://archeo.cd2.com.br/api/mcp`.
3. Autentique com a sua conta do Archeo (**Entrar com a conta do Archeo**) ou informe uma chave de acesso.

O acesso vale só enquanto a janela estiver aberta e nada é gravado no pen drive. Para deixar o Archeo como padrão, edite `web/config.json` (`mcp.mode` = `"mcp"` e `mcp.url`); cada pessoa ainda entra com a própria conta.

Sem internet ou sem MCP, o modo **Demonstração** funciona offline com dados fictícios, sempre rotulados como tal na resposta da ferramenta.

A Demonstração também ensina a conectar o banco (`web/demo.js`): para perguntas como "Que gráficos você faz com o meu banco?", "Como você entende o meu banco?" e "É seguro conectar?", o modelo recebe os fatos do roteiro e escreve a resposta com as palavras dele, seguida de gráficos de exemplo de vendas, ruptura e cliente. Se o modelo não responder, entra a resposta pronta. A conversa abre com uma saudação brasileira e um comentário sobre o momento (madrugada, almoço, sexta, fim de mês, estação), sorteados em `web/momento.js` pelo relógio do computador. Cada resultado de ferramenta vira gráfico na tela, e toda resposta termina com o convite para conectar o banco do ERP em Configurações.

## 3c. Relatórios e gráficos

No modo Banco de dados, cada pergunta vira uma consulta, um gráfico e um relatório. Ao conectar, a IA de Bolso lê o mapa do banco uma vez e sugere quatro perguntas tiradas do próprio esquema do cliente.

- **Pergunte do jeito que falaria:** "faturamento por mês", "top 10 produtos por valor", "quantos clientes por estado".
- **O que aparece:** título, uma frase de resumo e o visual (número, barra, linha, pizza ou tabela), com a consulta usada num bloco recolhível.
- **Os números vêm do banco, não do modelo.** O modelo escolhe a consulta e o tipo de gráfico, mas nunca vê o resultado. A frase de resumo é montada pela IA de Bolso a partir do dado.
- **Quando não dá:** a IA de Bolso diz que não achou tabelas, que não conseguiu montar a consulta, que a consulta demorou demais ou que veio vazia.
- **Confira a consulta usada.** Os números vêm do banco, mas a consulta pode estar errada. Um JOIN errado mostra um número real, só que de outra pergunta.
- **Levar embora:** **CSV** baixa os dados (abre no Excel com acentos e decimais certos). **Imprimir/PDF** gera o relatório com logo e contato da CD2 no rodapé.
- **Contato no relatório:** preencha `report.contact` em `web/config.json` (site, e-mail, telefone).

Para medir a qualidade num banco de teste com cara de ERP: `sh tests/eval/rodar.sh` (precisa de Docker). O portão para levar a cliente é 70% de acerto com o llamafile só em CPU. Na avaliação atual o modelo padrão acerta 90% das perguntas (18 de 20) e passa no portão. Em 2 de 20 ele mostrou um resultado errado, sem aviso; a avaliação marca esses casos como `[ERRADO]`. Veja `tests/eval/`.

## 4. Personalizar

- `web/config.json`: nome, frase de efeito, prompt de sistema, sugestões da tela inicial.
- `web/index.html`: a interface segue o Design System da CD2 (ver seção 5). As cores e medidas são os tokens do sistema, declarados no bloco `:root`; ajuste ali, nunca com valor solto no meio do CSS.
- Outro modelo: coloque outro `.gguf` em `models/` e deixe só um arquivo lá. O launcher usa o primeiro que encontrar.
- Contexto maior: variável `CD2_CTX` (padrão 8192; mais contexto consome mais RAM).
- `web/config.json`: `mcp.mode` aceita `toolbox` (padrão), `demo` e `mcp`; `suggestions` pode ser uma lista única ou uma por modo.
- `web/config.json`: `report.contact` é o contato que sai no rodapé do relatório impresso.

## 5. Identidade visual

Interface, site e README seguem o **Design System CD2 Tech** (Manual de Marca CD2 2026).

| Elemento | Como aparece aqui |
|---|---|
| Cores | `blue-retail` #0000ff em botões e destaques; `white-human` #f2f2f2 de fundo; texto em `ink` #0d1233. `yellow-like` #dbff2a só como energia pontual (modo Demonstração, terceiro passo do site). |
| Tema | Claro (White_HUMAN) na interface da IA de Bolso, como pede o sistema para sistemas e dashboards; azul (Blue_RETAIL) no hero e no rodapé do site. |
| Tipografia | Manrope (400 a 800), embutida em `web/fonts/` e `docs/assets/fonts/` em WOFF2 com subconjunto latino. Funciona offline. Licença SIL OFL 1.1, gravada nos próprios arquivos. |
| Logo | `cd2-logo-horizontal` colorida sobre a barra superior branca (fixa no topo, no padrão dos sites da CD2 em cd2tec.github.io); símbolo como avatar da IA e favicon, o mesmo do site do Archeo. Arquivos em `docs/assets/logos/`. |
| Componentes | Botões em pílula (primário, secundário), StatusTag para o estado do modelo e das ferramentas (Concluído = pronto, Em melhoria = Demonstração, Atenção = offline ou erro), SectionHeader com eyebrow e régua de 240px, blocos de conexão no site. |
| Movimento | Só transições de 150 ms em cor. O indicador de carregamento é uma barra que desliza; nada gira, pulsa ou salta, e ele para com "reduzir movimento". Exceção: o robô em pixel art da análise das tabelas anda em dois quadros, no estilo do robô do terminal, e também para com "reduzir movimento". No terminal, enquanto o modelo carrega, um robô em pixel art leva blocos do pen drive até um cérebro que enche com o progresso real da leitura (`core/robo-unix.sh` e `core/robo-windows.ps1`, quadros em `core/robo.txt`). Sem cor (`NO_COLOR`) ou com `CD2_SIMPLES=1`, vira texto. |

Regras de texto que valem para qualquer mudança: português do Brasil, frases curtas, sem emoji e sem travessão. A tagline de mercado "Para operações que não podem parar." é a única que vai para fora.

## 6. Limitações e cuidados

- **Experimental, não comercial.** É um conjunto para experimentar e mostrar o que uma IA local e específica consegue fazer. Não é indicado para uso comercial nem para decisões importantes sem conferência humana. A interface mostra esse aviso toda vez que abre.
- **Velocidade.** O modelo de IA padrão responde sem fase de raciocínio. No modo Banco de dados cada pergunta custa uma chamada ao modelo (duas no pior caso). Em notebook só com CPU, espere cerca de 9 segundos por pergunta, medido na avaliação em `tests/eval/`.
- **Procedência do modelo.** O arquivo do modelo de IA vem de fonte conhecida, com licença aberta, e é conferido por SHA-256 no preparo. Origem e licença estão na seção 7.
- **Nome do modelo.** Para quem usa, é só "o modelo de IA": a tela e o terminal não mostram qual é.
- **Antivírus e SmartScreen.** O llamafile é um executável "poliglota" (APE) e alguns antivírus corporativos o bloqueiam. Teste nos notebooks em que vai rodar e leve o seu como reserva.
- **Mac.** Em Apple Silicon o llamafile precisa das Xcode Command Line Tools na primeira execução. Se o macOS reclamar de "desenvolvedor não verificado", libere em Ajustes do Sistema > Privacidade e Segurança.
- **Linux.** Se der erro de `run-detectors`/WINE, é `binfmt_misc`; a doc do llamafile traz o registro do `ape`.
- **Pen drive.** exFAT não guarda permissão de execução, então Mac/Linux rodam o motor a partir de uma cópia em `~/.cache/cd2-harness` (ou `~/Library/Caches/cd2-harness`). Nada é instalado e nada fica no sistema além desse cache.
- **Oracle.** Driver Go puro: sem wallet, Kerberos ou TNS. **SQL Server** em Docker para teste é amd64; no Apple Silicon ligue Rosetta.
- **Firebird.** No campo da base vai o caminho do arquivo `.fdb` no servidor (ex.: `C:\dados\erp.fdb`); não há schema. A consulta avulsa é um único SELECT, sem WITH (use subconsulta), e a lista de tabelas vem sem contagem de linhas, que o catálogo do Firebird não guarda.
- **Modelo pequeno erra.** O aviso na tela é proposital. Em números do Archeo, o dado vem da ferramenta; o texto em volta é do modelo.

## 7. Licenças e origem dos componentes

Este código é Apache 2.0 (ver `LICENSE`). A IA de Bolso baixa, mas não redistribui no repositório:

- **Manrope** (The Manrope Project Authors, SIL Open Font License 1.1), embutida em subconjunto em `web/fonts/` e `docs/assets/fonts/`
- **llamafile** (Mozilla.ai, Apache 2.0), de https://github.com/mozilla-ai/llamafile
- **MCP Toolbox for Databases** (Google, Apache 2.0), de https://github.com/googleapis/mcp-toolbox
- **cd2-ctl** (código deste repositório, Apache 2.0, pasta `ctl/`), compilado pelo GitHub Actions e baixado do release `ctl-v<versão>` deste repositório
- **Modelo GGUF**: o padrão é o Qwen3-4B-Instruct-2507 (Qwen, Apache 2.0), quantizado em Q4_K_M pela Unsloth e baixado de https://huggingface.co/unsloth/Qwen3-4B-Instruct-2507-GGUF; a Qwen não publica o GGUF oficial deste modelo. É configurável em `CD2_MODEL_URL`, sempre junto com `CD2_MODEL_FILE` (o nome do arquivo em `models/`). Com outro modelo, confira licença e origem antes de distribuir os pen drives sob a marca da CD2.

## Para você que é dev

O código é aberto, sob Apache 2.0: launchers, interface, o `cd2-ctl` com a guarda de leitura do banco e os scripts do pen drive. Fuça à vontade.

- **Gostou?** [Deixe uma estrela no repositório](https://github.com/cd2tec/cd2-ia-de-bolso). É o jeito mais simples de dizer que vale continuar.
- **Siga a CD2 no GitHub:** [github.com/cd2tec](https://github.com/cd2tec), onde moram este e os outros projetos abertos da CD2.
- **Achou um bug ou teve uma ideia?** [Abra uma issue](https://github.com/cd2tec/cd2-ia-de-bolso/issues) ou mande um pull request.
