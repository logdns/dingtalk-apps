@echo off
REM Kingdee Requirement Server - start script
REM Put this file in the same folder as dingtalk-req-collector.exe and config.json
title Kingdee Requirement Server
cd /d %~dp0
dingtalk-req-collector.exe
echo.
echo Server exited. Read the error message above.
pause
