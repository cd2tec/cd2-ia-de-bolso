#!/bin/sh
# CD2 Harness - regra dos lancadores na copia que vai para o pen drive (POSIX sh).
# So define a funcao; carregar este arquivo nao faz nada.
#
# Para cada sistema, se o lancador esta na copia ("IA de Bolso.exe" no Windows,
# "IA de Bolso.app" no Mac), o INICIAR dele sai da raiz e vai para core/: a raiz
# fica so com o lancador. Sem o lancador (plano B), o INICIAR continua na raiz.
# O iniciar-cd2-linux.sh nunca se move.
arrumar_lancadores() {
  _al_stage="$1"
  if [ -f "$_al_stage/IA de Bolso.exe" ] && [ -f "$_al_stage/INICIAR-CD2-Windows.bat" ]; then
    mkdir -p "$_al_stage/core" && mv -f "$_al_stage/INICIAR-CD2-Windows.bat" "$_al_stage/core/INICIAR-CD2-Windows.bat" || return 1
  fi
  if [ -d "$_al_stage/IA de Bolso.app" ] && [ -f "$_al_stage/INICIAR-CD2-Mac.command" ]; then
    mkdir -p "$_al_stage/core" && mv -f "$_al_stage/INICIAR-CD2-Mac.command" "$_al_stage/core/INICIAR-CD2-Mac.command" || return 1
  fi
  return 0
}
