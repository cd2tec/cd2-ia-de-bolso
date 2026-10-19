// Páginas HTML do Worker. Todo valor dinâmico passa por esc() antes de entrar no HTML.

export function esc(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]);
}

const ESTILO = `
:root{--bg:#f2f2f2;--ink:#0d1233;--muted:#464a53;--brand:#0000ff;--brand-hover:#0333f7}
*{box-sizing:border-box}
body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;padding:24px;
 background:var(--bg);color:var(--ink);font-family:"Manrope","Segoe UI",Arial,sans-serif;line-height:1.5}
main{background:#fff;border-radius:24px;padding:40px 32px;max-width:640px;width:100%}
h1{margin:0 0 12px;font-size:1.75rem;font-weight:800;color:var(--brand)}
p{margin:0 0 16px;color:var(--muted)}
.chave{display:block;word-break:break-all;font-family:ui-monospace,Menlo,Consolas,monospace;font-size:.95rem;
 background:var(--bg);border-radius:12px;padding:16px;margin:0 0 20px;color:var(--ink);user-select:all}
.acoes{display:flex;flex-wrap:wrap;gap:12px}
.botao{display:inline-block;border:2px solid var(--brand);border-radius:999px;padding:12px 24px;font:inherit;font-weight:700;
 text-decoration:none;cursor:pointer;background:var(--brand);color:#fff}
.botao:hover{background:var(--brand-hover)}
.botao.vazado{background:transparent;color:var(--brand)}
.botao.vazado:hover{background:var(--bg)}
`;

function pagina(titulo, corpo, extra = '') {
  return `<!doctype html>
<html lang="pt-BR">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex">
<title>${esc(titulo)}</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Manrope:wght@400;600;700;800&display=swap" rel="stylesheet">
<style>${ESTILO}</style>
</head>
<body>
<main>
${corpo}
</main>
${extra}
</body>
</html>`;
}

export function paginaErro(titulo, mensagem) {
  return pagina(titulo, `<h1>${esc(titulo)}</h1>\n<p>${esc(mensagem)}</p>`);
}

export function paginaCancelou(doInstalador = false) {
  if (doInstalador) {
    return pagina('Pagamento cancelado',
      '<h1>Pagamento cancelado</h1>\n<p>Tudo bem, nada foi cobrado. Pode fechar esta página e voltar para o instalador.</p>');
  }
  return pagina('Pagamento cancelado',
    '<h1>Tudo bem, a IA continua no pen drive.</h1>\n<p>Nada foi cobrado. Você já pode fechar esta aba e voltar para a IA de Bolso.</p>');
}

export function paginaChave(chave, porta, doInstalador = false) {
  const voltar = porta ? `http://127.0.0.1:${porta}/?chave=${encodeURIComponent(chave)}` : null;
  const aviso = voltar
    ? '<p>Guarde a chave e depois use o botão para voltar para ' + (doInstalador ? 'o instalador' : 'a IA') + '.</p>'
    : '<p>Copie a chave e cole na tela de instalação da IA de Bolso.</p>';
  const botaoVoltar = voltar ? `<a class="botao vazado" id="voltar" href="${esc(voltar)}">${doInstalador ? 'Voltar para o instalador' : 'Voltar para a IA'}</a>` : '';
  const script = `<script>
(function(){
  var chave=${JSON.stringify(chave).replace(/</g, '\\u003c')};
  var b=document.getElementById('copiar');
  b.addEventListener('click',function(){
    function ok(){b.textContent='Chave copiada';}
    if(navigator.clipboard&&navigator.clipboard.writeText){navigator.clipboard.writeText(chave).then(ok,function(){});}
    else{var r=document.createRange();r.selectNodeContents(document.getElementById('chave'));
      var s=getSelection();s.removeAllRanges();s.addRange(r);try{document.execCommand('copy');ok();}catch(e){}}
  });
})();
</script>`;
  return pagina('Sua chave de instalação',
    `<h1>Pagamento confirmado</h1>
<p>Esta é a sua chave de instalação. Guarde uma cópia: ela também está na fatura enviada para o seu e-mail.</p>
<code class="chave" id="chave">${esc(chave)}</code>
${aviso}
<div class="acoes">
<button class="botao" id="copiar" type="button">Copiar a chave</button>
${botaoVoltar}
</div>`, script);
}
