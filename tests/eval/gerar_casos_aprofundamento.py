"""Gera os casos de aprofundamento da bateria da História 6 (CIDB-26) a partir do erp_pdv fictício.

Piloto (a confirmar pelo produto): as consultas de flash de vendas e de ruptura, os exemplos do brief.
Cada caso é uma consulta agendada com condição que dispara, o resultado dela, 3 consultas vinculadas
como aprofundamento (com o resultado de cada uma, para o resumo usar o da escolhida) e a escolha
esperada. "aceitaveis" lista as escolhas que também contam como acerto quando mais de uma explica o
alerta; "esperada" é a melhor. O porquê de cada gabarito vai em "porque", para revisão do produto.

Precisa do container erp-pdv-mock no ar (~/work/cd2/erp-pdv-mock). Uso:
    python3 tests/eval/gerar_casos_aprofundamento.py > tests/eval/aprofundamento-casos.json
"""
import json
import subprocess
import sys

FIM = "(SELECT max(data_hora)::date FROM venda)"


def sql(q):
    """Linhas de uma consulta como lista de dicts (json_agg pelo psql do container)."""
    out = subprocess.run(
        ["docker", "exec", "erp-pdv-mock", "psql", "-U", "leitura_ro", "-d", "erp_pdv", "-At", "-c",
         f"SELECT coalesce(json_agg(t), '[]') FROM ({q}) t"],
        check=True, capture_output=True, text=True).stdout
    return json.loads(out)


def periodo(dias, desloc=0, campo="v.data_hora"):
    return f"{campo} >= {FIM} - {dias + desloc - 1} AND {campo} < {FIM} + 1 - {desloc}"


VENDA_OK = "v.status = 'concluida'"

# Consultas que disparam o alerta: (nome, x, y, sql(desloc)). desloc=7: a semana anterior.
PRINCIPAIS = {
    "fat_dia": ("Faturamento por dia nos últimos 7 dias", "data", "faturamento",
                lambda d=0: f"SELECT v.data_hora::date AS data, sum(v.valor_total) AS faturamento FROM venda v "
                            f"WHERE {VENDA_OK} AND {periodo(7, d)} GROUP BY 1 ORDER BY 1"),
    "fat_loja": ("Faturamento por loja nos últimos 7 dias", "loja", "faturamento",
                 lambda d=0: f"SELECT l.nome AS loja, sum(v.valor_total) AS faturamento FROM venda v JOIN loja l ON l.id = v.loja_id "
                             f"WHERE {VENDA_OK} AND {periodo(7, d)} GROUP BY 1 ORDER BY 2 DESC"),
    "canc_dia": ("Cupons cancelados por dia nos últimos 7 dias", "data", "cancelados",
                 lambda d=0: f"SELECT v.data_hora::date AS data, count(*) AS cancelados FROM venda v "
                             f"WHERE v.status = 'cancelada' AND {periodo(7, d)} GROUP BY 1 ORDER BY 1"),
    "ticket_loja": ("Ticket médio por loja nos últimos 7 dias", "loja", "ticket_medio",
                    lambda d=0: f"SELECT l.nome AS loja, round(avg(v.valor_total), 2) AS ticket_medio FROM venda v JOIN loja l ON l.id = v.loja_id "
                                f"WHERE {VENDA_OK} AND {periodo(7, d)} GROUP BY 1 ORDER BY 2 DESC"),
    "fat_pag": ("Faturamento por forma de pagamento nos últimos 7 dias", "forma", "valor",
                lambda d=0: f"SELECT f.descricao AS forma, sum(p.valor) AS valor FROM venda_pagamento p "
                            f"JOIN forma_pagamento f ON f.id = p.forma_pagamento_id JOIN venda v ON v.id = p.venda_id "
                            f"WHERE {VENDA_OK} AND {periodo(7, d)} GROUP BY 1 ORDER BY 2 DESC"),
    "rup_loja": ("Ruptura de estoque por loja", "loja", "itens_em_ruptura",
                 lambda d=0: "SELECT l.nome AS loja, count(*) AS itens_em_ruptura FROM estoque e JOIN loja l ON l.id = e.loja_id "
                             "WHERE e.quantidade = 0 GROUP BY 1 ORDER BY 2 DESC"),
    "min_loja": ("Itens abaixo do estoque mínimo por loja", "loja", "itens",
                 lambda d=0: "SELECT l.nome AS loja, count(*) AS itens FROM estoque e JOIN loja l ON l.id = e.loja_id "
                             "WHERE e.quantidade < e.estoque_minimo GROUP BY 1 ORDER BY 2 DESC"),
    "rupA_loja": ("Produtos curva A em ruptura por loja", "loja", "itens",
                  lambda d=0: "SELECT l.nome AS loja, count(*) AS itens FROM estoque e JOIN loja l ON l.id = e.loja_id "
                              "JOIN produto p ON p.id = e.produto_id WHERE e.quantidade = 0 AND p.curva_abc = 'A' GROUP BY 1 ORDER BY 2 DESC"),
    "rup_dep": ("Ruptura de estoque por departamento", "departamento", "itens_em_ruptura",
                lambda d=0: "SELECT d.nome AS departamento, count(*) AS itens_em_ruptura FROM estoque e "
                            "JOIN produto p ON p.id = e.produto_id JOIN departamento d ON d.id = p.departamento_id "
                            "WHERE e.quantidade = 0 GROUP BY 1 ORDER BY 2 DESC"),
    "perda_motivo": ("Perdas por motivo nos últimos 30 dias", "motivo", "valor_custo",
                     lambda d=0: f"SELECT pe.motivo, sum(pe.valor_custo) AS valor_custo FROM perda pe "
                                 f"WHERE {periodo(30, d, 'pe.data')} GROUP BY 1 ORDER BY 2 DESC"),
}

