param()
$ErrorActionPreference = 'Continue'

try { [Console]::OutputEncoding = [Text.Encoding]::UTF8 } catch {}

function Say([string]$msg, [string]$color = 'Gray') {
  Write-Host $msg -ForegroundColor $color
}

Say "============================================"
Say "  DingTalk Requirement Collector - remove auto-start" 'Cyan'
Say "============================================"
Say ""

$startup = [Environment]::GetFolderPath('Startup')
$lnk = Join-Path $startup 'DingTalkReqCollector.lnk'

if (Test-Path -LiteralPath $lnk) {
  try {
    Remove-Item -LiteralPath $lnk -Force
    Say "[REMOVED] $lnk" 'Green'
  } catch {
    Say "[FAILED] Could not remove: $($_.Exception.Message)" 'Red'
    exit 1
  }
} else {
  Say "[NOT INSTALLED] No DingTalkReqCollector.lnk in the Startup folder" 'Yellow'
}
Say ""
Say "Note: this only removes auto-start; it does not stop a running service." 'Gray'
Say "      To stop: end dingtalk-req-collector.exe in Task Manager"
