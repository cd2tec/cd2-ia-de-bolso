#!/usr/bin/env sh
# tests/toolbox/check-recipe.sh ENGINE
# Valida core/toolbox/ENGINE.yaml contra o banco de tests/toolbox/ENGINE.env com
# `toolbox invoke`. Sai com 1 na primeira divergencia. Precisa de: toolbox, jq.
set -u
ENGINE="${1:?uso: check-recipe.sh postgres|oracle|mariadb|sqlserver|firebird}"
HERE="$(cd "$(dirname "$0")" && pwd)"; ROOT="$(cd "$HERE/../.." && pwd)"
YAML="$ROOT/core/toolbox/$ENGINE.yaml"
set -a; . "$HERE/$ENGINE.env"; set +a
# mesmo caminho do cd2-ctl: a receita usa password: "${CD2_DB_PASSWORD}", entao a senha vai
# escapada para YAML entre aspas duplas (\ vira \\, " vira \")
CD2_DB_PASSWORD="$(printf '%s' "$CD2_DB_PASSWORD" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g')"
# prefere o toolbox do pen drive (bin/toolbox/<plat>) ao que estiver no PATH
case "$(uname -s)-$(uname -m)" in
  Darwin-arm64) PLAT=darwin-arm64 ;; Darwin-*) PLAT=darwin-amd64 ;;
  Linux-aarch64|Linux-arm64) PLAT=linux-arm64 ;; *) PLAT=linux-amd64 ;;
esac
TOOLBOX="$ROOT/bin/toolbox/$PLAT/toolbox"; [ -x "$TOOLBOX" ] || TOOLBOX=toolbox
fail() { printf '[FALHOU] %s\n' "$*" >&2; exit 1; }
# invoke TOOL JSON -> imprime so o JSON do resultado (o log do toolbox vai antes, no stdout)
invoke() { "$TOOLBOX" invoke "$1" "$2" --config "$YAML" 2>&1 | sed -n '/^\[/,$p'; }
# invoke_err TOOL JSON -> imprime a ultima linha (mensagem de erro)
invoke_err() { "$TOOLBOX" invoke "$1" "$2" --config "$YAML" 2>&1 | tail -n 1; }
keys() { jq -r '.[0] | keys | join(",")'; }  # ordenado: a ordem das colunas varia por driver
TBL=produto; VW=v_venda; [ "$ENGINE" = oracle ] && TBL=PRODUTO && VW=V_VENDA

echo "== $ENGINE: toolbox $TOOLBOX"
echo "== $ENGINE: db_connection_status"
S="$(invoke db_connection_status '{}')"
[ "$(printf '%s' "$S" | keys)" = "database,db_version,engine,host,schema,user,visible_tables" ] || fail "status: colunas $(printf '%s' "$S" | keys)"
[ "$(printf '%s' "$S" | jq -r '.[0].engine')" = "$ENGINE" ] || fail "status: engine"
[ "$(printf '%s' "$S" | jq -r '.[0].host')" = "$CD2_DB_HOST" ] || fail "status: host nao veio do ambiente"
[ "$(printf '%s' "$S" | jq -r '.[0].visible_tables')" -ge 2 ] || fail "status: visible_tables < 2"

echo "== $ENGINE: db_list_tables"
L="$(invoke db_list_tables '{}')"
[ "$(printf '%s' "$L" | keys)" = "comments,name,num_rows,type" ] || fail "list: colunas"
printf '%s' "$L" | jq -e --arg t "$TBL" '.[] | select(.name == $t and .type == "TABLE")' >/dev/null || fail "list: $TBL como TABLE"
printf '%s' "$L" | jq -e --arg v "$VW" '.[] | select(.name == $v and .type == "VIEW")' >/dev/null || fail "list: $VW como VIEW"
L2="$(invoke db_list_tables '{"pattern":"VEND","include_views":false}')"
[ "$(printf '%s' "$L2" | jq 'length')" = 1 ] || fail "list: pattern VEND sem views deveria dar 1 (deu $(printf '%s' "$L2" | jq 'length'))"
L3="$(invoke db_list_tables '{"limit":1,"offset":1}')"
[ "$(printf '%s' "$L3" | jq 'length')" = 1 ] || fail "list: limit/offset"

