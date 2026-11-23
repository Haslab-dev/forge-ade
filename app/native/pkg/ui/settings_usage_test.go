package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/settings"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	"github.com/hasdev/forge-ade/app/native/pkg/usage"
)

func TestRenderSettingsTabUsage(t *testing.T) {
	appUI := &AppUI{
		ThemeMgr: theme.NewThemeManager(),
	}
	appUI.Settings = settings.NewSettingsManager(nil, nil)

	tmpDir := t.TempDir()
	appUI.UsageMgr = usage.NewUsageManager(tmpDir, nil)

	// Record some sample turns
	appUI.UsageMgr.RecordTurn("s1", "Task 1", "deepseek-v4-1-flash", "DeepSeek", 1200, 300, 600, 1500, 0.003)
	appUI.UsageMgr.RecordTurn("s2", "Task 2", "gemini-2.5-pro", "Google Gemini", 2500, 500, 1250, 3000, 0.007)

	tc := appUI.ThemeMgr.Colors()

	// Verify renderSettingsTabUsage runs without panics in Model grouping mode
	appUI.Settings.UsageGroupBy = "model"
	appUI.Settings.UsageTimeRange = "7d"
	ui.NewTesterWith(func(c *ui.Context) {
		appUI.renderSettingsTabUsage(c, tc, appUI.Settings)
	}, 1280, 800, ui.TesterOptions{Dark: true})

	// Verify renderSettingsTabUsage runs in Provider grouping mode
	appUI.Settings.UsageGroupBy = "provider"
	ui.NewTesterWith(func(c *ui.Context) {
		appUI.renderSettingsTabUsage(c, tc, appUI.Settings)
	}, 1280, 800, ui.TesterOptions{Dark: true})

	// Verify renderSettingsTabUsage runs in Custom Date Range mode
	appUI.Settings.UsageTimeRange = "custom"
	appUI.Settings.UsageCustomStart = time.Now().Add(-2 * 24 * time.Hour).Format("2006-01-02")
	appUI.Settings.UsageCustomEnd = time.Now().Format("2006-01-02")
	ui.NewTesterWith(func(c *ui.Context) {
		appUI.renderSettingsTabUsage(c, tc, appUI.Settings)
	}, 1280, 800, ui.TesterOptions{Dark: true})
}
