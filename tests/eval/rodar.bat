@echo off
rem Versao Windows do tests/eval/rodar.sh; as variaveis CD2_EVAL_* estao descritas em rodar.ps1.
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0rodar.ps1"
exit /b %errorlevel%
