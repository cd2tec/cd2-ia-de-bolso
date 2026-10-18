# Instalação paga no computador (Stripe): design

Data: 2026-10-17. Status: rascunho para aprovação.

## Por que

Hoje a IA de Bolso só roda do pen drive. Nada fica instalado no computador além de um cache. Quem testou, gostou e quer usar todo dia sem o pen drive não tem como fazer isso.

A ideia é oferecer, de dentro da própria IA, um **serviço de instalação digital avulsa**: por R$ 25, a IA é copiada do pen drive para aquele computador e passa a abrir sem o pen drive. É também um jeito de o projeto receber uma contribuição de quem acha a IA útil.

## Objetivos

- Quem está usando a IA pelo pen drive vê um convite para instalar, na barra superior do chat e no lançador. Clicar abre um modal que explica o serviço antes de levar ao pagamento.
- O pagamento é feito no Stripe, numa página que já leva o código daquele computador.
- Pagou, a pessoa volta para a IA, responde duas perguntas e a instalação acontece sozinha, copiando do pen drive.
- A IA instalada só abre no computador para o qual foi paga. Copiada para outro computador, ou copiada do pen drive para o disco sem pagamento, ela não abre.
- Depois de instalada, ela se comporta como o lançador de hoje: janelinha, ícone perto do relógio ou na barra de menus, colocar para dormir e conversa no navegador.

## Não objetivos

- Fechar o código. O lançador e o `cd2-ctl` continuam abertos, sob Apache 2.0 (decisão do Gabriel). A trava segura quem é honesto. Ela não impede quem sabe compilar o próprio lançador ou subir a IA pelos scripts do plano B (`core/start-*`, `INICIAR-CD2-*`), e isso é aceito.
- Licença, assinatura ou mensalidade. É um serviço avulso, pago uma vez por instalação.
- Atualização automática da IA instalada. Para atualizar, a pessoa reinstala de um pen drive mais novo usando a mesma chave (ver "Reinstalar").
- Linux.
- Mudar o teste pelo pen drive. Ele continua livre e completo.
- Qualquer servidor além de um Worker na Cloudflare. Não há banco de dados nem serviço de e-mail.

## Antiobjetivos (seria um fracasso mesmo funcionando)

- A pessoa pagar e ficar sem instalação, sem chave e sem como recuperar. Toda falha depois do pagamento precisa deixar a chave nas mãos dela.
- A chave secreta do Stripe ou o segredo de assinatura aparecerem no repositório, no pen drive ou no navegador da pessoa.
- O teste pelo pen drive ficar travado ou pedir pagamento.
- O modal esconder as condições: valer só para um computador, ser avulso e pedir novo pagamento ao trocar de computador.
- Mandar para fora a identidade crua da máquina (MachineGuid, UUID do hardware). Só sai um código derivado dela.
- A instalação deixar sujeira ao desinstalar: atalho, início automático ou entrada em Aplicativos esquecida.
- A pessoa ver terminal, caminho de arquivo ou mensagem em inglês sem tradução. Vale a mesma regra do instalador e do lançador.

## Regras do serviço (decididas pelo Gabriel)

- **Nome:** "serviço de instalação digital". Não é doação nem licença. A palavra "doação" foi descartada: com contrapartida obrigatória, juridicamente é venda de serviço. **Nota fiscal: fica para depois (decisão do Gabriel).** Tudo é construído e testado no modo de teste do Stripe; cobrar de verdade só depois de a contadora da CD2 responder sobre a nota fiscal de serviço, o enquadramento e o que o Checkout precisa coletar do tomador.
- **Preço:** R$ 25, pagamento único, por instalação.
- **Vale para um computador.** A chave fica presa ao código daquele computador.
- **Reinstalar no mesmo computador com a chave é livre.** Perdeu a chave ou trocou de computador, é uma nova instalação, com novo pagamento. Não há recuperação manual pelo WhatsApp.
- **Infraestrutura:** só o que já está no pen drive, o Stripe e um Worker minúsculo na Cloudflare.
- **Endereço do Worker:** `instalariadebolso.cd2.com.br` (decisão do Gabriel). Ele fica gravado em cada pen drive, então não muda depois.
- **Reembolso e arrependimento:** risco aceito pelo Gabriel. A chave é conferida offline e não pode ser desativada depois de um reembolso ou estorno; a CD2 devolve o dinheiro se for o caso, e a instalação continua funcionando.

