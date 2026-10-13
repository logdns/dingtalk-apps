param(
  [Parameter(Mandatory = $true)][string]$AppDir
)
$ErrorActionPreference = 'Continue'

try { [Console]::OutputEncoding = [Text.Encoding]::UTF8 } catch {}

function Say([string]$msg, [string]$color = 'Gray') {
  Write-Host $msg -ForegroundColor $color
}

Say "============================================"
Say "  DingTalk Requirement Collector - auto-start self check" 'Cyan'
Say "============================================"
Say ""

$startup = [Environment]::GetFolderPath('Startup')
if ([string]::IsNullOrWhiteSpace($startup)) {
  Say "[FAILED] Cannot locate the current user's Startup folder." 'Red'
  exit 1
}
$lnk = Join-Path $startup 'DingTalkReqCollector.lnk'

Say "--- Auto-start ---"
if (Test-Path -LiteralPath $lnk) {
  Say "[INSTALLED] Auto-start" 'Green'
  Say "  Shortcut  : $lnk"
  try {
    $ws = New-Object -ComObject WScript.Shell
    $s = $ws.CreateShortcut($lnk)
    Say "  Target    : $($s.TargetPath)"
    Say "  Workdir   : $($s.WorkingDirectory)"
    if (-not (Test-Path -LiteralPath $s.TargetPath)) {
      Say "  [WARN] Target script does not exist! Re-run install-autostart.bat" 'Yellow'
    }
    $wd = $s.WorkingDirectory
    if ($wd -and -not (Test-Path -LiteralPath (Join-Path $wd 'dingtalk-req-collector.exe'))) {
      Say "  [WARN] dingtalk-req-collector.exe not found in the working directory" 'Yellow'
    }
    if ($wd -and -not (Test-Path -LiteralPath (Join-Path $wd 'config.json'))) {
      Say "  [WARN] config.json not found in the working directory" 'Yellow'
    }
  } catch {
    Say "  [WARN] Could not read shortcut: $($_.Exception.Message)" 'Yellow'
  }
} else {
  Say "[NOT INSTALLED] No DingTalkReqCollector.lnk in the Startup folder" 'Red'
  Say "  Install: double-click deploy\install-autostart.bat"
}
Say ""

Say "--- Service status (port 1450) ---"
$listen = Get-NetTCPConnection -LocalPort 1450 -State Listen -ErrorAction SilentlyContinue
if ($listen) {
  Say "[RUNNING] Port 1450 is listening" 'Green'
  foreach ($c in $listen) {
    $procName = ''
    try { $procName = (Get-Process -Id $c.OwningProcess -ErrorAction Stop).ProcessName } catch {}
    Say "  $($c.LocalAddress):$($c.LocalPort)  PID=$($c.OwningProcess) $procName"
  }
} else {
  Say "[NOT RUNNING] Port 1450 is not listening" 'Yellow'
  Say "  If it just booted, wait a few seconds; otherwise check the log below"
}
Say ""

Say "--- Recent log (logs\server.log, last 15 lines) ---"
$log = Join-Path (Join-Path $AppDir 'logs') 'server.log'
if (Test-Path -LiteralPath $log) {
  Get-Content -LiteralPath $log -Tail 15 -Encoding UTF8 | ForEach-Object { Write-Host $_ }
} else {
  Say "Not found: $log (never started, or log_dir was changed)" 'Yellow'
}
Say ""

Say "--- Quick check (open in browser) ---"
Say "  http://127.0.0.1:1450/"
