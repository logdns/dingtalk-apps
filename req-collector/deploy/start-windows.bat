@echo off
REM DingTalk Requirement Collector - Windows start script
REM May live next to the exe (release root start.bat) or inside deploy\
REM The window stays open on error so you can read the message
title DingTalk Requirement Collector
cd /d "%~dp0"
if not exist "dingtalk-req-collector.exe" if exist "..\dingtalk-req-collector.exe" cd ..

if not exist "dingtalk-req-collector.exe" (
  echo [ERROR] dingtalk-req-collector.exe not found
  echo         Current dir: %CD%
  echo         Put this script next to dingtalk-req-collector.exe and config.json.
  echo.
  pause
  exit /b 1
)

echo Starting DingTalk Requirement Collector (dir: %CD%) ...
echo.
dingtalk-req-collector.exe
echo.
echo Server exited. See the error above (also written to logs\server.log).
pause
