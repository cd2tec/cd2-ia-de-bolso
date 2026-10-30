#!/usr/bin/env python3
"""Benchmark do motor da IA de Bolso: responde às três perguntas do Daniel com números.

1. MoE ou densa? Fatiar entre CPU e GPU ganha tokens/s?
   -> lê a arquitetura do .gguf e mede a geração com 0 camadas na GPU, metade e todas.
2. Quantos parâmetros, quantos bits, quantos tokens/s?
   -> conta os parâmetros tensor a tensor, calcula bits por peso e mede leitura e geração.
3. Quanto contexto cabe com o KV cache? A velocidade cai com o contexto cheio?
   -> calcula o KV cache por token, lê o tamanho real no log do motor e mede a
      velocidade com o contexto em 5%..90%, e numa conversa que vai acumulando turnos.

Só usa a biblioteca padrão do Python 3.8+. Roda no Mac (M1/M2/M3), Windows 11 e Linux:

    python3 bench/bench_modelo.py              # Mac e Linux
    py bench\\bench_modelo.py                  # Windows (instale com: winget install Python.Python.3.12)

Opções úteis: --ctx 16384, --rapido, --sondar-ctx (tenta subir com 32k/64k/128k de contexto),
--modelo outro.gguf (ex.: um MoE para comparar), --extra "--n-cpu-moe 20".
O relatório sai em bench/resultados/ (.md e .json).
"""

import argparse
import json
import os
import platform
import re
import shutil
import socket
import statistics
import struct
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request
from datetime import datetime
from pathlib import Path

RAIZ = Path(__file__).resolve().parent.parent
WINDOWS = platform.system() == "Windows"

# ---------------------------------------------------------------- leitura do GGUF

# tamanho em bytes dos tipos escalares de metadado do GGUF
_ESCALARES = {0: "<B", 1: "<b", 2: "<H", 3: "<h", 4: "<I", 5: "<i", 6: "<f", 7: "<?", 10: "<Q", 11: "<q", 12: "<d"}
_TIPOS_QUANT = {0: "F32", 1: "F16", 2: "Q4_0", 3: "Q4_1", 6: "Q5_0", 7: "Q5_1", 8: "Q8_0", 10: "Q2_K", 11: "Q3_K",
                12: "Q4_K", 13: "Q5_K", 14: "Q6_K", 15: "Q8_K", 30: "BF16"}
_FILE_TYPE = {0: "F32", 1: "F16", 7: "Q8_0", 10: "Q2_K", 12: "Q3_K_M", 14: "Q4_K_S", 15: "Q4_K_M", 16: "Q5_K_S",
              17: "Q5_K_M", 18: "Q6_K", 32: "BF16"}


def _ler(f, fmt):
    return struct.unpack(fmt, f.read(struct.calcsize(fmt)))[0]


def _texto(f):
    return f.read(_ler(f, "<Q")).decode("utf-8", "replace")


def _valor(f, tipo):
    if tipo in _ESCALARES:
        return _ler(f, _ESCALARES[tipo])
    if tipo == 8:
        return _texto(f)
    if tipo == 9:
        sub, n = _ler(f, "<I"), _ler(f, "<Q")
        if sub in _ESCALARES:  # arrays grandes (vocabulário): pula sem guardar
            f.seek(struct.calcsize(_ESCALARES[sub]) * n, 1)
        else:
            for _ in range(n):
                _valor(f, sub)
        return "[array %d]" % n
    raise ValueError("tipo de metadado GGUF desconhecido: %d" % tipo)


def ler_gguf(caminho):
    """Devolve (metadados, tensores). Cada tensor: (nome, forma, tipo)."""
    with open(caminho, "rb") as f:
        if f.read(4) != b"GGUF":
            raise ValueError("%s não é um arquivo GGUF" % caminho)
        _ler(f, "<I")  # versão
        n_tensores, n_meta = _ler(f, "<Q"), _ler(f, "<Q")
        meta = {}
        for _ in range(n_meta):
            chave = _texto(f)
            meta[chave] = _valor(f, _ler(f, "<I"))
        tensores = []
        for _ in range(n_tensores):
            nome = _texto(f)
            forma = [_ler(f, "<Q") for _ in range(_ler(f, "<I"))]
            tipo = _ler(f, "<I")
            _ler(f, "<Q")  # offset
            tensores.append((nome, forma, tipo))
    return meta, tensores


