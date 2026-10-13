param(
  [Parameter(Mandatory = $true)][string]$AppDir
)
$ErrorActionPreference = 'Stop'

try { [Console]::OutputEncoding = [Text.Encoding]::UTF8 } catch {}

function Say([string]$msg, [string]$color = 'Gray') {
  Write-Host $msg -ForegroundColor $color
}

Say "============================================"
Say "  DingTalk Requirement Collector - install auto-start" 'Cyan'
Say "============================================"
Say ""

$AppDir = (Resolve-Path -LiteralPath $AppDir).Path
$startFile = Join-Path (Join-Path $AppDir 'deploy') 'start-windows.bat'
if (-not (Test-Path -LiteralPath $startFile)) {
  $startFile = Join-Path $AppDir 'start.bat'
}
if (-not (Test-Path -LiteralPath $startFile)) {
  Say "[ERROR] start script not found:" 'Red'
  Say "        $AppDir\deploy\start-windows.bat"
  Say "        or $AppDir\start.bat"
  Say ""
  Say "Run this script inside the release folder (next to dingtalk-req-collector.exe)." 'Yellow'
  exit 1
}

$exe = Join-Path $AppDir 'dingtalk-req-collector.exe'
if (-not (Test-Path -LiteralPath $exe)) {
  Say "[WARN] dingtalk-req-collector.exe not found in:" 'Yellow'
  Say "       $AppDir"
  Say "       Auto-start will still be installed, but make sure the program is here."
  Say ""
}
$cfg = Join-Path $AppDir 'config.json'
if (-not (Test-Path -LiteralPath $cfg)) {
  Say "[WARN] config.json not found; the service will fail to start." 'Yellow'
  Say ""
}

$startup = [Environment]::GetFolderPath('Startup')
if ([string]::IsNullOrWhiteSpace($startup)) {
  Say "[FAILED] Cannot locate the current user's Startup folder." 'Red'
  Say "         Use Task Scheduler or NSSM instead (see README), or create a shortcut manually." 'Yellow'
  exit 1
}
$lnk = Join-Path $startup 'DingTalkReqCollector.lnk'

Say "App dir   : $AppDir"
Say "Start file: $startFile"
Say "Shortcut  : $lnk"
Say ""

try {
  $ws = New-Object -ComObject WScript.Shell
  $s = $ws.CreateShortcut($lnk)
  $s.TargetPath = $startFile
  $s.WorkingDirectory = $AppDir
  $s.Description = 'DingTalk Requirement Collector'
  $s.WindowStyle = 7
  $s.Save()
} catch {
  Say "[FAILED] Failed to create shortcut: $($_.Exception.Message)" 'Red'
  Say "         Try running this script as Administrator." 'Yellow'
  exit 1
}

if (Test-Path -LiteralPath $lnk) {
  Say "[OK] Auto-start installed." 'Green'
  Say "     It will start automatically (minimized) after the next Windows login."
  Say ""
  Say "Verify : double-click deploy\check-autostart.bat" 'Gray'
  Say "         or Win+R, type shell:startup, you should see DingTalkReqCollector.lnk" 'Gray'
  Say "Remove : double-click deploy\uninstall-autostart.bat" 'Gray'
  exit 0
} else {
  Say "[FAILED] Could not create the auto-start shortcut." 'Red'
  Say "         Run as Administrator, or create it manually (see README)." 'Yellow'
  exit 1
}
