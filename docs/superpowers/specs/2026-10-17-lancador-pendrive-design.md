# Lançador do pen drive (Windows e Mac): design

Data: 2026-10-17. Status: rascunho para aprovação.

## Por que

Hoje, para usar a IA de Bolso, a pessoa espeta o pen drive e dá dois cliques em `INICIAR-CD2-Windows.bat` ou `INICIAR-CD2-Mac.command`. Abre um terminal com o robô em texto, que carrega o modelo, sobe o servidor e o `cd2-ctl` e abre o Edge ou o Chrome como janela de aplicativo. Para encerrar, é ENTER no console do Windows, ou Ctrl+C ou fechar o Terminal no Mac.

Funciona, mas a primeira coisa que a pessoa vê é um terminal, e o terminal precisa ficar aberto o tempo todo. Para o público de negócio, um terminal assusta, e fechar a janela errada mata a IA sem aviso.

O objetivo é trocar o `.bat` e o `.command` por um lançador de verdade: um `.exe` no Windows e um `.app` no Mac, na raiz do pen drive, com uma janelinha no pixel art da IA de Bolso e um ícone perto do relógio (Windows) ou na barra de menus (Mac).

## Objetivos

- Dois cliques no lançador abrem uma **janelinha**, sem terminal nenhum, que mostra o carregamento e depois abre a conversa no navegador, como hoje.
- A janelinha pode ser fechada a qualquer momento, durante ou depois do carregamento, **sem encerrar a IA**. A IA continua num ícone perto do relógio ou na barra de menus.
- Encerrar fica explícito e com cara de despedida: **"Colocar a IA para dormir"**.
- Visual na mesma família do resto: pixel art do robô e do cérebro (`core/robo.txt`), cores e fonte da CD2.

## Não objetivos

- Abrir a conversa dentro do próprio lançador. A conversa continua no Edge ou no Chrome, em modo janela de aplicativo, como hoje (decisão do Gabriel).
- Linux. Continua com `iniciar-cd2-linux.sh`.
- Mudar a interface da conversa (`web/`), o `cd2-ctl`, o modelo ou as portas. A única exceção é o aviso de "IA dormindo" descrito no passo 5.
- Assinar o lançador com certificado (Apple Developer ID, notarização, certificado de código no Windows). Mesma decisão do instalador.
- Distribuir o lançador como `.dmg`. O `.dmg` é para baixar e instalar; no pen drive o certo é o `.app` direto na raiz, que sabe onde está o pen drive. O `.dmg` continua sendo só o formato do instalador.

## Antiobjetivos (seria um fracasso mesmo funcionando)

- A pessoa ver um terminal, um caminho de arquivo ou uma mensagem em inglês do sistema sem tradução.
- Fechar a janelinha encerrar a IA, ou a IA ficar rodando sem nenhum sinal visível (o ícone sempre existe enquanto a IA estiver acordada).
- Sobrar processo rodando depois de "Colocar a IA para dormir": o motor, o `cd2-ctl` e o toolbox são todos encerrados.
- Abrir dois lançadores e subir duas IAs. Um segundo duplo clique traz a janelinha da IA que já está rodando.
- Perder alguma garantia que os scripts de hoje dão (escuta só em `127.0.0.1`, chave da sessão, queda da GPU para a CPU, aviso quando o `cd2-ctl` não sobe).

## A experiência (decidida)

Os rascunhos aprovados estão em `.superpowers/brainstorm/` (fora do git), arquivo `lancador-mix.html`. O estilo é o mix aprovado: a cena "hacker" de fundo escuro da opção A, as piadas da opção B e o ícone de bandeja/barra de menus da opção C.

### 1. Dois cliques: carregando

Janelinha pequena (por volta de 320 px de largura), fundo escuro `#0d1233`, título "CD2 IA de Bolso". Dentro:

- A cena do terminal de hoje em pixel art: o pen drive à esquerda, o robô andando com o bloco amarelo e o cérebro à direita, que vai enchendo conforme o progresso, dormindo.
- Uma barra de progresso amarela (`#dbff2a`) e uma linha em fonte monoespaçada: "carregando o modelo de IA ... 62%". O progresso é real: é a leitura do modelo em pedaços, como o `robo_ler` faz hoje.
- Embaixo, uma piada da lista do mascote (`installer/frontend/dist/mascote.js`, `PIADAS`), com um `> ` na frente, trocando a cada poucos segundos sem repetir a anterior.
- Os avisos de hoje aparecem como uma linha nessa janela, em português simples. Exemplo: "Aceleração por GPU indisponível, tentando só com CPU...".

