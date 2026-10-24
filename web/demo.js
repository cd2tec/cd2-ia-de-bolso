/* Modo Demonstração: ferramentas com dados fictícios (funciona offline) e o roteiro que ensina a
   conectar o banco do ERP. Cada ferramenta devolve o texto para o modelo e o mesmo dado como
   gráfico para a tela. As perguntas de orientação têm resposta pronta, sem passar pelo modelo. */
(function(root){
"use strict";
const TAG = '[DADOS FICTÍCIOS DE DEMONSTRAÇÃO]\n';

const TOOLS = [
  { name:'vendas_por_departamento', description:'Vendas do mês por departamento do supermercado (demonstração).', inputSchema:{ type:'object', properties:{}, required:[] } },
  { name:'vendas_por_mes', description:'Venda total de cada mês do ano no supermercado (demonstração).', inputSchema:{ type:'object', properties:{}, required:[] } },
  { name:'perdas_do_mes', description:'Maiores perdas (quebras) do mês por motivo (demonstração).', inputSchema:{ type:'object', properties:{}, required:[] } },
  { name:'rupturas_criticas', description:'Produtos de curva A em ruptura nas lojas, com a venda perdida (demonstração).', inputSchema:{ type:'object', properties:{ limite:{ type:'integer', description:'Quantidade máxima de itens' } }, required:[] } },
  { name:'perfil_do_cliente', description:'Em que departamentos um cliente do cartão fidelidade gasta, no mês (demonstração).', inputSchema:{ type:'object', properties:{}, required:[] } }
];

const RUPTURAS = [
  ['Arroz 5kg tipo 1', 'Loja 03', 4, 18420],
  ['Óleo de soja 900ml', 'Loja 01', 3, 9870],
  ['Leite integral 1L', 'Loja 07', 2, 8310],
  ['Café torrado 500g', 'Loja 03', 5, 7650],
  ['Detergente 500ml', 'Loja 05', 6, 3240]
];
const limiteDe = a => Math.max(1, Math.min(RUPTURAS.length, parseInt(a && a.limite) || 3));
// Flash de vendas: a venda de cada hora de hoje, das 08h às 20h (o exemplo agendado de hora em hora).
const FLASH = [12840, 18320, 22470, 27910, 34650, 29880, 21360, 19740, 24120, 31560, 38270, 33190, 17460];
const pad = n => (n < 10 ? '0' : '') + n;

// O gráfico de cada ferramenta: o formato que CD2Chart.render recebe (colunas, linhas, x, y).
const GRAFICOS = {
  vendas_por_departamento:() => ({ titulo:'Vendas do mês por departamento', visual:'barra', x:'Departamento', y:'Venda (R$)',
    frase:'Mercearia lidera, mas a Padaria tem a melhor margem: 38,2%.',
    colunas:['Departamento', 'Venda (R$)', 'Margem (%)'],
    linhas:[['Mercearia', 1842300, 24.1], ['Açougue', 1402560, 22.9], ['Bebidas', 1215870, 27.8], ['Hortifruti', 934110, 31.5], ['Limpeza', 702980, 29.4], ['Padaria', 611420, 38.2]] }),
  vendas_por_mes:() => ({ titulo:'Vendas por mês em 2026', visual:'linha', x:'Mês', y:'Venda (R$)',
    frase:'Março e julho puxaram para cima; setembro fechou 6% acima de agosto.',
    colunas:['Mês', 'Venda (R$)'],
    linhas:[['2026-01-01', 6120400], ['2026-02-01', 5874300], ['2026-03-01', 6703900], ['2026-04-01', 6318200], ['2026-05-01', 6452700],
      ['2026-06-01', 6280100], ['2026-07-01', 6931500], ['2026-08-01', 6409800], ['2026-09-01', 6794400]] }),
  perdas_do_mes:() => ({ titulo:'Perdas do mês por motivo', visual:'barra', x:'Motivo', y:'Perda (R$)',
    frase:'Vencimento é a maior perda: 1,12% da venda do mês.',
    colunas:['Motivo', 'Perda (R$)', '% da venda'],
    linhas:[['Vencimento', 118400, 1.12], ['Ajuste de inventário (não identificada)', 96700, 0.92], ['Avaria', 64250, 0.61], ['Quebra operacional', 41900, 0.40]] }),
  rupturas_criticas:a => ({ titulo:'Rupturas da curva A: venda perdida', visual:'barra', x:'Produto', y:'Venda perdida (R$)',
    frase:'Arroz 5kg na loja 03 é o furo mais caro: 4 dias sem estoque.',
    colunas:['Produto', 'Loja', 'Dias sem estoque', 'Venda perdida (R$)'],
    linhas:RUPTURAS.slice(0, limiteDe(a)).map(r => r.slice()) }),
  perfil_do_cliente:() => ({ titulo:'Cliente 0042 do cartão fidelidade: onde gasta', visual:'pizza', x:'Departamento', y:'Gasto no mês (R$)',
    frase:'Quase metade do gasto vai para Mercearia, e a Padaria nem aparece nas compras: oportunidade de oferta.',
    colunas:['Departamento', 'Gasto no mês (R$)'],
    linhas:[['Mercearia', 612.4], ['Açougue', 318.9], ['Hortifruti', 167.3], ['Bebidas', 142.8], ['Limpeza', 96.5]] }),
  // a.hora: até que hora do dia (sem ela, o dia todo); pelo menos a primeira hora.
  flash_de_vendas:a => {
    const h = a && Number.isInteger(a.hora) ? a.hora : 20;
    const linhas = FLASH.slice(0, Math.max(1, Math.min(FLASH.length, h - 7))).map((v, i) => [pad(8 + i) + 'h', v]);
    const pico = linhas.reduce((x, y) => y[1] > x[1] ? y : x);
    return { titulo:'Venda por hora hoje', visual:'barra', x:'Hora', y:'Venda (R$)',
      frase:(linhas.length > 1 ? 'Até agora, o pico foi às ' : 'Primeira hora do dia: ') + pico[0] + ', com R$ ' + BR.format(pico[1]) + ' na hora.',
      colunas:['Hora', 'Venda (R$)'], linhas };
  }
};
function grafico(name, a){
  const f = GRAFICOS[name]; if(!f) return null;
  const g = f(a);
  g.linhas = g.linhas.map(r => Object.fromEntries(g.colunas.map((c, i) => [c, r[i]])));
  return g;
}
const BR = new Intl.NumberFormat('pt-BR', { maximumFractionDigits:2 });
function call(name, a){
  const g = grafico(name, a);
  if(!g) throw new Error('Ferramenta desconhecida: ' + name);
  const txt = v => typeof v === 'number' ? BR.format(v) : String(v);
  return TAG + g.colunas.join(' | ') + '\n' + g.linhas.map(r => g.colunas.map(c => txt(r[c])).join(' | ')).join('\n');
}

// Instrução extra para o modelo no modo Demonstração.
const PROMPT = ' Você está no modo Demonstração: as ferramentas devolvem dados fictícios de um supermercado de exemplo. '
  + 'A tela já mostra o gráfico de cada resultado de ferramenta: diga que os números são fictícios e comente em até duas frases o que mais chama atenção, conferindo cada número na tabela antes de citar. '
  + 'Você é a IA de Bolso da CD2, feita para analisar ERPs de varejo; com o banco de dados conectado em Configurações, a mesma pergunta usa os dados reais da loja.';

// Roteiro: perguntas de orientação com resposta pronta. Cada bloco é um parágrafo (p), uma lista
// (lista, numerada ou não) ou um gráfico de exemplo (grafico, com a pergunta que o geraria).
const ROTEIRO = [
  // Primeiro: "como funciona o agendamento", "configurar", "apagar um agendamento" casariam em outras.
  // As palavras largas (sozinha, automático, todo dia) só contam junto de rodar/executar/atualizar:
  // "aprende sozinha", "conexão automática" e "vendemos todo dia" são de outras entradas ou do modelo.
  { id:'agendamento', casa:/agend|notific|me avis|avisar|(roda|rode|executa|atualiza)\w*\s+(\w+\s+){0,3}(sozinh|automatic|todo dia|toda hora|de hora em hora)|(todo dia|toda hora|de hora em hora)\s.*\b(rod|execut|atualiz)/,
    fecho:'o agendamento é da versão instalada no computador', blocos:[
    { p:'Na versão instalada, uma consulta salva roda **sozinha**: a cada X minutos (mínimo 5), de hora em hora, todo dia, semanal ou mensal.' },
    { lista:[
      'Datas viram **períodos** calculados a cada execução: hoje, ontem, esta semana, este mês, últimas X horas.',
      'Cada execução fica guardada no **histórico** da consulta e avisa com uma notificação do sistema e uma marca no ícone da IA de Bolso, mesmo com a janela fechada.',
      'Se a IA estiver fechada no horário, ela não roda depois sozinha: mostra o que não rodou e deixa você **rodar agora**.'
    ] },
    { p:'Exemplos: flash de vendas no horário de pico e ruptura de estoque toda manhã. Com dados fictícios:' },
    { grafico:'flash_de_vendas', sobre:'Flash de vendas', pergunta:'Quanto vendemos em cada hora hoje?' },
    { p:'Para ver como fica, abra os exemplos com ⏰ em **Consultas salvas**, na barra ao lado: cada um tem o histórico das execuções e a janela **Agendar**.' }
  ] },
  { id:'seguranca', casa:/segur|somente leitura|so leitura|apagar|estragar|perig|alterar meus dados/,
    evite:'dizer que é 100% seguro ou dar garantia absoluta; fale das travas',
    fecho:'a trava extra é o TI criar um usuário do banco só de leitura para mim', blocos:[
    { p:'Pode conectar sem medo: eu não consigo estragar nada no seu banco.' },
    { lista:[
      'Eu **leio** os dados do banco, mas **não escrevo** nada: todas as minhas ferramentas de banco de dados são somente leitura. Não existe ferramenta de escrita, então eu não apago, não altero e não insiro dados.',
      'Antes de rodar, cada consulta passa por uma trava que só deixa passar **SELECT**. Qualquer outra coisa é recusada.',
      'Consulta que demora demais é cancelada, para não pesar no banco da loja.',
      'O modelo de IA roda aqui, neste computador. A senha do banco fica guardada só nesta máquina e nunca volta para a tela.'
    ] },
    { p:'Mesmo com essas travas, a recomendação é que o TI da empresa crie um **usuário do banco só de leitura** para eu usar. É a trava extra, do lado do banco: se um dia alguém tentar escrever por mim, o próprio banco recusa.' }
  ] },
  { id:'aprendizado', casa:/entende|aprend|como (voce|vc) (le|funciona)|como funciona|demora|quanto tempo|inteligente|estud/,
    fecho:'isso pode levar vários minutos, dependendo do computador e de quantas tabelas o ERP tem; dá para ir perguntando enquanto isso, e a faixa com o robozinho no topo da tela mostra o progresso', blocos:[
    { p:'Quando você conecta o banco, eu faço como um funcionário novo no primeiro dia: dou uma volta pela loja antes de responder.' },
    { lista:[
      'Listo **todas as tabelas** do ERP.',
      'Leio **o começo de cada uma**, só umas poucas linhas, para ver que tipo de informação tem ali.',
      'Com a IA, **descrevo o que cada tabela faz**: o que é venda, o que é cadastro de produto, o que é estoque, o que é cliente.'
    ], numerada:true },
    { p:'Isso não atrapalha: dá para ir perguntando desde o primeiro minuto. Mas eu fico **mais inteligente a cada tabela que entendo**, e as respostas melhoram junto.' },
    { p:'Pode levar vários minutos, dependendo do computador e de quantas tabelas o ERP tem. É só acompanhar a faixa com o robozinho no topo da tela: ela mostra quantas tabelas já li e quantas já entendi. Quando aparecer **Banco entendido**, estou no meu melhor.' }
  ] },
  { id:'graficos', casa:/(que|quais|tipos? de) grafic|grafic\w* (voce|vc)|faz\w* grafic|dashboard/, blocos:[
    { p:'Conectada ao banco da sua loja, eu monto gráficos como estes direto da pergunta, em português, sem planilha. Os números abaixo são fictícios:' },
    { grafico:'vendas_por_mes', sobre:'Sobre vendas', pergunta:'Quanto vendemos por mês este ano?' },
    { grafico:'rupturas_criticas', args:{ limite:5 }, sobre:'Sobre ruptura', pergunta:'Quais produtos da curva A estão faltando nas lojas?' },
    { grafico:'perfil_do_cliente', sobre:'Sobre um cliente', pergunta:'Em que departamentos o cliente 0042 gasta mais?' },
    { p:'Cada resposta mostra a consulta usada e sai em CSV ou PDF com um clique.' }
  ] },
  { id:'ferramentas', casa:/ferramenta/, blocos:[
    { p:'Agora estou com as ferramentas da Demonstração, todas com dados fictícios de um supermercado de exemplo:' },
    { lista:[
      '**Vendas por departamento** do mês.', '**Vendas por mês** do ano.', '**Perdas do mês** por motivo.',
      '**Rupturas** da curva A, com a venda perdida.', '**Perfil de um cliente** do cartão fidelidade.'
    ] },
    { p:'Com o banco conectado, as ferramentas viram outras: eu listo as tabelas do seu ERP, leio a estrutura de cada uma e monto a consulta para responder qualquer pergunta sobre os seus dados, não só essas cinco.' }
  ] },
  // Antes de 'conectar': "não tenho acesso a um banco" também casaria lá.
  { id:'planilhas', casa:/planilha|excel|xlsx|nao (tenho|tem|temos) (acesso a (um )?)?(banco|erp|sistema)|sem (banco|erp|sistema)/,
    fecho:'as planilhas nunca são alteradas, porque eu leio uma cópia delas', blocos:[
    { p:'Não tem acesso a um banco? Comece com as suas **planilhas do Excel**. Se o controle da loja está numa pasta de planilhas, eu converso com elas do mesmo jeito.' },
    { lista:[
      'Abra a guia **Configurações** e escolha **MCP Nativo para Banco de dados**.',
      'Em **Motor**, escolha **Planilhas Excel**.',
      'Em **Pasta das planilhas**, informe a pasta com os arquivos `.xlsx`.',
      'Clique em **Salvar e conectar**.'
    ], numerada:true },
    { p:'Cada aba vira uma tabela. Eu acho o cabeçalho sozinha, mesmo com título em cima, e deixo de fora linhas de total e abas ocultas. As planilhas não são alteradas: eu leio uma cópia. Mudou a planilha? Clique em **↻ Atualizar com o banco**, na guia Conhecimento.' }
  ] },
  { id:'conectar', casa:/conect|conex|configur|ligar|banco de dados|meu banco|meu erp|dados reais|de verdade/,
    fecho:'o ideal é usar um usuário do banco só de leitura', blocos:[
    { p:'Para eu trabalhar com os números da sua loja, é rápido:' },
    { lista:[
      'Abra a guia **Configurações**, logo abaixo do topo da tela.',
      'Escolha **MCP Nativo para Banco de dados**.',
      'Preencha o motor (PostgreSQL, Oracle, MariaDB/MySQL, SQL Server, Firebird ou DBF), o endereço do servidor, a porta, a base, o usuário e a senha. Sem banco? Escolha **Planilhas Excel** e informe só a pasta das planilhas.',
      'Clique em **Salvar e conectar**.'
    ], numerada:true },
    { p:'Dica: peça ao TI um **usuário somente leitura** para mim. Minhas ferramentas já são só de leitura, mas essa é a trava extra do lado do banco.' },
    { p:'Depois de conectar, eu leio as tabelas do ERP para entender o banco. Dá para ir perguntando enquanto isso.' }
  ] },
  { id:'boas-vindas', casa:/o que (voce|vc) (consegue|faz|sabe)|o que da pra|como (voce|vc) (pode|ajuda)|^oi\b|^ola\b|^ajuda$|comecar|quem e (voce|vc)/, blocos:[
    { p:'Eu sou a **IA de Bolso da CD2**, criada especialmente para analisar ERPs de varejo. Meu lugar é dentro do banco de dados da sua loja: é lá que eu respondo com os seus números de verdade.' },
    { p:'Agora você está no **modo Demonstração**: tudo o que eu mostrar usa dados fictícios, só para você ver como funciona. Pode pedir vendas por departamento, vendas por mês, perdas, rupturas ou o perfil de um cliente. Por exemplo:' },
    { grafico:'vendas_por_departamento', sobre:'Sobre vendas', pergunta:'Mostre as vendas por departamento' },
    { p:'Conectada ao seu banco, você pergunta o que quiser, em português, e eu monto a consulta, o gráfico e o relatório. Na versão instalada, as consultas salvas ainda podem ser **agendadas** para rodar sozinhas e avisar quando ficam prontas.' }
  ] }
];
function norm(t){ return String(t || '').toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/\s+/g, ' ').trim(); }
function roteiro(pergunta){
  const q = norm(pergunta);
  if(!q) return null;
  return ROTEIRO.find(r => r.casa.test(q)) || null;
}
// O roteiro em texto corrido, para o histórico que o modelo vê depois.
function texto(r){
  const limpo = s => s.replace(/\*\*/g, '');
  return r.blocos.map(b => b.p ? limpo(b.p) : b.lista ? b.lista.map((x, i) => (b.numerada ? (i + 1) + '. ' : '- ') + limpo(x)).join('\n')
    : '[Gráfico fictício: ' + b.sobre + ', "' + b.pergunta + '"]').join('\n\n');
}

// Instrução para o modelo responder uma pergunta do roteiro em prosa própria, com os fatos dele.
function instrucao(r){
  const limpo = x => x.replace(/\*\*/g, '');
  const fatos = r.blocos.filter(b => !b.grafico).map(b => b.p ? limpo(b.p) : b.lista.map((x, i) => (b.numerada ? (i + 1) + '. ' : '- ') + limpo(x)).join('\n')).join('\n');
  const graficos = r.blocos.filter(b => b.grafico).map(b => b.sobre.toLowerCase() + ' ("' + b.pergunta + '")');
  return ' Para esta pergunta, responda com os fatos abaixo, reescritos com as suas palavras, em prosa curta e calorosa, de até oito frases, sem deixar nenhum fato de fora;'
    + ' se forem passos, use uma lista numerada. Mantenha o sentido exato de cada fato, sem exagerar nem inverter nada, sem prometer 100% ou garantia absoluta, e não invente nada além deles.\nFatos:\n' + fatos
    + (r.evite ? '\nEvite ' + r.evite + '.' : '')
    + (r.fecho ? '\nTermine a resposta dizendo, com as suas palavras, que ' + r.fecho + '.' : '')
    + (graficos.length ? '\nLogo abaixo da sua resposta a tela mostra gráficos de exemplo, com dados fictícios: ' + graficos.join('; ') + '. Convide a pessoa a olhar.' : '');
}

const CONVITE = { titulo:'Quer ver isso com os números da sua loja?',
  texto:'A IA de Bolso foi feita para analisar ERPs de varejo. Conecte o banco de dados do seu ERP em Configurações, de preferência com um usuário somente leitura. Sem ERP? Aponte a pasta das suas planilhas do Excel.' };

// Barra de consultas salvas na Demonstração: exemplos montados com os gráficos fictícios acima.
const EXEMPLOS_TOPO = 'Exemplos. Com o seu banco, você salva com 👍 as perguntas que mais usa, vê o resultado atualizado num clique e agenda para rodar sozinha.';
const EXEMPLO_FRASE = 'No seu banco, esta consulta traz os números atualizados num clique.';
// Os agendados (agenda) trazem a bolinha de execuções novas (novas); o flash tem um parâmetro de data
// fictício, para a janela Agendar mostrar os períodos.
const EXEMPLOS = [
  { id:'ex-flash', nome:'Flash de vendas de hoje', grafico:'flash_de_vendas', novas:2, params:[{ nome:'Data', tipo:'data' }],
    agenda:{ ligado:true, freq:'horas', a_cada:1, janela:{ de:'08:00', ate:'20:00' }, dias:[1, 2, 3, 4, 5, 6],
      params:[{ modo:'periodo', periodo:'hoje', ponta:'inicio' }] } },
  { id:'ex-departamentos', nome:'Vendas do mês por departamento', grafico:'vendas_por_departamento' },
  { id:'ex-meses', nome:'Vendas por mês no ano', grafico:'vendas_por_mes' },
  { id:'ex-ruptura', nome:'Curva A em ruptura nas lojas', grafico:'rupturas_criticas', args:{ limite:5 }, novas:1,
    agenda:{ ligado:true, freq:'semanal', a_cada:1, hora:'07:30', dias:[1, 2, 3, 4, 5, 6] } },
  { id:'ex-perdas', nome:'Maiores perdas do mês', grafico:'perdas_do_mes' },
  { id:'ex-cliente', nome:'Onde o cliente 0042 gasta', grafico:'perfil_do_cliente' }
];
const dataIso = d => d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate());
const hm = d => pad(d.getHours()) + ':' + pad(d.getMinutes());
// O flash vai até a hora em que rodou; os outros usam os args fixos.
const argsDe = (e, quando) => e.grafico === 'flash_de_vendas' ? { hora:quando.getHours() } : e.args;
const copia = o => JSON.parse(JSON.stringify(o));
// Os exemplos como a barra mostra, no formato de uma consulta salva: nome, tipo de gráfico (visual),
// o gráfico pronto para o card e, nos agendados, agenda, parâmetros e quantas execuções novas.
function exemplos(agora){
  agora = agora || new Date();
  return EXEMPLOS.map(e => {
    const g = grafico(e.grafico, argsDe(e, agora));
    const x = { id:e.id, nome:e.nome, visual:g.visual, g, novas:e.novas || 0 };
    if(e.agenda) x.agenda = copia(e.agenda);
    if(e.params) x.params = e.params.map(p => Object.assign({}, p, { valor:dataIso(agora) }));
    return x;
  });
}