def analisar_modelo(caminho):
    meta, tensores = ler_gguf(caminho)
    arq = meta.get("general.architecture", "?")
    m = lambda k, d=None: meta.get("%s.%s" % (arq, k), d)

    params = 0
    params_especialistas = 0
    por_tipo = {}
    for nome, forma, tipo in tensores:
        n = 1
        for d in forma:
            n *= d
        params += n
        if "_exps" in nome:  # pesos dos especialistas de um MoE (ffn_*_exps)
            params_especialistas += n
        t = _TIPOS_QUANT.get(tipo, str(tipo))
        por_tipo[t] = por_tipo.get(t, 0) + n

    especialistas = m("expert_count", 0) or 0
    usados = m("expert_used_count", 0) or 0
    ativos = params
    if especialistas and usados:
        ativos = params - params_especialistas + params_especialistas * usados / especialistas

    camadas = m("block_count")
    cabecas = m("attention.head_count")
    cabecas_kv = m("attention.head_count_kv", cabecas)
    emb = m("embedding_length")
    dim_k = m("attention.key_length", emb // cabecas if emb and cabecas else None)
    dim_v = m("attention.value_length", dim_k)
    kv_por_token = camadas * cabecas_kv * (dim_k + dim_v) * 2 if camadas and cabecas_kv and dim_k else None  # f16

    tamanho = os.path.getsize(caminho)
    return {
        "arquivo": Path(caminho).name,
        "tamanho_bytes": tamanho,
        "arquitetura": arq,
        "nome": meta.get("general.name"),
        "tipo_arquivo": _FILE_TYPE.get(meta.get("general.file_type"), meta.get("general.file_type")),
        "moe": bool(especialistas),
        "especialistas": especialistas,
        "especialistas_ativos": usados,
        "parametros": params,
        "parametros_ativos_por_token": int(ativos),
        "bits_por_peso": tamanho * 8 / params,
        "parametros_por_tipo": por_tipo,
        "camadas": camadas,
        "cabecas_atencao": cabecas,
        "cabecas_kv": cabecas_kv,
        "dim_cabeca": dim_k,
        "contexto_treino": m("context_length"),
        "kv_bytes_por_token_f16": kv_por_token,
    }


# ---------------------------------------------------------------- motor (llamafile)


def achar_binario(arg):
    if arg:
        return Path(arg)
    nome = "llamafile.exe" if WINDOWS else "llamafile"
    p = RAIZ / "bin" / nome
    if not p.exists():
        sys.exit("Não achei %s. Rode a preparação do pen drive ou passe --llamafile." % p)
    return p


def preparar_binario(origem):
    """Pen drive exFAT não guarda bit de execução: roda de uma cópia local, como o start-unix.sh."""
    if WINDOWS:
        return origem
    destino = Path(tempfile.gettempdir()) / "cd2-bench" / "llamafile"
    destino.parent.mkdir(parents=True, exist_ok=True)
    if not destino.exists() or destino.stat().st_size != origem.stat().st_size:
        print("Copiando o motor para %s (só na primeira vez)..." % destino)
        shutil.copyfile(origem, destino)
        destino.chmod(0o755)
        if platform.system() == "Darwin":
            subprocess.call(["xattr", "-dr", "com.apple.quarantine", str(destino)], stderr=subprocess.DEVNULL)
    return destino


def porta_livre():
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def http(url, corpo=None, prazo=600):
    dados = json.dumps(corpo).encode() if corpo is not None else None
    req = urllib.request.Request(url, data=dados, headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=prazo) as r:
        return json.loads(r.read() or b"{}")


class Motor:
    def __init__(self, binario, modelo, ctx, ngl, extra, pasta_log, rotulo):
        self.porta = porta_livre()
        self.url = "http://127.0.0.1:%d" % self.porta
        self.log = pasta_log / ("motor-%s.log" % rotulo)
        args = ["-m", str(modelo), "--server", "--host", "127.0.0.1", "--port", str(self.porta),
                "-c", str(ctx), "-np", "1", "--no-webui", "-lv", "4"]  # -lv 4: o log mostra camadas e KV
        args += ["--gpu", "disable", "-ngl", "0"] if ngl == 0 else ["-ngl", str(ngl)]
        args += extra
        self.cmd = [str(binario)] + args
        self.proc = None

    def subir(self, prazo=300):
        saida = open(self.log, "wb")
        try:
            self.proc = subprocess.Popen(self.cmd, stdout=saida, stderr=subprocess.STDOUT)
        except OSError:
            # Linux com binfmt de outro programa (Wine) pegando o APE: roda via sh
            self.proc = subprocess.Popen(["sh"] + self.cmd, stdout=saida, stderr=subprocess.STDOUT)
        fim = time.time() + prazo
        while time.time() < fim:
            if self.proc.poll() is not None:
                return False
            try:
                if http(self.url + "/health", prazo=2).get("status") == "ok":
                    return True
            except (urllib.error.URLError, OSError, ValueError):
                pass
            time.sleep(1)
        return False

    def descer(self):
        if self.proc and self.proc.poll() is None:
            self.proc.terminate()
            try:
                self.proc.wait(20)
            except subprocess.TimeoutExpired:
                self.proc.kill()

    def texto_log(self):
        try:
            return self.log.read_text("utf-8", "replace")
        except OSError:
            return ""

    def resumo_log(self):
        """Linhas do log que provam onde ficaram as camadas e quanto ocupou o KV cache."""
        log = self.texto_log()
        r = {}
        m = re.search(r"offloaded (\d+)/(\d+) layers to GPU", log)
        if m:
            r["camadas_gpu"] = "%s/%s" % m.groups()
        m = re.findall(r"llama_kv_cache.*?size\s*=\s*([\d.]+)\s*MiB", log)
        if m:
            r["kv_cache_mib"] = float(m[-1])
        m = re.search(r"llama_context: n_ctx\s*=\s*(\d+)", log)
        if m:
            r["ctx_real"] = int(m.group(1))
        r["buffers"] = sorted(set(re.findall(r"(\w+(?:_Mapped)?) model buffer size\s*=\s*([\d.]+) MiB", log)))
        r["dispositivos"] = sorted(set(re.findall(r"using device (\w+)", log)))[:4]
        return r

    def tokenizar(self, texto):
        return http(self.url + "/tokenize", {"content": texto})["tokens"]

    def completar(self, tokens, n, cache):
        r = http(self.url + "/completion", {
            "prompt": tokens, "n_predict": n, "temperature": 0, "ignore_eos": True, "cache_prompt": cache,
        })
        t = r["timings"]
        return {"prompt_n": t["prompt_n"], "prompt_tps": t["prompt_per_second"], "prompt_ms": t["prompt_ms"],
                "gen_n": t["predicted_n"], "gen_tps": t["predicted_per_second"], "texto": r.get("content", "")}


def texto_enchimento(n_linhas):
    # dados de varejo com números variados: tokeniza como uma consulta de verdade
    return "\n".join("Loja %d, %02d/10: vendeu %d unidades do produto %d (%s), margem de %d%%." % (
        i % 37 + 1, i % 28 + 1, i * 7 % 1000, i * 13 % 500, ["bebidas", "limpeza", "hortifruti", "padaria"][i % 4],
        i * 11 % 60) for i in range(n_linhas))


# ---------------------------------------------------------------- testes


def mediana(xs):
    return statistics.median(xs) if xs else 0.0


def teste_offload(binario, modelo, info, args, pasta):
    """Pergunta 1: mesma consulta com 0, metade e todas as camadas na GPU."""
    total = (info["camadas"] or 0) + 1  # +1: camada de saída
    niveis = args.ngl or [0, total // 2, 999]
    linhas = []
    for ngl in niveis:
        rotulo = "ngl%d" % ngl
        print("  -ngl %-4d subindo..." % ngl, end="", flush=True)
        mot = Motor(binario, modelo, 4096, ngl, args.extra, pasta, rotulo)
        try:
            if not mot.subir():
                print(" não subiu (veja %s)" % mot.log)
                linhas.append({"ngl": ngl, "subiu": False})
                continue
            toks = mot.tokenizar(texto_enchimento(200))[:512]
            mot.completar(toks[:32], 8, False)  # aquecimento
            rodadas = [mot.completar(toks, args.gerar, False) for _ in range(args.repeticoes)]
            res = {"ngl": ngl, "subiu": True, **mot.resumo_log(),
                   "leitura_tps": mediana([r["prompt_tps"] for r in rodadas]),
                   "geracao_tps": mediana([r["gen_tps"] for r in rodadas])}
            linhas.append(res)
            print(" leitura %7.1f tok/s | geração %6.1f tok/s | camadas na GPU: %s" % (
                res["leitura_tps"], res["geracao_tps"], res.get("camadas_gpu", "0 (CPU)")))
        finally:
            mot.descer()
    return linhas


def melhor_ngl(offload):
    ok = [l for l in offload if l.get("subiu")]
    return max(ok, key=lambda l: l["geracao_tps"])["ngl"] if ok else 0


def teste_contexto(binario, modelo, args, pasta, ngl):
    """Pergunta 3: velocidade com o contexto cada vez mais cheio, e numa conversa que acumula."""
    mot = Motor(binario, modelo, args.ctx, ngl, args.extra, pasta, "ctx%d" % args.ctx)
    frio, conversa = [], []
    try:
        print("  subindo com -c %d -ngl %d..." % (args.ctx, ngl))
        if not mot.subir():
            print("  não subiu (veja %s)" % mot.log)
            return {"subiu": False}
        log = mot.resumo_log()
        print("  KV cache alocado pelo motor: %s MiB" % log.get("kv_cache_mib", "?"))
        base = mot.tokenizar(texto_enchimento(args.ctx // 12 + 200))
        mot.completar(base[:32], 8, False)

        print("  a) consulta isolada (sem reaproveitar cache), contexto preenchido em:")
        fracoes = [0.05, 0.25, 0.5, 0.75, 0.9] if not args.rapido else [0.05, 0.5, 0.9]
        for fr in fracoes:
            n = min(int(args.ctx * fr), args.ctx - args.gerar - 8, len(base))
            r = mot.completar(base[:n], args.gerar, False)
            r.pop("texto")
            r["ocupacao"] = fr
            frio.append(r)
            print("     %3d%% (%5d tok): leitura %7.1f tok/s em %5.1f s | geração %6.1f tok/s" % (
                fr * 100, n, r["prompt_tps"], r["prompt_ms"] / 1000, r["gen_tps"]))

        print("  b) conversa: cada turno manda ~%d tokens novos e reaproveita o KV cache dos anteriores" % args.turno)
        # outro começo: senão o turno 1 reaproveita o KV que sobrou das consultas isoladas
        historico, pos, turno = mot.tokenizar("Nova conversa sobre as vendas da rede.\n"), 0, 0
        while True:
            novos = base[pos:pos + args.turno]
            pos += args.turno
            if len(historico) + len(novos) + args.gerar + 8 > args.ctx or not novos:
                break
            historico += novos
            turno += 1
            r = mot.completar(historico, args.gerar, True)
            historico += mot.tokenizar(r.pop("texto"))
            r.update(turno=turno, contexto=len(historico))
            conversa.append(r)
            print("     turno %2d: contexto %5d tok | processou só %5d tok novos em %5.1f s | geração %6.1f tok/s" % (
                turno, r["contexto"], r["prompt_n"], r["prompt_ms"] / 1000, r["gen_tps"]))
        return {"subiu": True, "ngl": ngl, **log, "isolada": frio, "conversa": conversa}
    finally:
        mot.descer()


def sondar_contexto(binario, modelo, info, args, pasta, ngl):
    """Pergunta 3, limite: tenta subir o motor com contextos maiores e lê o KV alocado."""
    res = []
    for ctx in [8192, 32768, 65536, 131072, info.get("contexto_treino") or 262144]:
        mot = Motor(binario, modelo, ctx, ngl, args.extra, pasta, "sonda%d" % ctx)
        try:
            print("  -c %-6d" % ctx, end="", flush=True)
            ok = mot.subir(prazo=180)
            r = {"ctx": ctx, "subiu": ok, **mot.resumo_log()}
            if ok:
                # prova que cabe de verdade: lê um prompt de 90% do contexto
                n = int(ctx * 0.9)
                toks = (mot.tokenizar(texto_enchimento(4000)) * (n // 4000 + 1))[:n]
                try:
                    c = mot.completar(toks, 16, False)
                    r.update(leitura_tps=c["prompt_tps"], geracao_tps=c["gen_tps"], segundos=c["prompt_ms"] / 1000)
                except (urllib.error.URLError, OSError, KeyError) as e:
                    r["erro"] = str(e)
            res.append(r)
            print(" %s | KV %s MiB%s" % ("subiu" if ok else "NÃO subiu", r.get("kv_cache_mib", "?"),
                  " | 90%% cheio: leitura %.0f tok/s em %.0f s, geração %.1f tok/s" % (
                      r["leitura_tps"], r["segundos"], r["geracao_tps"]) if "leitura_tps" in r else ""))
            if not ok:
                break
        finally:
            mot.descer()
    return res


# ---------------------------------------------------------------- máquina e relatório


def maquina():
    m = {"so": "%s %s" % (platform.system(), platform.release()), "arquitetura": platform.machine(),
         "cpu": platform.processor(), "nucleos": os.cpu_count()}
    try:
        if platform.system() == "Darwin":
            m["cpu"] = subprocess.check_output(["sysctl", "-n", "machdep.cpu.brand_string"], text=True).strip()
            m["ram_gb"] = int(subprocess.check_output(["sysctl", "-n", "hw.memsize"], text=True)) / 2**30
            m["gpu"] = m["cpu"] + " (memória unificada)"
        elif WINDOWS:
            ps = lambda c: subprocess.check_output(["powershell", "-NoProfile", "-Command", c], text=True).strip()
            m["cpu"] = ps("(Get-CimInstance Win32_Processor).Name")
            m["ram_gb"] = int(ps("(Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory")) / 2**30
            m["gpu"] = ps("(Get-CimInstance Win32_VideoController | Select-Object -Expand Name) -join ' / '")
        else:
            with open("/proc/cpuinfo") as f:
                n = re.search(r"model name\s*:\s*(.*)", f.read())
                m["cpu"] = n.group(1) if n else m["cpu"]
            with open("/proc/meminfo") as f:
                m["ram_gb"] = int(re.search(r"MemTotal:\s*(\d+)", f.read()).group(1)) / 2**20
            try:
                m["gpu"] = subprocess.check_output(["sh", "-c", "lspci | grep -iE 'vga|3d|display'"], text=True).strip()
            except (OSError, subprocess.CalledProcessError):
                pass
        if shutil.which("nvidia-smi"):
            m["vram"] = subprocess.check_output(
                ["nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader"], text=True).strip()
    except (OSError, subprocess.CalledProcessError, ValueError, AttributeError):
        pass
    return m


def gb(b):
    return b / 2**30


def relatorio(r):
    i, maq = r["modelo"], r["maquina"]
    L = ["# Benchmark do motor: %s" % i["arquivo"], "",
         "Máquina: %s, %s, %.0f GB de RAM, GPU: %s%s. Rodado em %s." % (
             maq.get("cpu"), maq["so"], maq.get("ram_gb", 0), maq.get("gpu", "?"),
             " (%s)" % maq["vram"] if maq.get("vram") else "", r["quando"]), ""]

    L += ["## 1. MoE ou densa? Fatiar entre CPU e GPU ajuda?", ""]
    if i["moe"]:
        L.append("**MoE**: %d especialistas, %d ativos por token. %.2f bi de parâmetros no total, ~%.2f bi ativos por token." % (
            i["especialistas"], i["especialistas_ativos"], i["parametros"] / 1e9, i["parametros_ativos_por_token"] / 1e9))
    else:
        L.append("**Densa** (arquitetura `%s`, sem especialistas): todos os %.2f bi de parâmetros trabalham em cada token." % (
            i["arquitetura"], i["parametros"] / 1e9))
    if r.get("offload"):
        L += ["", "| camadas na GPU (-ngl) | camadas no log | leitura (tok/s) | geração (tok/s) |", "|---|---|---|---|"]
        for l in r["offload"]:
            if l.get("subiu"):
                L.append("| %d | %s | %.1f | %.1f |" % (l["ngl"], l.get("camadas_gpu", "0 (CPU)"), l["leitura_tps"], l["geracao_tps"]))
            else:
                L.append("| %d | não subiu | – | – |" % l["ngl"])
    L.append("")

    L += ["## 2. Tamanho e velocidade", "",
          "| | |", "|---|---|",
          "| Parâmetros | %.3f bilhões (%d) |" % (i["parametros"] / 1e9, i["parametros"]),
          "| Quantização | %s, média de %.2f bits por peso |" % (i["tipo_arquivo"], i["bits_por_peso"]),
          "| Arquivo | %.2f GB |" % gb(i["tamanho_bytes"]),
          "| Pesos por tipo | %s |" % ", ".join("%s: %.0f%%" % (t, 100 * n / i["parametros"])
                                                for t, n in sorted(i["parametros_por_tipo"].items(), key=lambda x: -x[1])),
          ""]
    if r.get("offload"):
        ok = [l for l in r["offload"] if l.get("subiu")]
        if ok:
            b = max(ok, key=lambda l: l["geracao_tps"])
            L += ["Melhor configuração (-ngl %d): **%.1f tok/s gerando** e %.1f tok/s lendo o prompt "
                  "(prompt de 512 tokens, %d tokens gerados, mediana de %d rodadas)." % (
                      b["ngl"], b["geracao_tps"], b["leitura_tps"], r["args"]["gerar"], r["args"]["repeticoes"]), ""]

    L += ["## 3. Contexto, KV cache e decaimento", ""]
    kv = i["kv_bytes_por_token_f16"]
    if kv:
        L += ["KV cache (f16) = %d camadas × %d cabeças KV × %d dims × 2 (K e V) × 2 bytes = **%d KiB por token**. "
              "Contexto de treino: %s tokens." % (i["camadas"], i["cabecas_kv"], i["dim_cabeca"], kv // 1024,
                                                  "{:,}".format(i["contexto_treino"]).replace(",", ".")), "",
              "| contexto | KV cache | KV + pesos |", "|---|---|---|"]
        for c in [4096, 8192, 16384, 32768, 65536, 131072, i["contexto_treino"] or 262144]:
            L.append("| %d | %.2f GB | %.2f GB |" % (c, gb(kv * c), gb(kv * c + i["tamanho_bytes"])))
        L.append("")
    c = r.get("contexto") or {}
    if c.get("subiu"):
        L += ["Motor com `-c %d`: KV alocado de verdade = %s MiB." % (r["args"]["ctx"], c.get("kv_cache_mib", "?")), "",
              "**a) Consulta isolada**, sem cache:", "",
              "| contexto ocupado | tokens | leitura (tok/s) | tempo de leitura | geração (tok/s) |", "|---|---|---|---|---|"]
        for x in c["isolada"]:
            L.append("| %d%% | %d | %.1f | %.1f s | %.1f |" % (x["ocupacao"] * 100, x["prompt_n"], x["prompt_tps"],
                                                              x["prompt_ms"] / 1000, x["gen_tps"]))
        L += ["", "**b) Conversa** (consultas seguintes reaproveitam o KV cache):", "",
              "| turno | contexto total | tokens processados | tempo de leitura | geração (tok/s) |", "|---|---|---|---|---|"]
        for x in c["conversa"]:
            L.append("| %d | %d | %d | %.1f s | %.1f |" % (x["turno"], x["contexto"], x["prompt_n"], x["prompt_ms"] / 1000, x["gen_tps"]))
        if c["isolada"]:
            p, u = c["isolada"][0], c["isolada"][-1]
            L += ["", "Decaimento da geração de %d%% para %d%% de contexto: %.1f → %.1f tok/s (%+.0f%%)." % (
                p["ocupacao"] * 100, u["ocupacao"] * 100, p["gen_tps"], u["gen_tps"], 100 * (u["gen_tps"] / p["gen_tps"] - 1))]
        L.append("")
    if r.get("sonda"):
        L += ["**Limite de contexto nesta máquina** (motor sobe e lê um prompt de 90% do contexto):", "",
              "| -c | subiu? | KV alocado | leitura 90% | geração |", "|---|---|---|---|---|"]
        for x in r["sonda"]:
            L.append("| %d | %s | %s MiB | %s | %s |" % (
                x["ctx"], "sim" if x["subiu"] else "não", x.get("kv_cache_mib", "?"),
                "%.0f s" % x["segundos"] if "segundos" in x else x.get("erro", "–"),
                "%.1f tok/s" % x["geracao_tps"] if "geracao_tps" in x else "–"))
        L.append("")
    return "\n".join(L)


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--modelo", help="arquivo .gguf (padrão: o primeiro em models/)")
    ap.add_argument("--llamafile", help="binário do motor (padrão: bin/llamafile ou bin/llamafile.exe)")
    ap.add_argument("--ctx", type=int, default=8192, help="contexto do teste de decaimento (padrão 8192, o do produto)")
    ap.add_argument("--gerar", type=int, default=128, help="tokens gerados por consulta")
    ap.add_argument("--repeticoes", type=int, default=3)
    ap.add_argument("--turno", type=int, default=1000, help="tokens novos por turno da conversa")
    ap.add_argument("--ngl", type=int, nargs="*", help="níveis de camadas na GPU a comparar (padrão: 0, metade, todas)")
    ap.add_argument("--extra", default="", help='argumentos a mais para o llamafile, ex.: "--n-cpu-moe 20"')
    ap.add_argument("--rapido", action="store_true", help="menos medições")
    ap.add_argument("--sondar-ctx", action="store_true", help="tenta subir com 32k, 64k, 128k e o máximo de treino")
    ap.add_argument("--so-metadados", action="store_true", help="só lê o .gguf, não sobe o motor")
    args = ap.parse_args()
    sys.stdout.reconfigure(line_buffering=True, errors="replace")  # console do Windows em cp1252
    args.extra = args.extra.split()
    if args.rapido:
        args.repeticoes = 1

    modelo = Path(args.modelo) if args.modelo else next(iter(sorted((RAIZ / "models").glob("*.gguf"))), None)
    if not modelo or not modelo.exists():
        sys.exit("Não achei o modelo .gguf em models/. Passe --modelo.")

    agora = datetime.now()
    pasta = RAIZ / "bench" / "resultados"
    pasta.mkdir(parents=True, exist_ok=True)
    pasta_log = Path(tempfile.mkdtemp(prefix="cd2-bench-"))

    print("== Máquina")
    maq = maquina()
    for k, v in maq.items():
        print("  %-12s %s" % (k, "%.1f" % v if isinstance(v, float) else v))

    print("\n== Modelo: %s" % modelo.name)
    info = analisar_modelo(modelo)
    print("  arquitetura %s -> %s" % (info["arquitetura"], "MoE (%d especialistas, %d ativos)" % (
        info["especialistas"], info["especialistas_ativos"]) if info["moe"] else "DENSA (sem especialistas)"))
    print("  parâmetros  %.3f bi | %s | %.2f bits/peso | arquivo %.2f GB" % (
        info["parametros"] / 1e9, info["tipo_arquivo"], info["bits_por_peso"], gb(info["tamanho_bytes"])))
    if info["kv_bytes_por_token_f16"]:
        print("  KV cache    %d KiB/token -> %.2f GB em 8k, %.2f GB em 32k, %.2f GB no máximo de treino (%d)" % (
            info["kv_bytes_por_token_f16"] // 1024, gb(info["kv_bytes_por_token_f16"] * 8192),
            gb(info["kv_bytes_por_token_f16"] * 32768),
            gb(info["kv_bytes_por_token_f16"] * (info["contexto_treino"] or 0)), info["contexto_treino"] or 0))

    r = {"quando": agora.isoformat(timespec="seconds"), "maquina": maq, "modelo": info,
         "args": {k: v for k, v in vars(args).items()}}
    if not args.so_metadados:
        binario = preparar_binario(achar_binario(args.llamafile))
        print("\n== 1/2. Velocidade conforme as camadas na GPU (contexto 4096, prompt 512, gera %d)" % args.gerar)
        r["offload"] = teste_offload(binario, modelo, info, args, pasta_log)
        ngl = melhor_ngl(r["offload"])
        print("\n== 3. Contexto e decaimento (-c %d)" % args.ctx)
        r["contexto"] = teste_contexto(binario, modelo, args, pasta_log, ngl)
        if args.sondar_ctx:
            print("\n== 3. Sondando o limite de contexto (pode demorar e encher a memória)")
            r["sonda"] = sondar_contexto(binario, modelo, info, args, pasta_log, ngl)
        print("\nLogs do motor em %s" % pasta_log)

    base = "bench-%s-%s" % (re.sub(r"\W+", "-", socket.gethostname()).strip("-").lower(), agora.strftime("%Y%m%d-%H%M"))
    (pasta / (base + ".json")).write_text(json.dumps(r, indent=2, ensure_ascii=False), "utf-8")
    (pasta / (base + ".md")).write_text(relatorio(r), "utf-8")
    print("Relatório: %s" % (pasta / (base + ".md")))


if __name__ == "__main__":
    main()
