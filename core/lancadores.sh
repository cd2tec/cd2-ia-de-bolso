#!/bin/sh
# CD2 Harness - regra dos lancadores na copia que vai para o pen drive (POSIX sh).
# So define a funcao; carregar este arquivo nao faz nada.
#
# Com o lancador de um sistema na copia ("IA de Bolso.app" no Mac, "IA de Bolso.exe" no
# Windows), o INICIAR dele sai da raiz e vai para core/, como plano B pelo terminal: a raiz
# fica so com o lancador. Sem o lancador, o INICIAR continua na raiz. O ia-de-bolso.sh
# (Linux, que nao tem lancador) nunca se move.
arrumar_lancadores() {
  _al_stage="$1"
  if [ -d "$_al_stage/IA de Bolso.app" ] && [ -f "$_al_stage/INICIAR-CD2-Mac.command" ]; then
    mkdir -p "$_al_stage/core" && mv -f "$_al_stage/INICIAR-CD2-Mac.command" "$_al_stage/core/INICIAR-CD2-Mac.command" || return 1
  fi
  if [ -f "$_al_stage/IA de Bolso.exe" ] && [ -f "$_al_stage/INICIAR-CD2-Windows.bat" ]; then
    mkdir -p "$_al_stage/core" && mv -f "$_al_stage/INICIAR-CD2-Windows.bat" "$_al_stage/core/INICIAR-CD2-Windows.bat" || return 1
  fi
  return 0
}