# Consultas que podem ser vinculadas como aprofundamento.
OPCOES = {
    "fat_loja": ("Faturamento por loja nos últimos 7 dias", PRINCIPAIS["fat_loja"][3]()),
    "fat_dep": ("Faturamento por departamento nos últimos 7 dias",
                f"SELECT d.nome AS departamento, sum(i.valor_total) AS faturamento FROM venda_item i "
                f"JOIN venda v ON v.id = i.venda_id JOIN produto p ON p.id = i.produto_id JOIN departamento d ON d.id = p.departamento_id "
                f"WHERE {VENDA_OK} AND {periodo(7)} GROUP BY 1 ORDER BY 2 DESC"),
    "fat_pag": ("Faturamento por forma de pagamento nos últimos 7 dias", PRINCIPAIS["fat_pag"][3]()),
    "fat_hora": ("Faturamento por hora do dia nos últimos 7 dias",
                 f"SELECT extract(hour FROM v.data_hora)::int AS hora, sum(v.valor_total) AS faturamento FROM venda v "
                 f"WHERE {VENDA_OK} AND {periodo(7)} GROUP BY 1 ORDER BY 1"),
    "canc_loja": ("Cupons cancelados por loja nos últimos 7 dias",
                  f"SELECT l.nome AS loja, count(*) AS cancelados FROM venda v JOIN loja l ON l.id = v.loja_id "
                  f"WHERE v.status = 'cancelada' AND {periodo(7)} GROUP BY 1 ORDER BY 2 DESC"),
    "canc_op": ("Cupons cancelados por operador nos últimos 7 dias",
                f"SELECT o.nome AS operador, count(*) AS cancelados FROM venda v JOIN operador o ON o.id = v.operador_id "
                f"WHERE v.status = 'cancelada' AND {periodo(7)} GROUP BY 1 ORDER BY 2 DESC LIMIT 10"),
    "top_prod": ("Top 10 produtos por faturamento nos últimos 7 dias",
                 f"SELECT p.descricao AS produto, sum(i.valor_total) AS faturamento FROM venda_item i "
                 f"JOIN venda v ON v.id = i.venda_id JOIN produto p ON p.id = i.produto_id "
                 f"WHERE {VENDA_OK} AND {periodo(7)} GROUP BY 1 ORDER BY 2 DESC LIMIT 10"),
    "desc_loja": ("Desconto concedido por loja nos últimos 7 dias",
                  f"SELECT l.nome AS loja, sum(v.desconto) AS desconto FROM venda v JOIN loja l ON l.id = v.loja_id "
                  f"WHERE {VENDA_OK} AND {periodo(7)} GROUP BY 1 ORDER BY 2 DESC"),
    "rup_loja": ("Ruptura de estoque por loja", PRINCIPAIS["rup_loja"][3]()),
    "rup_dep": ("Ruptura de estoque por departamento", PRINCIPAIS["rup_dep"][3]()),
    "rupA_prod": ("Produtos curva A em ruptura",
                  "SELECT l.nome AS loja, p.descricao AS produto FROM estoque e JOIN loja l ON l.id = e.loja_id "
                  "JOIN produto p ON p.id = e.produto_id WHERE e.quantidade = 0 AND p.curva_abc = 'A' ORDER BY 1, 2 LIMIT 20"),
    "rup_forn": ("Ruptura de estoque por fornecedor",
                 "SELECT f.razao_social AS fornecedor, count(*) AS itens_em_ruptura FROM estoque e "
                 "JOIN produto p ON p.id = e.produto_id JOIN fornecedor f ON f.id = p.fornecedor_id "
                 "WHERE e.quantidade = 0 GROUP BY 1 ORDER BY 2 DESC LIMIT 10"),
    "compras_pend": ("Compras pendentes por fornecedor",
                     "SELECT f.razao_social AS fornecedor, count(*) AS pedidos, sum(c.valor_total) AS valor FROM compra c "
                     "JOIN fornecedor f ON f.id = c.fornecedor_id WHERE c.status = 'pendente' GROUP BY 1 ORDER BY 3 DESC LIMIT 10"),
    "perda_loja": ("Perdas por loja nos últimos 30 dias",
                   f"SELECT l.nome AS loja, sum(pe.valor_custo) AS valor_custo FROM perda pe JOIN loja l ON l.id = pe.loja_id "
                   f"WHERE {periodo(30, 0, 'pe.data')} GROUP BY 1 ORDER BY 2 DESC"),
    "perda_prod": ("Produtos com mais perda nos últimos 30 dias",
                   f"SELECT p.descricao AS produto, sum(pe.valor_custo) AS valor_custo FROM perda pe JOIN produto p ON p.id = pe.produto_id "
                   f"WHERE {periodo(30, 0, 'pe.data')} GROUP BY 1 ORDER BY 2 DESC LIMIT 10"),
    "clientes_cidade": ("Clientes cadastrados por cidade",
                        "SELECT c.cidade, count(*) AS clientes FROM cliente c GROUP BY 1 ORDER BY 2 DESC LIMIT 10"),
}