## A experiência (decidida)

Os rascunhos aprovados estão em `.superpowers/brainstorm/41689-1792287902/content/`, fora do git: `botao-comprar.html` (opção B nos dois lugares), `modal-compra-v2.html` e `telas-instalacao.html`.

### 1. O convite (só quando a IA roda do pen drive)

- **Barra superior do chat** (`web/index.html`, `.topbar`): uma etiqueta amarela **"rodando do pen drive"** junto das etiquetas de hoje e um botão de contorno azul **"Instalar no computador"** antes de "Nova conversa". **Sem preço no botão** (decisão do Gabriel).
- **Lançador**, nas telas "Acordei!" e "Me chamou?": embaixo dos dois botões, uma caixinha tracejada amarela com "Quer me usar sem o pen drive?" e, em destaque, "Instale neste computador".
- **Menu do ícone:** um item **"Instalar neste computador"**, entre "Abrir uma nova janela da conversa" e "Colocar a IA para dormir".
- Na IA instalada, a etiqueta, o botão, a caixinha e o item somem.
- **Escolha minha:** se a conversa não souber com certeza de onde a IA está rodando (por exemplo, aberta pelos scripts do plano B sem a informação), o convite não aparece.

### 2. O modal de compra

O mesmo modal nos dois lugares. **Escolha minha:** a janelinha do lançador é pequena demais para ele, então a caixinha e o item do menu abrem uma janela da conversa no navegador já com o modal aberto.

Conteúdo, como aprovado no rascunho:

- Sobretítulo "Serviço de instalação digital", com a régua azul do SectionHeader. Título "Instale a IA de Bolso neste computador".
- A frase "Gostou do teste? Com a instalação, a IA passa a morar neste computador e abre sem o pen drive, do mesmo jeitinho que você já conhece."
- **R$ 25**, com "pagamento único, por instalação".
- **O que está incluído:** a IA copiada do pen drive sem baixar nada; o ícone nos programas e, se a pessoa quiser, na área de trabalho; acordar junto com o computador, se ela quiser, com o aviso de que ocupa de 4 a 5 GB de memória desde que o computador liga e de que dá para desligar depois; desinstalar quando quiser.
- **Como funciona:** é um serviço avulso, não assinatura nem licença; vale só para este computador e, copiada para outro, não abre; a chave chega por e-mail e reinstala neste mesmo computador; trocou de computador ou perdeu a chave, é uma nova instalação, com novo pagamento.
- O **código deste computador** num bloco cinza, com a nota "vai junto com o pagamento".
- A caixa **"Entendi que a instalação vale só para este computador."** O botão **"Ir para o pagamento"** só acende com ela marcada. Ao lado fica "Agora não".
- No rodapé: "Pagamento seguro pelo Stripe. Precisa de internet só agora."
- **Escolha minha:** se este computador já tem uma chave válida guardada (ver "Onde a chave mora"), o modal troca o pagamento por **"Reinstalar com a minha chave"**, sem cobrar.
- **Escolha minha:** um link "Já tenho a chave" no modal abre um campo para colar a chave, para quem pagou mas perdeu a volta.

### 3. O pagamento e a volta

1. "Ir para o pagamento" abre, numa aba nova, o Worker com o código do computador e a porta da IA. O Worker cria a sessão de pagamento no Stripe e redireciona para ela.
2. A pessoa paga no Stripe.
3. O Stripe devolve a pessoa ao Worker. O Worker confere com o Stripe que a sessão está paga e gera a chave.
4. O Worker mostra uma página simples, com a identidade da CD2, contendo a chave, um botão "Copiar a chave" e um botão **"Voltar para a IA"**, que leva de volta à IA em `127.0.0.1` com a chave. A volta é só pelo botão: a página não redireciona sozinha, para a chave ficar sempre à vista.
5. A chave também chega por e-mail (ver "Como a chave chega por e-mail").