echo "== $ENGINE: db_describe_table"
D="$(invoke db_describe_table "{\"table_name\":\"$(printf '%s' "$TBL" | tr 'A-Z' 'a-z')\"}")"
[ "$(printf '%s' "$D" | keys)" = "column_name,comments,data_type,foreign_key_references,is_primary_key,nullable,table" ] || fail "describe: colunas"
printf '%s' "$D" | jq -e '.[] | select((.column_name|ascii_downcase) == "id" and (.is_primary_key|tostring|test("1|true")))' >/dev/null || fail "describe: id como PK"
printf '%s' "$D" | jq -e '.[] | select((.column_name|ascii_downcase) == "nome" and .comments == "Descricao do produto")' >/dev/null || fail "describe: comentario de nome"
DV="$(invoke db_describe_table '{"table_name":"venda"}')"
printf '%s' "$DV" | jq -e '.[] | select((.column_name|ascii_downcase) == "produto_id" and (.foreign_key_references|ascii_downcase|test("produto\\(id\\)")))' >/dev/null || fail "describe: FK de venda.produto_id"
[ "$(invoke db_describe_table '{"table_name":"nao_existe"}' | jq 'length')" = 0 ] || fail "describe: tabela inexistente deveria dar []"

echo "== $ENGINE: db_search_columns"
C="$(invoke db_search_columns '{"pattern":"produto_id"}')"
[ "$(printf '%s' "$C" | keys)" = "column,comments,data_type,table" ] || fail "search: colunas"
[ "$(printf '%s' "$C" | jq 'length')" -ge 1 ] || fail "search: produto_id nao achado"

echo "== $ENGINE: db_sample_table"
A="$(invoke db_sample_table "{\"table_name\":\"$TBL\"}")"
[ "$(printf '%s' "$A" | jq 'length')" = 20 ] || fail "sample: teto de 20 (deu $(printf '%s' "$A" | jq 'length'))"
invoke_err db_sample_table '{"table_name":"produto; DROP TABLE produto"}' | grep -q "not an allowed value" || fail "sample: injecao passou"

echo "== $ENGINE: db_catalog"
K="$(invoke db_catalog '{}')"
[ "$(printf '%s' "$K" | keys)" = "column_name,data_type,num_rows,ordinal,table_name,table_type" ] || fail "catalog: colunas $(printf '%s' "$K" | keys)"
printf '%s' "$K" | jq -e --arg t "$TBL" '.[] | select(.table_name == $t and .table_type == "TABLE" and (.column_name|ascii_downcase) == "nome")' >/dev/null || fail "catalog: $TBL.nome"
printf '%s' "$K" | jq -e --arg v "$VW" '.[] | select(.table_name == $v and .table_type == "VIEW")' >/dev/null || fail "catalog: $VW como VIEW"
[ "$(printf '%s' "$K" | jq '[.[] | .ordinal | tonumber] | max')" -le 60 ] || fail "catalog: ordinal acima de 60"

echo "== $ENGINE: db_read_only_query"
Q="$(invoke db_read_only_query "{\"sql\":\"select count(*) as n from $TBL\"}")"
[ "$(printf '%s' "$Q" | jq -r '.[0].n // .[0].N')" = 25 ] || fail "query: count"
[ "$(invoke db_read_only_query "{\"sql\":\"select id from $TBL\"}" | jq 'length')" = 25 ] || fail "query: 25 linhas esperadas"
invoke_err db_read_only_query "{\"sql\":\"delete from $TBL\"}" | grep -q "not an allowed value" || fail "query: DELETE passou"
invoke_err db_read_only_query "{\"sql\":\"select 1) q; delete from $TBL; select (1\"}" | grep -q "excluded value" || fail "query: statement multiplo passou"
invoke_err db_read_only_query "{\"sql\":\"select count(*) from $TBL;\"}" | grep -q "excluded value" || fail "query: ; final deveria ser recusado no servidor (o front tira antes)"
[ "$(invoke db_read_only_query "{\"sql\":\"select count(*) as n from $TBL\"}" | jq -r '.[0].n // .[0].N')" = 25 ] || fail "query: linhas sumiram apos tentativa de DELETE"
if [ "$ENGINE" = sqlserver ]; then
  invoke_err db_read_only_query '{"sql":"select 1 a) q shutdown with nowait select * from (select 1 b"}' | grep -q "excluded value" || fail "query: lote T-SQL com SHUTDOWN passou"
  [ "$(invoke db_read_only_query '{"sql":"select id, use_count = 1 from produto order by id offset 10 rows fetch next 5 rows only"}' | jq 'length')" = 5 ] || fail "query: OFFSET/FETCH legitimo foi barrado"
fi
if [ "$ENGINE" = postgres ]; then
  [ "$(invoke db_read_only_query '{"sql":"select current_setting($$statement_timeout$$) as t"}' | jq -r '.[0].t')" = 15s ] || fail "query: statement_timeout da sessao nao e 15s"
  invoke_err db_read_only_query '{"sql":"select pg_terminate_backend(1)"}' | grep -q "excluded value" || fail "query: pg_terminate_backend passou"
  [ "$(invoke db_read_only_query '{"sql":"select current_setting($$default_transaction_read_only$$) as ro"}' | jq -r '.[0].ro')" = on ] || fail "query: sessao nao esta em default_transaction_read_only=on"
fi
echo "OK: $ENGINE"
