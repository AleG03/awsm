package daemon

import (
	"os/exec"
	"strings"
)

// notify raises a balloon through PowerShell and Windows Forms.
//
// Toast notifications proper need an application identity registered with the
// system, which a CLI does not have; the tray balloon needs nothing and is
// visible in the same place.
func notify(title, message string) error {
	script := `
[void][System.Reflection.Assembly]::LoadWithPartialName('System.Windows.Forms')
$n = New-Object System.Windows.Forms.NotifyIcon
$n.Icon = [System.Drawing.SystemIcons]::Information
$n.BalloonTipTitle = ` + psQuote(title) + `
$n.BalloonTipText = ` + psQuote(message) + `
$n.Visible = $true
$n.ShowBalloonTip(10000)
Start-Sleep -Seconds 10
$n.Dispose()`
	return exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).Run()
}

// psQuote renders a Go string as a PowerShell single-quoted literal, where the
// only escape is a doubled quote.
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
