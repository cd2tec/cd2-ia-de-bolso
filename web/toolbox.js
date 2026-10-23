/* Modo "MCP Nativo para Banco de dados" do CD2 Harness: lógica sem DOM.
   O toolbox (googleapis/mcp-toolbox) sobe pelo launcher em 127.0.0.1 na porta
   do harness + 1000; o front só deriva a URL. A guarda de SELECT é a terceira
   camada de leitura (conta do banco e excludedValues do YAML são as outras). */
(function(root){
"use strict";
const ENGINES = {
  postgres:  { label:'PostgreSQL',      port:5432 },
  oracle:    { label:'Oracle',          port:1521 },
  mariadb:   { label:'MariaDB / MySQL', port:3306 },
  sqlserver: { label:'SQL Server',      port:1433 },
  firebird:  { label:'Firebird',        port:3050 },
  dbf:       { label:'DBF',             port:0 },
  excel:     { label:'Planilhas Excel', port:0 }
};
// Exemplos dos campos do formulário do banco. Só exemplos genéricos: nada de
// host, usuário ou senha reais, que circulariam com o pen drive.
const PLACEHOLDERS = {
  postgres:  { host:'ex.: pg.empresa.local',    base:'ex.: erp',      usuario:'ex.: leitura_ro' },
  oracle:    { host:'ex.: ora.empresa.local',   base:'ex.: ORCLPDB1', usuario:'ex.: LEITURA_RO' },
  mariadb:   { host:'ex.: mysql.empresa.local', base:'ex.: erp',      usuario:'ex.: leitura_ro' },
  sqlserver: { host:'ex.: mssql.empresa.local', base:'ex.: ERP',      usuario:'ex.: leitura_ro' },
  firebird:  { host:'ex.: 192.168.0.10',        base:'ex.: C:\\dados\\erp.fdb', usuario:'ex.: LEITURA_RO' },
  dbf:       { host:'ex.: nao usado',            base:'ex.: C:\\dados\\loja',    usuario:'ex.: nao usado' },
  excel:     { host:'ex.: nao usado',            base:'ex.: C:\\Controle da loja', usuario:'ex.: nao usado' }
};
const TOOL_DESCRIPTIONS = {
  db_catalog:           'Lê o mapa do banco (tabelas e colunas) para o roteiro de relatórios.',
  db_connection_status: 'Verifica se o banco está acessível e se a conexão está pronta para uso.',
  db_describe_table:    'Apresenta as colunas, os tipos e a estrutura de uma tabela.',
  db_list_tables:       'Lista as tabelas e views disponíveis no banco de dados.',
  db_search_columns:    'Encontra colunas pelo nome em todas as tabelas e schemas.',
  db_read_only_query:   'Executa uma consulta SELECT avulsa em modo somente leitura.',
  db_sample_table:      'Mostra uma amostra de até 20 linhas para entender o conteúdo de uma tabela.'
};
// Uniao das listas dos YAML (base + SQL Server + PostgreSQL). \b protege nomes
// como offset, settings e use_count. T-SQL aceita varios comandos sem ";".
const FORBIDDEN = new RegExp('\\b(' + [
  'insert','update','delete','drop','alter','truncate','create','grant','revoke','merge','exec','execute','call','into',
  'backup','restore','kill','shutdown','dbcc','deny','reconfigure','waitfor','declare','set','use','openrowset','openquery','opendatasource',
  'copy','pg_terminate_backend','pg_cancel_backend','set_config','nextval','setval','dblink\\w*','lo_import','lo_export',
  'pg_read_file','pg_read_binary_file','pg_write\\w*','pg_sleep\\w*'
].join('|') + ')\\b', 'i');

function engineLabel(e){ return (ENGINES[e] && ENGINES[e].label) || 'Banco de dados'; }
function urlFor(loc){ const p = parseInt(loc.port, 10) || 80; return 'http://127.0.0.1:' + (p + 1000) + '/mcp'; }

function guardSql(sql){
  const bad = m => ({ ok:false, error:'Consulta recusada: só SELECT é permitido (' + m + '). Reescreva como um único SELECT, sem ponto e vírgula.' });
  if(typeof sql !== 'string') return bad('texto vazio');
  let s = sql.replace(/--[^\n]*/g, '').replace(/\/\*[\s\S]*?\*\//g, '').trim();
  s = s.replace(/;\s*$/, '').trim();
  if(!s) return bad('texto vazio');
  if(s.includes(';')) return bad('ponto e vírgula no meio da consulta');
  if(!/^(select|with)\b/i.test(s)) return bad('não começa com SELECT ou WITH');
  const m = s.match(FORBIDDEN); if(m) return bad('palavra ' + m[1].toUpperCase());
  return { ok:true, sql:s };
}

// O toolbox envolve a consulta numa subconsulta (TOP (200) no SQL Server), e o SQL Server
// não aceita ORDER BY em subconsulta sem TOP ou OFFSET. Só o nível externo conta (OVER (ORDER BY) e
// subconsultas não). Os outros motores passam intactos.
// Devolve só o texto em profundidade 0 de parênteses, sem literais '...', "..." e [...].
function topLevelSql(sql){
  let out = '', depth = 0, i = 0;
  while(i < sql.length){
    const c = sql[i];
    if(c === "'" || c === '"' || c === '['){
      const close = c === '[' ? ']' : c;
      i++;
      while(i < sql.length){
        if(sql[i] === close){ if(close !== ']' && sql[i+1] === close){ i += 2; continue; } break; }
        i++;
      }
      i++; out += ' '; continue;
    }
    if(c === '(') depth++;
    else if(c === ')') { if(depth > 0) depth--; }
    else if(depth === 0) out += c;
    i++;
  }
  return out;
}
function prepareSql(sql, motor){
  if(motor !== 'sqlserver') return sql;
  const top = topLevelSql(sql);
  if(/\border\s+by\b/i.test(top) && !/\b(top|offset)\b/i.test(top)) return sql + ' OFFSET 0 ROWS';
  return sql;
}

function parseStatus(text){
  try{
    const j = JSON.parse(text);
    const row = Array.isArray(j) ? j[0] : j;
    if(!row || typeof row !== 'object' || !row.engine) return null;
    return row;
  }catch(e){ return null; }
}
// Ao abrir a página: o toolbox respondeu, mas há banco de verdade do outro lado? Só se o
// db_connection_status trouxer o motor e o db_catalog tiver sido lido (catalogo: parseRows dele).
function bancoAtivo(statusTxt, catalogo){
  return !!parseStatus(statusTxt) && !!(catalogo && catalogo.ok);
}
function statusLines(s){
  const v = x => (x == null ? '' : String(x));
  return [
    ['Motor', engineLabel(s.engine)],
    ['Versão', v(s.db_version).split('\n')[0].trim()],
    ['Host', v(s.host)], ['Base', v(s.database)], ['Usuário', v(s.user)], ['Schema', v(s.schema)],
    ['Tabelas visíveis', v(s.visible_tables)]
  ];
}

/* ---------- cd2-ctl: controle do banco na porta do harness + 3000 ---------- */
/* portas que Chrome e Firefox bloqueiam (ERR_UNSAFE_PORT); o trio P, P+1000, P+3000 nao pode cair aqui */
const UNSAFE_PORTS = [1,7,9,11,13,15,17,19,20,21,22,23,25,37,42,43,53,69,77,79,87,95,101,102,103,104,109,110,111,113,115,117,119,123,135,137,139,143,161,179,389,427,465,512,513,514,515,526,530,531,532,540,548,554,556,563,587,601,636,989,990,993,995,1719,1720,1723,2049,3659,4045,4190,5060,5061,6000,6566,6665,6666,6667,6668,6669,6679,6697,10080];
function ctlUrlFor(loc){ const p = parseInt(loc.port, 10) || 80; return 'http://127.0.0.1:' + (p + 3000); }
// A chave da sessão chega uma vez no endereço (#k=<64 hex>), posta pelo launcher.
function readKeyFromHash(hash){
  const m = /(?:^#|&)k=([0-9a-fA-F]{64})(?=&|$)/.exec(String(hash || ''));
  return m ? m[1] : null;
}
// Mesmas regras do cd2-ctl (e da v0.5.0): Oracle usa o usuário em maiúsculas, MariaDB a base.
function defaultsFor(motor, usuario, base){
  const e = ENGINES[motor];
  if(!e) return { porta:'', schema:'' };
  const schema = { postgres:'public', oracle:String(usuario || '').toUpperCase(), mariadb:String(base || ''), sqlserver:'dbo', firebird:'', dbf:'', excel:'' }[motor];
  return { porta:e.port, schema };
}
// Exemplos do formulário para o motor. O schema repete a regra do defaultsFor e,
// quando ela depende de um campo ainda vazio, usa o exemplo desse campo.
function placeholdersFor(motor, usuario, base){
  const p = PLACEHOLDERS[motor];
  if(!p) return { host:'', base:'', usuario:'', senha:'', schema:'' };
  const d = defaultsFor(motor, usuario, base);
  const fallback = { oracle:'LEITURA_RO', mariadb:'erp', firebird:'nao usado', dbf:'nao usado', excel:'nao usado' }[motor] || '';
  return { host:p.host, base:p.base, usuario:p.usuario, senha:'Senha da conta de leitura', schema:'ex.: ' + (d.schema || fallback) };
}
// Porta ao trocar de motor: vazia ou ainda na porta padrão de algum motor vira a
// padrão do motor novo; porta digitada pela pessoa fica como está.
function portFor(motor, atual){
  const a = String(atual == null ? '' : atual).trim();
  const e = ENGINES[motor];
  if(!e) return a;
  const padrao = Object.keys(ENGINES).some(k => String(ENGINES[k].port) === a);
  return (a === '' || padrao) ? String(e.port) : a;
}
// Corpo de POST /config. A senha não é aparada (espaço pode fazer parte dela) e só vai se
// foi digitada: vazia significa "manter a salva".
function configPayload(form){
  const t = v => String(v == null ? '' : v).trim();
  const out = { motor:t(form.motor), host:t(form.host), base:t(form.base), usuario:t(form.usuario), schema:t(form.schema) };
  const p = t(form.porta);
  if(p) out.porta = /^\d+$/.test(p) ? Number(p) : p;
  const s = form.senha == null ? '' : String(form.senha);
  if(s) out.senha = s;
  return out;
}
function ctlStatusLines(cs){
  const v = x => (x == null ? '' : String(x));
  return [
    ['Motor', engineLabel(cs.motor)], ['Host', v(cs.host)], ['Porta', v(cs.porta)],
    ['Base', v(cs.base)], ['Usuário', v(cs.usuario)], ['Schema', v(cs.schema)]
  ];
}

// Motores que leem uma pasta de arquivos em vez de falar com um servidor.
function motorArquivo(motor){ return motor === 'excel' || motor === 'dbf'; }
// Relatório da última leitura das planilhas (campo planilhas do /status do cd2-ctl).
function planilhasLinha(p){
  const a = Number(p && p.arquivos) || 0, t = Number(p && p.tabelas) || 0;
  const arq = a + (a === 1 ? ' arquivo' : ' arquivos');
  const abas = t === 0 ? 'nenhuma aba virou tabela' : (t === 1 ? '1 aba virou tabela' : t + ' abas viraram tabelas');
  return arq + ', ' + abas;
}
// Uma linha por aba (ou arquivo inteiro, com aba vazia) deixada de fora. Nome de arquivo e de aba vêm do disco: só texto.
function ignoradasLinhas(p){
  const lista = (p && Array.isArray(p.ignoradas)) ? p.ignoradas : [];
  return lista.map(i => String(i.arquivo == null ? '' : i.arquivo) + (i.aba ? ' › ' + i.aba : '') + ': ' + String(i.motivo == null ? '' : i.motivo));
}

const api = { ENGINES, TOOL_DESCRIPTIONS, engineLabel, urlFor, guardSql, prepareSql, parseStatus, bancoAtivo, statusLines,
  UNSAFE_PORTS, ctlUrlFor, readKeyFromHash, defaultsFor, placeholdersFor, portFor, configPayload, ctlStatusLines, motorArquivo, planilhasLinha, ignoradasLinhas };
if(typeof module !== 'undefined' && module.exports) module.exports = api; else root.CD2Toolbox = api;
})(typeof window !== 'undefined' ? window : globalThis);