Enquanto isso, a janela da conversa que abriu o modal mostra "Aguardando o pagamento...", com o campo para colar a chave como reserva.

### 4. A instalação

Tudo no mesmo modal da conversa:

1. **"Pagamento confirmado"**, com o título "Tudo certo! Só duas perguntinhas":
   - "Criar um atalho na área de trabalho". Vem marcada. O ícone nos programas é criado sempre.
   - "Acordar quando o computador ligar", com o aviso dos 4 a 5 GB de memória e de que dá para desligar no ícone. Vem marcada, porque a IA sempre carrega ao iniciar, com a opção de desligar (decisão do Gabriel).
   - Botão "Instalar".
2. **"Mudando para o seu computador..."**: barra de progresso real, com o que está sendo copiado, quanto falta e o tempo estimado, e o aviso "Não tire o pen drive agora".
3. **"Pronto! Agora eu moro aqui"**: abrir pelo ícone nos programas ou na área de trabalho e "Depois de abrir a IA instalada, pode tirar o pen drive". A chave na tela, com "Copiar a chave" e a nota "Guarde. Ela reinstala neste computador quando precisar". Botão "Abrir a IA instalada".

**"Abrir a IA instalada"** põe a IA do pen drive para dormir e abre o lançador instalado. Como os dois lançadores usam o mesmo `idUnico`, sem isso o lançador instalado só traria a janelinha da IA do pen drive. **Escolha minha:** a ordem é dormir, esperar tudo parar e só então abrir o instalado.

### 5. A IA instalada no dia a dia

- Abre pelo ícone nos programas (Menu Iniciar no Windows; Aplicativos, Launchpad e Spotlight no Mac) ou pelo atalho da área de trabalho, se a pessoa quis.
- Se a pessoa marcou, acorda quando o computador liga. **Escolha minha:** ao acordar com o computador, ela sobe direto no ícone, sem abrir a janelinha nem a conversa, para não jogar uma janela na cara de quem acabou de ligar o computador. Clicar no ícone abre como hoje.
- O menu do ícone ganha:
  - **"Acordar quando o computador ligar"**, com sinal de marcado, que liga e desliga na hora;
  - **"Desinstalar..."**.
- No Windows, a IA também aparece em Configurações > Aplicativos, com o desinstalar de lá funcionando igual.
- Todo o resto é o lançador de hoje.

### 6. Desinstalar

- Pede confirmação na janelinha, com o cérebro com sono: "Quer mesmo me desinstalar?". Explica que apaga a IA deste computador, os atalhos e o acordar com o computador, que a conexão com o banco também sai, e que para voltar basta usar a chave que foi para o e-mail. Botões "Desinstalar" e "Melhor não".
- Põe a IA para dormir, apaga a pasta da instalação, os atalhos, o início automático, a entrada em Aplicativos (Windows) e o `db.env` deste computador.
- **Escolha minha:** desinstalar **não** apaga a chave guardada no computador. Assim, reinstalar do pen drive no mesmo computador continua livre, que é a regra do serviço.

### 7. Cópia sem chave

Se o lançador estiver rodando fora do pen drive e não houver chave válida para este computador, ele não sobe a IA. A janelinha mostra o cérebro triste, "Esta cópia não foi instalada neste computador" e "Para me usar daqui, instale pelo pen drive. Se você já pagou por este computador, cole a sua chave", com os botões **"Instalar neste computador"** (que explica que é preciso abrir pelo pen drive) e **"Já tenho a chave"** (campo para colar; se a chave valer, a IA sobe).

## Como é feito

### Decidido

