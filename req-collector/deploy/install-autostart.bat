@echo off
chcp 65001 >nul
setlocal
cd /d "%~dp0"
cd ..
set "APPDIR=%CD%"
set "STARTFILE=%APPDIR%\start.bat"

if not exist "%STARTFILE%" (
  echo [ERROR] start.bat not found in %APPDIR%
  echo Please run this script from the deploy folder.
  pause
  exit /b 1
)

echo Installing auto-start shortcut for current user...
echo Target : %STARTFILE%
echo Folder : Startup

powershell -NoProfile -ExecutionPolicy Bypass -Command ^
  "$ws=(New-Object -ComObject WScript.Shell);" ^
  "$p=[Environment]::GetFolderPath('Startup');" ^
  "$s=$ws.CreateShortcut($p + '\DingTalkReqCollector.lnk');" ^
  "$s.TargetPath='%STARTFILE%';" ^
  "$s.WorkingDirectory='%APPDIR%';" ^
  "$s.Description='DingTalk Requirement Collector';" ^
  "$s.WindowStyle=7;" ^
  "$s.Save()"

if errorlevel 1 (
  echo [FAILED] Could not create shortcut.
) else (
  echo [OK] Auto-start installed. It will run on next login.
)
echo.
echo Tip: for server boot without login, use Task Scheduler or NSSM (see README).
pause
