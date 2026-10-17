@echo off
chcp 65001 >nul
title CD2 IA de Bolso
rem Na raiz do repositorio o script fica em core\; no pen drive com lancador este .bat mora em core\, ao lado dele.
set "CD2_PS1=%~dp0core\start-windows.ps1"
if not exist "%CD2_PS1%" set "CD2_PS1=%~dp0start-windows.ps1"
powershell -NoProfile -ExecutionPolicy Bypass -File "%CD2_PS1%"
if errorlevel 1 pause