/* histórico de exemplo dos agendados: execuções no formato do cd2-ctl (Execucao), relativas a agora */
const CONVITE_AGENDA = 'Na Demonstração os agendamentos são exemplos. Conecte o banco na versão instalada para agendar de verdade.';
const AVISO_NOTA = 'Na versão instalada, cada execução avisa assim, mesmo com a janela fechada.';
// Horários de cada agendado (minutos do dia) para achar a última execução antes de agora.
const SLOTS = { 'ex-flash':[20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8].map(h => h * 60), 'ex-ruptura':[450] };
// Último horário da agenda em ou antes de `ate` (dias de segunda a sábado, como os dois exemplos).
function ultimo(id, ate){
  for(let i = 0; i < 8; i++){
    const d = new Date(ate.getFullYear(), ate.getMonth(), ate.getDate() - i);
    if(d.getDay() === 0) continue;
    for(const m of SLOTS[id]){
      const h = new Date(d.getFullYear(), d.getMonth(), d.getDate(), Math.floor(m / 60), m % 60);
      if(h <= ate) return h;
    }
  }
  return null;
}
const inicioDoDia = d => new Date(d.getFullYear(), d.getMonth(), d.getDate());
const antes = d => new Date(d.getTime() - 1000);
const somaMin = (d, min) => new Date(d.getTime() + min * 60000);
function resultadoDe(e, quando){
  const g = grafico(e.grafico, argsDe(e, quando));
  return { ok:true, visual:g.visual, x:g.x, y:g.y, titulo:g.titulo, frase:g.frase, colunas:g.colunas, linhas:g.linhas, todas:g.linhas };
}
function execucao(e, tipo, ref, em, extra){
  return Object.assign({ tipo, ref:ref.toISOString(), em:em.toISOString(), valores:e.params ? [dataIso(ref)] : [],
    resultado:resultadoDe(e, ref) }, extra);
}
// Toda agendada, perda e erro cai num horário da agenda (no flash, de hora em hora das 08h às 20h, de
// segunda a sábado); a manual, dentro da janela. vistos: ids cujo histórico já foi aberto (sem NOVO).
function execucoes(id, agora, vistos){
  const e = EXEMPLOS.find(x => x.id === id);
  if(!e || !e.agenda) return [];
  const ag = (ref, extra) => execucao(e, 'agendada', ref, new Date(Math.min(ref.getTime() + 4000, agora.getTime())), extra);
  const s = [ultimo(id, agora)];
  if(id === 'ex-flash') while(s.length < 5) s.push(ultimo(id, antes(s[s.length - 1])));
  const ontem = ultimo(id, antes(inicioDoDia(s[s.length - 1]))), anteontem = ultimo(id, antes(inicioDoDia(ontem)));
  let l;
  if(id === 'ex-flash'){
    // A manual: 12 min depois da hora cheia anterior à última agendada, ou depois do erro se a última foi às 08h.
    const manual = s[0].getHours() > 8 ? somaMin(s[0], -48) : somaMin(s[4], 12);
    l = [ag(s[0], { novo:true }),
      execucao(e, 'manual', manual, manual),
      { tipo:'perda', ref:s[1].toISOString(), em:new Date(Math.min(somaMin(s[1], 50).getTime(), agora.getTime())).toISOString(), valores:[],
        motivo:'fechada', n:3, de:s[3].toISOString(), ate:s[1].toISOString(), novo:true },
      ag(s[4], { resultado:{ ok:false, erro:'Não consegui falar com o banco.' } }),
      ag(ontem), ag(ultimo(id, antes(ontem))), ag(anteontem)];
  } else {
    const manual = somaMin(inicioDoDia(ontem), 15 * 60 + 4);
    l = [ag(s[0], { novo:true }), execucao(e, 'manual', manual, manual), ag(ontem), ag(anteontem)];
  }
  // Da mais nova para a mais velha, pelo horário em que cada uma aparece (a perda no último perdido).
  const quando = x => new Date(x.tipo === 'perda' ? x.ate : x.tipo === 'manual' ? x.em : x.ref).getTime();
  l.sort((a, b) => quando(b) - quando(a));
  const visto = !!(vistos && vistos.has(id)), base = id === 'ex-flash' ? 140 : 90;
  return l.map((x, i) => { const y = Object.assign({ seq:base + l.length - i }, x); if(visto) delete y.novo; return y; });
}
// ▶ Rodar agora na Demonstração: execução manual na hora; da perda, referente ao último horário perdido.
function rodar(id, agora, seq, perda){
  const e = EXEMPLOS.find(x => x.id === id);
  if(!e) return null;
  const ref = perda ? new Date(perda.ate) : agora;
  return Object.assign({ seq }, execucao(e, 'manual', ref, agora));
}
// O aviso de exemplo do topo do histórico: a notificação do sistema e o menu do ícone na bandeja.
// vistos: os históricos já abertos saem da contagem (a marca e o menu contam só o que falta ver).
function aviso(id, agora, vistos){
  const e = EXEMPLOS.find(x => x.id === id);
  if(!e || !e.agenda) return null;
  const ult = execucoes(id, agora).find(x => x.tipo === 'agendada' && x.resultado.ok);
  const ags = EXEMPLOS.filter(x => x.agenda && x.novas && !(vistos && vistos.has(x.id))), total = ags.reduce((s, x) => s + x.novas, 0);
  return { cabecalho:'Como o aviso chega', titulo:'IA de Bolso · ' + e.nome, texto:'Rodou às ' + hm(new Date(ult.ref)) + ' · ' + ult.resultado.frase,
    hora:hm(agora), n:total, novos:!total ? 'Nenhum resultado novo' : total + (total === 1 ? ' resultado novo' : ' resultados novos'),
    itens:ags.map(x => '⏰ ' + x.nome + ' (' + x.novas + ')'), nota:AVISO_NOTA };
}

const api = { TAG, TOOLS, PROMPT, ROTEIRO, CONVITE, CONVITE_AGENDA, EXEMPLOS, EXEMPLOS_TOPO, EXEMPLO_FRASE, call, grafico, roteiro, texto, instrucao, norm,
  exemplos, execucoes, rodar, aviso };
if(typeof module !== 'undefined' && module.exports) module.exports = api; else root.CD2Demo = api;
})(typeof window !== 'undefined' ? window : globalThis);