# (principal, regra, opções na ordem do vínculo, esperada, aceitáveis além da esperada, porquê)
# regra: maior_soma | menor_min | maximo | caiu | subiu | primeira
CASOS = [
    # ---- flash de vendas ----
    ("fat_dia", "maior_soma", ["clientes_cidade", "fat_loja", "rup_dep"], "fat_loja", [],
     "faturamento acima do limite: abrir por loja mostra de onde veio; as outras não falam de venda"),
    ("fat_dia", "menor_min", ["compras_pend", "fat_hora", "perda_loja"], "fat_hora", [],
     "dia fraco: a única das três que abre a venda é por hora"),
    ("fat_dia", "caiu", ["fat_loja", "clientes_cidade", "fat_dep"], "fat_loja", ["fat_dep"],
     "queda contra a semana anterior: por loja ou por departamento localizam a queda"),
    ("fat_loja", "maior_soma", ["fat_dep", "rup_loja", "canc_op"], "fat_dep", [],
     "faturamento alto: por departamento mostra o que vendeu; ruptura e cancelamento não explicam alta"),
    ("fat_loja", "menor_min", ["clientes_cidade", "perda_prod", "rup_loja"], "rup_loja", [],
     "loja abaixo do mínimo: falta de produto na loja explica venda baixa"),
    ("fat_loja", "caiu", ["fat_pag", "canc_loja", "fat_dep"], "fat_dep", ["canc_loja"],
     "queda por loja: departamento mostra o que caiu; cancelamento por loja também pode explicar"),
    ("canc_dia", "maior_soma", ["canc_op", "top_prod", "fat_pag"], "canc_op", [],
     "muitos cancelamentos: por operador é a única que abre os cancelamentos"),
    ("canc_dia", "maximo", ["compras_pend", "canc_loja", "fat_dep"], "canc_loja", [],
     "pico de cancelamento: por loja localiza onde aconteceu"),
    ("canc_dia", "subiu", ["canc_loja", "canc_op", "rup_dep"], "canc_loja", ["canc_op"],
     "cancelamentos mudaram contra a semana anterior: por loja ou por operador abrem a mudança"),
    ("ticket_loja", "menor_min", ["clientes_cidade", "desc_loja", "compras_pend"], "desc_loja", [],
     "ticket baixo: desconto por loja é a única ligada ao valor do cupom"),
    ("ticket_loja", "caiu", ["perda_loja", "top_prod", "desc_loja"], "desc_loja", ["top_prod"],
     "ticket caiu: desconto ou mix de produtos explicam o valor do cupom"),
    ("fat_pag", "maior_soma", ["perda_prod", "fat_loja", "rupA_prod"], "fat_loja", [],
     "pagamentos acima do limite: por loja mostra de onde veio a venda"),
    ("fat_pag", "maximo", ["fat_hora", "rup_forn", "clientes_cidade"], "fat_hora", [],
     "forma de pagamento com valor alto: por hora é a única que abre a venda"),
    ("fat_dia", "maximo", ["perda_prod", "compras_pend", "top_prod"], "top_prod", [],
     "dia de pico: os produtos mais vendidos mostram o que puxou a venda"),
    ("fat_loja", "maximo", ["rup_forn", "fat_dep", "perda_loja"], "fat_dep", [],
     "loja com faturamento alto: por departamento mostra o que vendeu"),
    # ---- ruptura ----
    ("rup_loja", "maior_soma", ["fat_loja", "rup_dep", "clientes_cidade"], "rup_dep", [],
     "ruptura acima do limite: por departamento mostra o que está faltando"),
    ("rup_loja", "maximo", ["fat_pag", "fat_hora", "rup_forn"], "rup_forn", [],
     "loja com muita ruptura: por fornecedor é a única de estoque e aponta quem não entregou"),
    ("rup_loja", "primeira", ["canc_op", "compras_pend", "desc_loja"], "compras_pend", [],
     "ruptura alta: compras pendentes é a única ligada à reposição"),
    ("min_loja", "maior_soma", ["fat_hora", "compras_pend", "rup_dep"], "compras_pend", ["rup_dep"],
     "itens abaixo do mínimo: compras pendentes (reposição) ou ruptura por departamento"),
    ("min_loja", "maximo", ["top_prod", "fat_pag", "rup_forn"], "rup_forn", [],
     "estoque baixo: por fornecedor é a única de estoque"),
    ("rupA_loja", "maior_soma", ["canc_loja", "rupA_prod", "fat_loja"], "rupA_prod", [],
     "curva A em ruptura: a lista dos produtos A em falta abre o alerta"),
    ("rupA_loja", "maximo", ["fat_hora", "clientes_cidade", "compras_pend"], "compras_pend", [],
     "curva A em ruptura: compras pendentes é a única ligada à reposição"),
    ("rup_dep", "maior_soma", ["fat_pag", "rup_forn", "rup_loja"], "rup_forn", ["rup_loja"],
     "ruptura por departamento: por fornecedor ou por loja abrem onde falta"),
    ("rup_dep", "maximo", ["desc_loja", "clientes_cidade", "rupA_prod"], "rupA_prod", [],
     "departamento com muita ruptura: a lista dos produtos A em falta é a única de estoque"),
    ("rup_dep", "primeira", ["canc_op", "fat_hora", "compras_pend"], "compras_pend", [],
     "ruptura alta: compras pendentes é a única ligada à reposição"),
    ("perda_motivo", "maior_soma", ["perda_loja", "fat_pag", "rup_loja"], "perda_loja", [],
     "perdas acima do limite: por loja localiza onde"),
    ("perda_motivo", "maximo", ["clientes_cidade", "perda_prod", "fat_hora"], "perda_prod", [],
     "motivo de perda alto: produtos com mais perda é a única de perda"),
    ("perda_motivo", "caiu", ["top_prod", "perda_loja", "perda_prod"], "perda_loja", ["perda_prod"],
     "perdas mudaram contra o mês anterior: por loja ou por produto abrem a mudança"),
    ("rup_loja", "maior_soma", ["rup_dep", "rup_forn", "compras_pend"], "rup_dep", ["rup_forn"],
     "três de estoque: por departamento ou por fornecedor abrem a ruptura; compras pendentes não diz o que falta"),
    ("min_loja", "primeira", ["rupA_prod", "perda_loja", "fat_dep"], "rupA_prod", [],
     "estoque abaixo do mínimo: a lista dos produtos A em falta é a única de estoque"),
]


