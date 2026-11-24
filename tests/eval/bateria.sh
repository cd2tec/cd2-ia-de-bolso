#!/usr/bin/env sh
# Sobe um llamafile só para a bateria da História 6 (só CPU), roda tests/eval/bateria.js e derruba o motor.
# Feche a IA de Bolso antes: o modelo da tela e os agendamentos disputam a CPU e estragam a latência.
#   sh tests/eval/bateria.sh                                   modelo de models/, todas as threads
#   CD2_EVAL_MODEL=/caminho/Qwen3.5-4B-Q4_K_M.gguf sh tests/eval/bateria.sh
#   CD2_EVAL_SIMULACAO=1 sh tests/eval/bateria.sh              simula o hardware mínimo (Linux): motor preso a
#       4 núcleos (CD2_EVAL_NUCLEOS, padrão 4-7) e a 8 GB de memória (systemd-run). Não substitui a máquina real.
#   CD2_EVAL_ROTULO=qwen3-dev-1  CD2_EVAL_SAIDA=arquivo.json  CD2_EVAL_PORT=8091
# Na máquina de hardware mínimo de verdade (qualquer sistema): abra a IA de Bolso sem agendamentos e rode
#   CD2_EVAL_MODEL_URL=http://127.0.0.1:8080/v1/chat/completions node tests/eval/bateria.js
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"; ROOT="$(cd "$HERE/../.." && pwd)"
MODEL="${CD2_EVAL_MODEL:-$(ls "$ROOT"/models/*.gguf 2>/dev/null | head -n 1)}"
[ -n "$MODEL" ] && [ -f "$MODEL" ] || { echo "Sem modelo (CD2_EVAL_MODEL ou models/*.gguf)"; exit 1; }
PORT="${CD2_EVAL_PORT:-8091}"
LOG="${CD2_EVAL_LOG:-${TMPDIR:-/tmp}/cd2-bateria-llamafile.log}"
ENGINE="$ROOT/bin/llamafile"
export CD2_EVAL_MODEL_URL="http://127.0.0.1:$PORT/v1/chat/completions"

PRE=""; THREADS=""
if [ "${CD2_EVAL_SIMULACAO:-0}" = 1 ]; then
  NUC="${CD2_EVAL_NUCLEOS:-4-7}"
  PRE="systemd-run --user --scope --quiet -p MemoryMax=8G -p MemorySwapMax=0 taskset -c $NUC"
  THREADS="-t 4"
  export CD2_EVAL_THREADS=4 CD2_EVAL_SIMULACAO="8 GB, 4 núcleos ($NUC), sem GPU"
fi

LL=''
trap 'kill $LL 2>/dev/null' EXIT
trap 'exit 130' INT TERM
: > "$LOG"
# shellcheck disable=SC2086
$PRE "$ENGINE" -m "$MODEL" --server --host 127.0.0.1 --port "$PORT" --jinja -c 8192 --gpu disable -ngl 0 $THREADS >> "$LOG" 2>&1 &
LL=$!
n=0
until curl -sf "http://127.0.0.1:$PORT/health" >/dev/null 2>&1; do
  n=$((n + 1)); [ "$n" -ge 240 ] && { echo "O motor não subiu. Log: $LOG"; tail -20 "$LOG"; exit 1; }
  kill -0 "$LL" 2>/dev/null || { echo "O motor saiu. Log: $LOG"; tail -20 "$LOG"; exit 1; }
  sleep 1
done
node "$HERE/bateria.js"