- **Stripe** cobra e **um Worker na Cloudflare** confere e assina. O código do Worker fica neste repositório, numa pasta própria (sugestão: `worker/`), e é publicado com o Wrangler. A chave secreta do Stripe e o segredo de assinatura ficam como *secrets* do Worker, nunca no código.
- **O código do lançador e do `cd2-ctl` continua aberto.**
- **O lançador instalado é o mesmo lançador do pen drive**, copiado. Ele já acha a raiz a partir de onde está (`launcher/internal/raiz`), então roda da pasta de instalação sem lógica nova para subir a IA.

### O código do computador (escolha minha)

- É derivado da identidade da máquina, nunca enviado cru: no Windows, o `MachineGuid` (`HKLM\SOFTWARE\Microsoft\Cryptography`); no Mac, o `IOPlatformUUID`.
- Fórmula: SHA-256 de um prefixo fixo do projeto mais a identidade, com os primeiros 60 bits em base32 sem letras ambíguas, mostrados em três grupos de quatro (exemplo: `7F3K-Q9WD-2MXA`).
- **Consequência que precisa ficar clara:** no Windows, o `MachineGuid` muda ao reinstalar o Windows. Formatou, o código muda e é preciso pagar de novo, o que bate com a regra do serviço. No Mac, o UUID do hardware sobrevive à reinstalação do macOS, então a chave do e-mail continua valendo.

### A chave (escolha minha)

- É uma assinatura **Ed25519** do texto `cd2-instalacao-v1|<código do computador>`, feita pelo Worker com a chave privada. A chave pública vai embutida no lançador e no `cd2-ctl`, e a conferência é offline.
- O formato para a pessoa é `CD2-` seguido da assinatura em base32 sem preenchimento: são 107 caracteres, que cabem no limite de 140 do campo da fatura. Na tela, a chave pode aparecer em grupos para leitura, mas o "Copiar a chave" e o e-mail levam o texto corrido, e o campo de colar ignora espaços, hífens e maiúsculas ou minúsculas. A pessoa copia e cola, não digita.
- O `v1` permite trocar o par de chaves no futuro aceitando as duas por um tempo.
- **O Worker não guarda nada.** A chave depende só do código do computador, então ele não precisa de banco: a mesma compra sempre gera a mesma chave.

### O Worker (escolha minha, dentro do decidido)

- O Worker responde em `https://instalariadebolso.cd2.com.br`, num domínio personalizado da Cloudflare.
- `GET /comprar?codigo=…&porta=…`: confere o formato do código, cria uma Checkout Session de R$ 25 (BRL, preço cadastrado no painel do Stripe) com `client_reference_id` igual ao código e a porta nos metadados, e redireciona para o Stripe. A volta de sucesso é `/voltou?session_id={CHECKOUT_SESSION_ID}`, e a de cancelamento é uma página "Tudo bem, a IA continua no pen drive".
- `GET /voltou?session_id=…`: busca a sessão no Stripe, exige `payment_status = paid`, assina o código guardado na sessão e mostra a página da chave com o botão "Voltar para a IA" (`http://127.0.0.1:<porta>/?instalar=<chave>`). Recarregar a página mostra a mesma chave, porque a assinatura é determinística.
- Sem webhook. A conferência é feita na volta, direto no Stripe. O e-mail cobre quem fechou a aba antes de voltar.
- Meios de pagamento: cartão, e **Pix se a conta Stripe da CD2 tiver Pix ativo** (a conferir na conta).
- **Termos:** o Checkout pede o aceite de uma página curta de condições do serviço no site do projeto (`docs/`), com o mesmo texto de "Como funciona" do modal. **Escolha minha.**

### Como a chave chega por e-mail (escolha minha, a confirmar no Stripe)

Para não precisar de um serviço de e-mail: como a chave depende só do código do computador, o Worker já a calcula ao criar a sessão e a coloca num campo personalizado da fatura que o Checkout gera depois do pagamento (`invoice_creation`, com `invoice_data.custom_fields`, rótulo "Chave de instalação"). O Stripe manda a fatura por e-mail só depois que o pagamento cai. A pessoa nunca vê a chave antes de pagar, porque esse campo fica no servidor.