### 2. Ficou pronta

- O cérebro acorda (rosto `acordado` do `robo.txt`, piscando) e o título vira "Acordei! A conversa abriu no navegador."
- A conversa abre sozinha no Edge ou no Chrome, com a mesma regra de hoje: modo `--app`, maximizada, com a URL `?v=<versão>#k=<chave>`.
- A janelinha fica aberta com uma piada e dois botões:
  - **Abrir uma nova janela da conversa** (primário, amarelo): abre outra janela do navegador na mesma IA, com a mesma chave.
  - **Colocar a IA para dormir** (secundário, contorno).

### 3. Fechou a janelinha (durante ou depois do carregamento)

- A IA não para. A janelinha some e fica o ícone, que é o cérebro em pixel art, perto do relógio no Windows e na barra de menus no Mac.
- O carregamento continua normalmente com a janelinha fechada. Quando termina, a conversa abre no navegador do mesmo jeito.
- O menu do ícone tem:
  - um cabeçalho com o estado ("IA de Bolso · acordada" ou "IA de Bolso · carregando 62%");
  - **Mostrar a IA de Bolso**;
  - **Abrir uma nova janela da conversa** (apagado enquanto ainda carrega);
  - **Colocar a IA para dormir**.
- **Windows:** clique simples no ícone traz a janelinha de volta; o botão direito abre o menu. **Mac:** o clique abre o menu, que é o padrão de lá. (Esta divisão é uma escolha minha que o Gabriel viu no rascunho e aprovou.)
- O ícone pode mudar conforme o estado (cérebro meio cheio e dormindo enquanto carrega, cheio e acordado depois). Fica a critério de quem constrói.

### 4. Chamou de volta (clicou no ícone ou em "Mostrar")

- Se a IA já estiver acordada, a janelinha volta sempre com o cérebro acordado, o título "Me chamou?" e uma piada de outra lista, a de **quem foi chamada** (exemplo: "Tava aqui no cantinho do relógio, contando os segundos. Literalmente."), com os mesmos dois botões.
- Se ainda estiver carregando, a janelinha volta mostrando o carregamento onde ele está.
- A lista das piadas de chamada é nova e precisa ser escrita no tom da IA de Bolso. Os exemplos dos rascunhos são só ponto de partida.

### 5. Colocou para dormir (pelo botão ou pelo menu)

- O cérebro dorme (rosto `sono`, com um "z" subindo), aparecem "Boa noite! Pode tirar o pen drive." e uma piada de despedida.
- O lançador encerra o motor, o `cd2-ctl` (que derruba o toolbox) e o ícone. A janelinha some sozinha em uns 3 segundos.
- A frase "pode tirar o pen drive" só aparece depois que tudo já foi encerrado de fato.
- **As janelas da conversa no navegador continuam abertas** (decisão do Gabriel). O navegador segue no perfil normal da pessoa, para não perder o histórico nem o login do Archeo, que moram no `localStorage`. A conversa percebe que a IA parou de responder e mostra uma faixa: "A IA está dormindo. Abra o lançador do pen drive para acordá-la." Essa é a única mudança no `web/`. Como a conversa percebe isso (uma checagem periódica do servidor ou a falha da próxima pergunta) fica a critério de quem constrói, desde que a faixa apareça sem a pessoa precisar perguntar nada.

### Erros

- O que hoje é `[ERRO]` no terminal vira uma tela da janelinha com o cérebro triste ou com a cor de erro da CD2, a frase do problema em português e um botão para fechar. Exemplos: "O modelo de IA não está neste pen drive. Grave o pen drive de novo.", "A IA não conseguiu acordar."
- Os logs continuam nos mesmos lugares de hoje (`%TEMP%\cd2-harness`, `~/Library/Caches/cd2-harness`). A tela de erro pode oferecer "Abrir a pasta dos registros", mas não mostra o caminho por extenso.

## Como fica o pen drive (decidido)

