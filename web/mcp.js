/* Cliente MCP do CD2 Harness: Streamable HTTP + JSON-RPC, falando direto do
   navegador com o gateway do Archeo (/api/mcp).

   Protocolo 2025-06-18: a versão vai no corpo do initialize e, depois dele, no
   header MCP-Protocol-Version de todo request. O gateway do Archeo exige isso.

   Autenticação por provedor: `token()` é chamada a cada request (o OAuth
   renova sozinho). Num 401, `onUnauthorized()` tenta um token novo e o
   request é repetido UMA vez; falhou de novo, é erro de autenticação. */
(function(root){
"use strict";
const PROTOCOL_VERSION = '2025-06-18';
const CLIENT_INFO = { name:'cd2-harness', version:'0.14.0' };

function parseSse(text, id){
  for(const block of text.split(/\r?\n\r?\n/)){
    const data = block.split(/\r?\n/).filter(l => l.startsWith('data:')).map(l => l.slice(5).trim()).join('\n');
    if(!data) continue;
    try{ const m = JSON.parse(data); if(m.id === id) return m; }catch(e){}
  }
  throw new Error('Resposta MCP sem resultado');
}
async function bodyOf(r){
  const ct = r.headers.get('Content-Type') || '';
  if(ct.includes('application/json')){ try{ return await r.json(); }catch(e){ return null; } }
  try{ return await r.text(); }catch(e){ return null; }
}

class McpClient{
  // opts: { fetch, token: async () => string|null, onUnauthorized?: async () => string|null, origin?: string }
  constructor(url, opts){
    opts = opts || {};
    this.url = url;
    this.fetch = opts.fetch || ((...a) => root.fetch(...a));
    this.token = opts.token || (async () => null);
    this.onUnauthorized = opts.onUnauthorized || null;
    this.origin = opts.origin || (root.location && root.location.origin) || '';
    this.sid = null; this.n = 0; this.version = null;
  }
  async _send(method, params, notif, tok, signal){
    const body = { jsonrpc:'2.0', method }; if(params) body.params = params; if(!notif) body.id = ++this.n;
    const h = { 'Content-Type':'application/json', 'Accept':'application/json, text/event-stream' };
    if(tok) h.Authorization = 'Bearer ' + tok;
    if(this.sid) h['Mcp-Session-Id'] = this.sid;
    if(this.version && method !== 'initialize') h['MCP-Protocol-Version'] = this.version;
    const init = { method:'POST', headers:h, body:JSON.stringify(body) };
    if(signal) init.signal = signal;
    const r = await this.fetch(this.url, init);
    return { r, id: body.id };
  }
  // signal (opcional) cancela o request: Parar e o prazo do roteiro de relatórios usam.
  async rpc(method, params, notif, signal){
    let tok = await this.token();
    let { r, id } = await this._send(method, params, notif, tok, signal);
    if(r.status === 401 && this.onUnauthorized){
      const fresh = await this.onUnauthorized();
      if(fresh) ({ r, id } = await this._send(method, params, notif, fresh, signal));
    }
    if(r.status === 401) throw new Error('O Archeo não aceitou a autenticação. Entre de novo.');
    const sid = r.headers.get('Mcp-Session-Id'); if(sid) this.sid = sid;
    if(!r.ok){
      const b = await bodyOf(r);
      if(r.status === 403 && b && b.reason === 'ORIGIN_REJECTED') throw new Error('O Archeo recusou a origem ' + this.origin + '. Peça para liberar essa origem no servidor (DATABASE_MCP_BROWSER_ORIGINS).');
      throw new Error('MCP respondeu HTTP ' + r.status);
    }
    if(notif) return null;
    const ct = r.headers.get('Content-Type') || '';
    const msg = ct.includes('text/event-stream') ? parseSse(await r.text(), id) : await r.json();
    if(msg.error) throw new Error(msg.error.message || 'erro MCP');
    return msg.result;
  }
  async init(){
    const res = await this.rpc('initialize', { protocolVersion: PROTOCOL_VERSION, capabilities:{}, clientInfo: CLIENT_INFO });
    const v = res && res.protocolVersion;
    if(v !== PROTOCOL_VERSION) throw new Error('O servidor MCP respondeu com a versão de protocolo ' + v + '; este harness fala ' + PROTOCOL_VERSION + '.');
    this.version = v;
    await this.rpc('notifications/initialized', null, true);
    const r = await this.rpc('tools/list', {});
    return r.tools || [];
  }
  async call(name, args, signal){
    const r = await this.rpc('tools/call', { name, arguments: args }, false, signal);
    const text = (r.content || []).map(c => c.type === 'text' ? c.text : '[' + c.type + ']').join('\n');
    return (r.isError ? 'ERRO: ' : '') + text;
  }
}

const api = { McpClient, parseSse, PROTOCOL_VERSION, CLIENT_INFO };
if(typeof module !== 'undefined' && module.exports) module.exports = api;
else { root.McpClient = McpClient; root.CD2Mcp = api; }
})(typeof window !== 'undefined' ? window : globalThis);
