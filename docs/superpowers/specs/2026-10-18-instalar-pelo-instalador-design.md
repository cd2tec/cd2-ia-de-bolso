# Instalar no computador pelo instalador: design

Data: 2026-10-18. Status: rascunho para aprovação.

## Por que

Hoje o instalador (`installer/`, o "criador de pen drive" baixado do site) só grava pen drive. A cópia paga para o computador (PR #25, `docs/superpowers/specs/2026-10-17-instalacao-paga-design.md`) só é oferecida de dentro da IA rodando do pen drive. Quem chega pelo site e quer a IA direto no computador precisa primeiro gravar um pen drive.

O objetivo é que o instalador ofereça os dois caminhos logo na primeira tela: **criar o pen drive, que é grátis**, ou **instalar neste computador, comprando a cópia**. O segundo caminho paga no Stripe, volta ao instalador e instala a IA direto no computador, sem pen drive.

## Objetivos

- A primeira tela do instalador mostra duas escolhas lado a lado:
  - **"Criar o pen drive da IA de Bolso"**, com o selo **Grátis**;
  - **"Instalar neste computador"**, com "compre a cópia, R$ 25".
- O caminho do pen drive continua exatamente como hoje.
- O caminho de instalar paga pelo mesmo Worker e pelo mesmo produto da cópia para este computador (PR #25), volta ao instalador com a chave e instala no mesmo destino, com os mesmos atalhos, o mesmo acordar com o computador e o mesmo desinstalar.
- O computador instalado por aqui fica igual ao instalado a partir do pen drive. A chave vale do mesmo jeito.

## Não objetivos

- Mudar o fluxo de gravar pen drive.
- Mudar o Worker, o produto do Stripe ou as regras da cópia: um computador, chave por e-mail, reinstalar com a chave e novo pagamento em outro computador. A exceção é o pequeno ajuste de texto descrito em "Como é feito".
- Linux. O instalador não existe para Linux.
- Fechar o código ou assinar o instalador de outro jeito.

## Antiobjetivos

- A pessoa pagar e o instalador não conseguir instalar, sem dar a ela a chave. A chave aparece na página do Worker, chega por e-mail e pode ser colada no instalador.
- A IA ser instalada na pasta de **outra conta** do Windows, porque o instalador roda como administrador.
- O caminho do pen drive ficar mais lento, mais confuso ou passar a pedir pagamento.
- Baixar os 3,9 GB e só depois descobrir que falta espaço ou que a chave não vale.

## A experiência (decidida pelo Gabriel)

1. **Escolha.** Primeira tela com os dois cartões descritos em "Objetivos".
2. **Condições.** Uma tela com o mesmo conteúdo do modal da conversa: sobretítulo "Cópia para este computador", R$ 25, o que está incluído, como funciona e o código deste computador. Inclui a caixa "Entendi que a instalação vale só para este computador.", o botão **"Ir para o pagamento"** e o link "Já tenho a chave".
3. **Pagamento.** O Stripe abre no navegador padrão. O instalador mostra "Aguardando o pagamento...", com o campo para colar a chave como reserva.
4. **Volta.** Na página da chave do Worker, o botão de voltar devolve a chave ao instalador, que segue sozinho.
5. **Perguntas.** As mesmas duas da conversa: atalho na área de trabalho e acordar com o computador, as duas marcadas, com o aviso dos 4 a 5 GB de memória.
6. **Baixando e instalando.** Uma barra única com o que está acontecendo e o tempo restante: baixar (uns 3,9 GB na primeira vez), conferir e instalar.
7. **Pronto.** A chave na tela, com "Copiar a chave", e o botão **"Abrir a IA de Bolso"**.
- Se o computador já tem chave válida guardada, o caminho de instalar pula o pagamento e oferece **"Reinstalar com a minha chave"**, como no pen drive.

## Como é feito

### Decidido

- **Reaproveitar o preparo do instalador.** Ele já baixa e confere tudo numa pasta de cache (`preparo.Montar`, que monta o stage `CD2-HARNESS`). Instalar no computador é montar o mesmo stage e entregá-lo ao módulo `instalacao` (`instalacao.Instalar` com `Raiz` = stage), em vez de copiar o stage para um pen drive.
- **Mesmo módulo `instalacao`.** O instalador passa a usar `instalacao/` (via `replace ../instalacao`, como o ctl e o lançador) para o código do computador, a chave, a guarda e a instalação.
- **Mesmo Worker e mesmo produto.** A compra usa `/comprar?codigo=...&porta=...`. O Worker é escolhido pelo pacote `ambiente`: o build oficial usa o de produção e o build `-tags sandbox` usa o de teste.
- **A volta da chave.** O instalador sobe um servidor só em `127.0.0.1`, numa porta livre, que recebe `/?chave=...`, como a conversa faz. Ele responde com uma página simples, "Pronto, pode voltar para o instalador", entrega a chave à tela e se desliga. A chave só é aceita depois de `chave.Normalizar` e de conferida para este computador.
- **Windows sem administrador para instalar.** O instalador abre como administrador por causa da formatação. No caminho de instalar, ele **reabre a si mesmo sem administrador**, como o usuário que está logado, e a instância elevada sai. Assim, o destino (`%LOCALAPPDATA%\Programs\...`), o cache, a chave e o `HKCU` ficam na conta certa.
- **Abrir a IA ao final.** "Abrir a IA de Bolso" inicia o lançador instalado, destacado, com a pasta de trabalho na pasta da instalação, e fecha o instalador.

### Escolhas minhas (o Gabriel pode mudar)

- **Texto do botão de volta.** O Worker ganha um parâmetro opcional em `/comprar`, `de=instalador`, guardado nos metadados da sessão. Com ele, o botão da página da chave diz "Voltar para o instalador", e não "Voltar para a IA". É a única mudança no Worker, publicada nos dois ambientes.
- **Espaço.** Antes de pagar, a tela de condições confere se cabe: uns 3,9 GB do download no cache mais uns 3 GB da instalação. Se não couber, avisa e não deixa seguir para o pagamento.
- **Cache.** É o mesmo dos scripts e do caminho do pen drive. Quem já gravou um pen drive neste computador não baixa de novo.
- **Cancelar durante o download.** Volta para a tela de perguntas, com a chave já guardada, e dá para tentar de novo sem pagar.

### Respostas da revisão (já valem como decisão)

- **Conferência da chave é offline.** É feita com `chave.Conferir(chave, codigo.Ler())`, a assinatura Ed25519 sobre o código do computador. O instalador confere e grava com `guarda.Gravar` **antes** de baixar qualquer coisa. Não há consulta ao Worker.
- **Duas pastas de cache, como hoje.** A chave e o `db.env` ficam em `cd2-harness` (`%LOCALAPPDATA%\cd2-harness`, `~/Library/Caches/cd2-harness`), a mesma pasta que o lançador e o `cd2-ctl` usam. Esse é o `instalacao.Opcoes.Cache`. O stage baixado fica em `cd2-harness/usb`, a pasta do instalador e dos scripts. O desinstalar não mexe no stage.
- **Escolha minha: o stage fica depois de instalar**, como acontece depois de gravar um pen drive. Reinstalar ou gravar um pen drive não baixa de novo. A conta de espaço antes de pagar é: o que falta baixar (`preparo.BytesParaBaixar`) mais uns 3 GB da instalação, com folga de 10%.
- **Escolha minha: o instalador reabre no Windows com `--instalar`.** A instância sem administrador abre direto na tela de condições, sem repetir a escolha. Se a reabertura falhar, o caminho de instalar fica bloqueado com o aviso "Abra o instalador de novo sem ser como administrador (clique duplo normal) para instalar neste computador". O caminho do pen drive continua disponível.
- **Escolha minha: instalador fechado durante o pagamento.** Não há "pagamento em andamento" salvo em disco. A chave continua na página do Worker e no e-mail. Ao reabrir, a pessoa usa "Já tenho a chave". Se a porta antiga não existir mais, o botão de volta do Worker falha, mas a página já mostra a chave.
- **Escolha minha: `de=instalador` no Worker.** O Worker aceita só o valor exato `instalador` e ignora qualquer outro. Ele vai para os metadados da sessão e para o `cancel_url` (`/cancelou?de=instalador`). Com ele, o botão diz "Voltar para o instalador" e a página de cancelado diz "Tudo bem, nada foi cobrado. Pode fechar esta página e voltar para o instalador." Sem ele, tudo fica como hoje, inclusive para sessões já criadas.
- **Plataforma.** O stage de `preparo.Montar` traz o lançador universal do Mac e as variantes de toolbox e `cd2-ctl` de todas as plataformas. `copiar.Planejar(stage, runtime.GOOS, runtime.GOARCH)` escolhe a do computador.

### Riscos para conferir logo no começo

1. **Reabrir sem administrador no Windows.** O caminho mais comum é pedir ao Explorer que abra o programa, porque ele roda como o usuário logado. É preciso provar isso num Windows real: com o administrador sendo o próprio usuário e com outra conta digitando a senha. Se não funcionar, o caminho de instalar mostra um aviso pedindo para abrir o instalador sem ser como administrador (botão direito, sem "Executar como administrador"). A parte de formatar é que exige administrador.
2. **Mac.** O instalador não pede senha no Mac, então não há esse problema. Confirmar que o `.app` instalado a partir do stage (cópia do ZIP) abre sem aviso do Gatekeeper.

### A critério de quem constrói

- O desenho exato das telas, dentro do estilo atual do instalador e dos rascunhos já aprovados para o modal.
- A forma de juntar as etapas (baixar, conferir, instalar) numa barra só.
- Como a tela se comunica com o Go (eventos Wails, como o caminho de gravar já faz).

## O que fica igual

- O caminho de gravar pen drive.
- A IA instalada, que é a mesma do PR #25.
- O Worker, exceto pelo texto do botão de volta.

## Versão

Sobe a versão menor (`VERSION`, 0.15.0 para 0.16.0) e o instalador é regerado pelo ZIP, como sempre.
