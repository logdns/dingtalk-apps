@echo off
chcp 65001 >nul
echo Removing auto-start shortcut...
powershell -NoProfile -ExecutionPolicy Bypass -Command ^
  "$p=[Environment]::GetFolderPath('Startup') + '\DingTalkReqCollector.lnk';" ^
  "if (Test-Path $p) { Remove-Item $p -Force; 'Removed: ' + $p } else { 'Shortcut not found.' }"
echo Done.
pause