**A conferir no começo:** o limite de tamanho do valor do campo (a documentação fala em 140 caracteres; a chave tem uns 110) e o custo da fatura pós-pagamento do Stripe. Se não couber ou não compensar, a alternativa é um envio de e-mail pelo Worker (por exemplo, Resend), o que adiciona um serviço e precisa do aval do Gabriel.

### Onde a chave mora no computador (escolha minha)

- Num arquivo na pasta de cache da IA neste computador (`%LOCALAPPDATA%\cd2-harness`, `~/Library/Caches/cd2-harness`, as mesmas do `db.env`), **fora** da pasta de instalação.
- Assim, a chave sobrevive a desinstalar e reinstalar, e o pen drive percebe que este computador já tem chave e oferece "Reinstalar com a minha chave".
- A chave também é conferida no arranque do lançador instalado. Ela só vale se bater com o código deste computador. Se o arquivo for copiado para outro computador, a chave não vale lá.

### Pen drive ou instalado: a trava (escolha minha)

- No arranque, o lançador descobre se a raiz dele está **no pen drive** ou **no disco**. No Windows, pelo tipo da unidade (removível); no Mac, pelas propriedades do volume (externo ou removível).
- **No pen drive:** sobe livre, como hoje, e avisa a conversa que está rodando do pen drive, para mostrar o convite.
- **No disco:** só sobe a IA se houver chave válida para este computador. Sem ela, mostra a tela "Esta cópia não foi instalada neste computador".
- **Risco 1 (conferir antes de construir o resto):** alguns pen drives se apresentam ao Windows como disco fixo, e HDs ou SSDs externos aparecem como externos no Mac. O primeiro caso trancaria o teste de quem tem um pen drive desses, o que é um antiobjetivo. Quem constrói decide a regra exata. Uma ideia é considerar pen drive o volume com rótulo `CD2` e conteúdo do pen drive num barramento USB, mas a regra precisa ser testada em pen drives reais e nunca pode trancar o teste.

### Como a conversa e o lançador conversam na instalação

- A conversa já fala com o `cd2-ctl` em `127.0.0.1`, protegida pela chave da sessão. **Escolha minha:** o `cd2-ctl` ganha as rotas da instalação: saber de onde a IA roda e o código do computador, conferir uma chave, instalar com progresso e desinstalar. A cópia dos arquivos, os atalhos e o início automático moram num pacote Go compartilhado entre o `cd2-ctl` e o lançador, porque o menu do ícone também desinstala e liga ou desliga o início automático. Quem constrói pode mudar essa divisão, desde que a conversa só faça a tela e o trabalho no sistema fique em Go.
- **A volta do Stripe chega numa aba nova, sem a chave da sessão** (`#k=`), que só a aba original tem. **Escolha minha:** a aba nova, que tem a mesma origem `127.0.0.1:<porta>`, entrega a chave à aba original por `BroadcastChannel` ou `localStorage` e se fecha ou mostra "Pode voltar para a outra janela". Se a aba original não existir mais, ela mostra a chave e pede para abrir a conversa pelo lançador e colar a chave no modal.
- Se a IA do pen drive tiver sido fechada e aberta de novo entre o pagamento e a volta, a porta pode ter mudado e o "Voltar para a IA" não acha ninguém. Nesse caso, valem a chave na página do Worker, a chave no e-mail e o "Já tenho a chave".

### O que a instalação copia e onde fica (escolha minha)