def numero(v):
    return float(v)


def regra(tipo, linhas, anteriores, y):
    vs = [numero(l[y]) for l in linhas]
    soma, ant = sum(vs), sum(numero(l[y]) for l in anteriores) if anteriores else None
    if tipo == "maior_soma":
        return {"coluna": y, "agregacao": "soma", "operador": "maior", "valor": round(soma * 0.9)}
    if tipo == "menor_min":
        return {"coluna": y, "agregacao": "minimo", "operador": "menor", "valor": round(min(vs) * 1.1) + 1}
    if tipo == "maximo":
        return {"coluna": y, "agregacao": "maximo", "operador": "maior", "valor": round(max(vs) * 0.8)}
    if tipo == "primeira":
        return {"coluna": y, "agregacao": "primeira", "operador": "maior", "valor": round(vs[0] * 0.8)}
    if tipo in ("caiu", "subiu"):
        if ant is None:
            raise SystemExit(f"regra {tipo} sem semana anterior")
        media = y == "ticket_medio"  # somar ticket médio não tem sentido: compara a média das lojas
        atual, antes = (soma / len(vs), ant / len(anteriores)) if media else (soma, ant)
        op = "subiu" if atual >= antes else "caiu"  # o que de fato aconteceu, para disparar
        return {"coluna": y, "agregacao": "media" if media else "soma", "operador": op, "valor": 0, "base": "anterior"}
    raise SystemExit(f"regra desconhecida: {tipo}")