- Na raiz, o lançador do Windows e o do Mac, com nomes grandes e claros. Sugestão: `IA de Bolso.exe` e `IA de Bolso.app` (o nome final fica a critério de quem constrói, desde que seja curto e igual nos dois).
- O `INICIAR-CD2-Windows.bat` e o `INICIAR-CD2-Mac.command` **saem da raiz**, porque o pedido é que o lançador fique no lugar deles. Continuam existindo dentro de `core/` como plano B técnico, documentados em `docs/criar-pendrive.md`. Isso é uma escolha minha.
- O `iniciar-cd2-linux.sh` continua na raiz.
- `README.md`, `docs/criar-pendrive.md`, `docs/index.html` e o texto da release passam a falar do lançador e não do `.bat`.

## Como é feito

### Decidido

- **Go.** O repositório já usa Go no `cd2-ctl` e no instalador, e já compila para Windows, Mac Intel e Mac Apple Silicon no CI. A janelinha é HTML/CSS/JS, como o instalador, para reaproveitar o pixel art e o estilo.
- **A lógica de `core/start-windows.ps1` e `core/start-unix.sh` é portada para Go dentro do lançador**, sem chamar os scripts por baixo. Assim não abre terminal, e o progresso e os avisos chegam à janelinha. Isso inclui:
  - o trio de portas livres (P, P+1000, P+3000, a partir de 8080, avançando juntas até 20 tentativas);
  - a chave da sessão de 32 bytes, entregue ao `cd2-ctl` só pelo ambiente e à janela só pela URL (`#k=`);
  - a cópia do `llamafile` e do `cd2-ctl` para o cache no Mac, sem o atributo de quarentena;
  - a tentativa com GPU (`-ngl 999`), com queda para CPU (`--gpu disable -ngl 0`) e o log da GPU guardado;
  - a espera pelo servidor (até 240 s por tentativa);
  - a subida e a conferência do `cd2-ctl` (401 = no ar), com o aviso e a seguida sem banco se ele falhar;
  - a escolha do navegador (no Windows, Edge e depois Chrome; no Mac, Chrome, Edge, Brave e Chromium, com o tamanho da área útil da tela; navegador padrão em último caso);
  - o encerramento de tudo (no Windows, a árvore de processos; no Mac, TERM e, depois de 10 s, KILL).
- **Os scripts de start não são apagados.** Ficam como plano B e para o Linux. Os dois caminhos precisam continuar dando o mesmo resultado.
- **Uma IA por computador.** Se o lançador for aberto de novo com a IA já rodando, ele não sobe outra; traz a janelinha da que está rodando.

### Riscos para conferir logo no começo (antes de construir o resto)

1. **Janela e ícone de bandeja juntos.** O Wails v2, usado no instalador, não tem ícone de bandeja. As bibliotecas de bandeja e as de janela costumam disputar a thread principal no Mac. Quem constrói escolhe a combinação (Wails v3, Wails v2 com uma biblioteca de bandeja, outra biblioteca de janela...), mas precisa provar num protótipo descartável, no Windows e no Mac, que a janela abre, fecha para a bandeja e volta pelo ícone.
2. **O `.app` rodando direto de um pen drive exFAT.** É preciso confirmar que o Mac abre um `.app` sem assinatura de verdade (só ad hoc) a partir de um pen drive exFAT, com dois cliques, nos dois caminhos de gravação (o instalador e o `GRAVAR-PENDRIVE-Mac.command`), e se aparece o aviso do Gatekeeper. A minha expectativa é que não apareça, porque os arquivos gravados no pen drive não vêm marcados como baixados da internet, mas isso não foi testado. Se aparecer, o caminho de "Abrir mesmo assim" vai para o `docs/criar-pendrive.md`.
3. **O `.exe` e o SmartScreen.** Mesma pergunta no Windows: um `.exe` sem assinatura no pen drive abre sem o aviso "O Windows protegeu o computador"? E, se a janelinha depender do WebView2, ela abre num Windows 10 sem internet? Se o WebView2 puder faltar, é preciso um caminho que não deixe a pessoa sem IA (por exemplo, subir a IA e abrir a conversa mesmo sem a janelinha).

**Se um risco der errado (decisão do Gabriel):** o lançador é entregue onde funciona, e o `.bat` ou o `.command` volta para a raiz do pen drive, ao lado do lançador, na plataforma com problema, até isso ser resolvido. Se o problema for só um aviso do sistema na primeira abertura, o caminho para passar por ele ("Executar assim mesmo" ou "Abrir mesmo assim") vai para o `docs/criar-pendrive.md` e para a página do projeto. O mesmo vale se o WebView2 puder faltar no Windows: o `.bat` fica na raiz como saída garantida.

