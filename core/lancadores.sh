#!/bin/sh
# CD2 Harness - regra dos lancadores na copia que vai para o pen drive (POSIX sh).
# So define a funcao; carregar este arquivo nao faz nada.
#
# Se o lancador do Mac ("IA de Bolso.app") esta na copia, o INICIAR-CD2-Mac.command
# sai da raiz e vai para core/: a raiz fica so com o lancador. Sem o lancador
# (plano B), o INICIAR continua na raiz. O iniciar-cd2-linux.sh nunca se move.
# O INICIAR-CD2-Windows.bat fica na raiz mesmo com o "IA de Bolso.exe", como
# alternativa: o par do Windows volta para a regra depois do teste num Windows de verdade.
arrumar_lancadores() {
  _al_stage="$1"
  if [ -d "$_al_stage/IA de Bolso.app" ] && [ -f "$_al_stage/INICIAR-CD2-Mac.command" ]; then
    mkdir -p "$_al_stage/core" && mv -f "$_al_stage/INICIAR-CD2-Mac.command" "$_al_stage/core/INICIAR-CD2-Mac.command" || return 1
  fi
  return 0
}
