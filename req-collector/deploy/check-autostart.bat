@echo off
chcp 65001 >nul
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0check-autostart.ps1" -AppDir "%~dp0.."
echo.
pause