### Resultado do spike (17/10/2026)

Protótipo descartável na branch local `spike-lancador` (nunca integrada), feito num Mac Apple Silicon com macOS 26.6.2. Não havia Windows nem pen drive físico na hora: o pen drive foi simulado por uma imagem exFAT.

**Biblioteca escolhida: protótipo A, Wails v2.10.1 + `fyne.io/systray` v1.12.2**, compilado com Go 1.23.12 (`GOTOOLCHAIN=go1.23.12`; o CLI do Wails v2.10.1 não compila com Go mais novo). Como o A passou no Mac, o B (Wails v3) não foi montado.

Como a bandeja e o Wails convivem:

- **Mac:** o `NSApp` e a thread principal são do Wails. A bandeja entra no mesmo loop com `systray.RunWithExternalLoop(onReady, nil)`, que só registra. A função `start` que ele devolve cria o `NSStatusItem` e precisa rodar na thread principal. O `OnStartup` do Wails roda numa goroutine, então o `start` é agendado com `dispatch_async(dispatch_get_main_queue(), ...)`, num `.m` pequeno que chama de volta uma função Go exportada. O `systray.Run` não serve no Mac, porque troca o delegate do `NSApp` e roda um segundo `[NSApp run]`.
- **Windows:** `systray.Run` numa goroutine com `runtime.LockOSThread()`, com o próprio laço de mensagens, sem disputar com o Wails.
- **Os dois:** "Sair" chama `runtime.Quit(ctx)` do Wails. A função `end` do systray não é chamada no Mac, porque ela faz `[NSApp stop:]` e espera a thread principal. No Windows, a Task 9 deve chamar `systray.Quit()` no `OnShutdown`, para o ícone não ficar "fantasma" na bandeja.
- Janela: `HideWindowOnClose: true` e `SingleInstanceLock` com `OnSecondInstanceLaunch` chamando `WindowShow`.

**O que foi conferido no Mac (10 de 10 rodadas seguidas, sem falha):**

| Ponto | Como foi conferido | Resultado |
| --- | --- | --- |
| A janela abre | `NSWindow` do Wails visível, 320×300 | ok |
| Fechar esconde e o ícone fica | clique simulado no botão vermelho (`performClick`, que passa pelo `windowShouldClose:`); o processo continua e o `NSStatusItem` segue visível, com imagem, na barra de menus | ok |
| "Mostrar" traz a janela | disparo do item do menu da bandeja pelo próprio `NSMenu` (`performActionForItemAtIndex:`), que chega ao Go pelo `ClickedCh` | ok |
| Segundo duplo clique traz a janela | `open` do `.app` já aberto (o LaunchServices reabre o mesmo processo e o AppKit mostra a janela escondida) e `open -n` (segundo processo, que passa a vez pelo `SingleInstanceLock` e sai) | ok nos dois |
| "Sair" fecha tudo | o processo some em cerca de 100 ms, com `OnShutdown` e retorno do `wails.Run` | ok |

O que **não** foi conferido no Mac: o clique de verdade com o mouse no ícone da barra de menus, e ver a tela. Esta máquina não dá permissão de acessibilidade nem de gravação de tela para a sessão. Os disparos usaram o mesmo alvo e a mesma ação do AppKit que o clique usaria, mas falta o Gabriel abrir o menu com o mouse uma vez.

**Windows: pendente de teste do Gabriel.** O `.exe` compila no Mac (`wails build -skipbindings -platform windows/amd64 -webview2 embed`, um PE32+ GUI de 10 MB), e o manifesto padrão do Wails não pede administrador. Ainda faltam os cinco pontos acima, o SmartScreen, e a janela num Windows 10 sem internet (e a mensagem, se o WebView2 faltar).

**Gatekeeper no pen drive exFAT (imagem `hdiutil -fs ExFAT`, desmontada e montada de novo antes de abrir):**