- Só o que serve a este computador: o lançador da plataforma, `web/`, `core/` (o necessário para o plano B), o `llamafile`, o modelo, `VERSION`, e o toolbox e o `cd2-ctl` **só da plataforma atual**. Fica de fora o que é só do pen drive (lançador da outra plataforma, scripts `INICIAR-*`, `CHECKSUMS` das outras plataformas).
- Confere os SHA-256 do que copiou com o `CHECKSUMS.sha256` do pen drive. Se não bater, a instalação falha com "Não consegui copiar direito. Tente de novo", sem deixar meia instalação.
- Pede uns 3 GB livres e confere o espaço antes de começar, avisando em português se faltar.
- **Windows:** em `%LOCALAPPDATA%\Programs\CD2 IA de Bolso\`, sem pedir administrador. Atalho no Menu Iniciar sempre e na área de trabalho se marcado. Início automático pela chave `Run` de `HKCU`. Entrada em Configurações > Aplicativos pela chave `Uninstall` de `HKCU`, apontando para o próprio lançador com uma opção de desinstalar.
- **Mac:** a IA precisa aparecer em Aplicativos, no Launchpad e no Spotlight. Como o lançador acha a raiz pela pasta que contém o `.app`, quem constrói escolhe entre `~/Applications/IA de Bolso/` (pasta com o `.app` e os arquivos ao lado) ou os arquivos em `~/Library/Application Support/` com o `.app` em `~/Applications`. Precisa preservar a assinatura e a notarização que o `.app` já tem (branch `assinatura-mac`). Início automático por LaunchAgent em `~/Library/LaunchAgents` ou pelo `SMAppService`, a critério de quem constrói. O atalho da área de trabalho é um apelido (alias) do `.app`.
- **Risco 2 (conferir antes de construir o resto):** o `.app` copiado do pen drive para o disco abre sem aviso do Gatekeeper e mantém a assinatura válida? No Windows, o `.exe` copiado abre sem o SmartScreen?

### Reinstalar e atualizar

- Do pen drive, num computador que já tem chave válida guardada, o modal oferece "Reinstalar com a minha chave", sem pagamento, e segue direto para as duas perguntas.
- Reinstalar por cima de uma instalação existente substitui os arquivos. É assim que a pessoa atualiza para a versão de um pen drive mais novo.
- **Escolha minha, limitação desta primeira versão:** como os dois lançadores usam o mesmo `idUnico`, abrir o lançador do pen drive com a IA instalada acordada só traz a janelinha da instalada. Para atualizar, a pessoa põe a instalada para dormir e abre o lançador do pen drive, que então oferece "Reinstalar com a minha chave". O `docs/criar-pendrive.md` explica isso. Um "Atualizar pelo pen drive" no menu da instalada fica para depois.
- **Escolha minha, várias contas no mesmo computador:** a regra é "vale para este computador", e o código vem da máquina, não do usuário. Outra conta do mesmo computador instala com a mesma chave, sem pagar de novo. O texto das condições diz "neste computador", o que já cobre isso.
- A conexão com o banco (`db.env`) mora no cache e é compartilhada entre a IA do pen drive e a instalada, como hoje.

### A critério de quem constrói

- A regra exata de "está no pen drive" (respeitando o risco 1).
- A divisão do trabalho entre o `cd2-ctl` e o lançador.
- O layout final no Mac (respeitando o risco 2 e a assinatura).
- O desenho da página do Worker, dentro da identidade da CD2.
- O texto final das condições do serviço, a partir do "Como funciona" do modal.

## Riscos para conferir logo no começo

1. **Pen drive que parece disco fixo** (ver "a trava"). Não pode trancar o teste.
2. **Assinatura e avisos do sistema** no `.app` e no `.exe` copiados para o disco.
3. **A chave no e-mail pela fatura do Stripe:** tamanho do campo e custo.
4. **Pix na conta Stripe da CD2.**

## O que a CD2 precisa providenciar

- Conta no Stripe com o preço de R$ 25 cadastrado, primeiro em modo de teste.
- Conta na Cloudflare para o Worker.
- O par de chaves Ed25519, gerado uma vez: a privada vai como *secret* do Worker e a pública vai para o código.
- O DNS de `cd2.com.br` apontando `instalariadebolso` para o Worker na Cloudflare.
- A conversa com a contadora sobre a nota fiscal do serviço, antes de ligar o modo real do Stripe.

## O que fica igual

- O teste pelo pen drive, inteiro.
- O que a conversa vê, além do convite e do modal.
- O instalador desktop que grava o pen drive.
- O Linux.

## Versão

Mudança visível para quem usa: sobe a versão menor (`VERSION`), como manda o versionamento do projeto.