def casos():
    res_op = {k: sql(q) for k, (_, q) in OPCOES.items()}
    out = []
    for n, (pk, tipo, ops, esp, outras, porque) in enumerate(CASOS, 1):
        nome, x, y, q = PRINCIPAIS[pk]
        linhas = sql(q())
        anteriores = sql(q(30 if pk == "perda_motivo" else 7)) if tipo in ("caiu", "subiu") else []
        cond = regra(tipo, linhas, anteriores, y)
        assert esp in ops and all(o in ops for o in outras), n
        out.append({
            "id": f"A{n:02d}",
            "piloto": "vendas" if n <= 15 else "ruptura",
            "rotulo": f"{nome} · {tipo}",
            "consulta": {"id": "principal", "nome": nome, "x": x, "y": y,
                         "agenda": {"condicao": cond, "aprofundamentos": [f"op_{o}" for o in ops]}},
            "colunas": list(linhas[0].keys()),
            "linhas": linhas,
            "anteriores": [anteriores] if anteriores else [],
            "opcoes": [{"id": f"op_{o}", "nome": OPCOES[o][0],
                        "colunas": list(res_op[o][0].keys()) if res_op[o] else [],
                        "linhas": res_op[o]} for o in ops],
            "esperada": OPCOES[esp][0],
            "aceitaveis": [OPCOES[esp][0]] + [OPCOES[o][0] for o in outras],
            "porque": porque,
        })
    return out


if __name__ == "__main__":
    json.dump(casos(), sys.stdout, ensure_ascii=False, indent=1)
    sys.stdout.write("\n")