- `.app` assinado ad hoc e copiado sem atributos estendidos abre com dois cliques, sem aviso. Valeu para os dois jeitos de cópia: `cp -RX`, que é o que o `core/make-usb-unix.sh` faz, e a extração arquivo por arquivo de um ZIP, como o instalador faz. O `codesign --verify --deep --strict` passa nos dois. O `syspolicyd` registrou `GK evaluateScanResult: 2` (liberado, sem pergunta), e o app rodou do próprio volume, sem App Translocation.
- `.app` com `com.apple.quarantine` **é bloqueado**: `spctl --assess` dá `rejected`, o `syspolicyd` registra `Prompt shown` e depois `Gatekeeper denial breadcrumb`, e o Go nem chega a rodar. Por isso a cópia para o pen drive **não pode levar atributos estendidos**. Num exFAT, um `cp -R` comum leva a quarentena junto, num arquivo `._IA de Bolso.app`. Os dois caminhos de hoje já copiam sem atributos, e a Task de gravação precisa manter isso. No macOS 15 ou mais novo, o jeito de passar pelo aviso é Ajustes do Sistema > Privacidade e Segurança > "Abrir Mesmo Assim" (o clique direito em "Abrir" não basta mais). Esse caminho vai para o `docs/criar-pendrive.md` como reserva.

**Pendente de pen drive físico (Gabriel):** gravar pelo instalador e pelo `GRAVAR-PENDRIVE-Mac.command` num pen drive de verdade e abrir com dois cliques no Finder.

**Recomendação por plataforma (a decisão é do Gabriel):**

- **Mac: vai com o lançador** (Wails v2 + `fyne.io/systray`), sem plano B na raiz, desde que o teste com pen drive físico e o clique de verdade no ícone confirmem o que a imagem mostrou.
- **Windows: pendente de teste do Gabriel.** A recomendação é seguir com a mesma combinação. Se algum ponto falhar, ou o WebView2 puder faltar, o `INICIAR-CD2-Windows.bat` fica na raiz (plano B).

### Entrega do lançador no pen drive

- O lançador é compilado no CI, como o instalador e o `cd2-ctl`. Os dois caminhos de gravação (o instalador desktop e os scripts `GRAVAR-*`/`core/make-usb-*`/`core/prepare-*`) precisam colocar o lançador certo na raiz do pen drive, e ele entra no `CHECKSUMS.sha256` como o resto.
- Como o lançador chega até eles (dentro do ZIP, como release versionada igual ao `cd2-ctl`, ou outro jeito) fica a critério de quem constrói.
- No Mac, um `.app` universal (Intel e Apple Silicon) evita ter dois.

### Escolhas minhas depois da revisão (o Gabriel pode mudar)

- **Windows 11 e o menu "^".** O Windows 11 esconde ícones novos de bandeja no menu "^", e não há como forçar o ícone para fora. Aceito isso como sinal visível. Para compensar, na primeira vez que a pessoa fecha a janelinha, aparece uma notificação do sistema: "Continuo acordada aqui perto do relógio. Me chama quando quiser."
- **Uma IA por computador** vale entre lançadores. Uma IA aberta pelo plano B (scripts) não é detectada e convive com a do lançador, cada uma no seu trio de portas, como acontece hoje entre dois scripts. Se o lançador de outro pen drive for aberto com uma IA já rodando, ele traz a janelinha da IA que está rodando, mesmo que seja de outra versão.
- **Paridade com os scripts** é uma garantia de partida, não um teste permanente. O porte é feito com cuidado, e os scripts ficam documentados como plano B. Não entra um teste de paridade no CI.

### A critério de quem constrói

- A biblioteca de janela e de bandeja (ver risco 1).
- Como o lançador sabe que outra cópia já está rodando e como pede a ela para mostrar a janelinha.
- O tamanho exato da janelinha, as animações e o desenho do ícone, dentro do pixel art de `core/robo.txt` e de `mascote.js`.
- Reaproveitar ou não o código do instalador (`installer/`) num módulo comum.
- O texto final das piadas de chamada e de despedida.

## O que fica igual

- Tudo o que a conversa vê: a URL, a chave, as portas, o `cd2-ctl`, o toolbox e os logs. No `web/`, só entra a faixa de "IA dormindo".
- O Linux.
- O instalador desktop, exceto pelo que ele copia para a raiz do pen drive.

## Versão

Mudança visível para quem usa: sobe a versão menor (`VERSION`, hoje 0.13.1, vai para 0.14.0), como manda o versionamento do projeto.
