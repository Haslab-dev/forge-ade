package app

import (
	"fmt"

	"github.com/hasdev/forge-ade/internal/plugins"
)

// ---------------------------------------------------------------------------
// Plugins API
// ---------------------------------------------------------------------------

func (a *App) ListPlugins() []*plugins.Plugin {
	if a.pluginMgr == nil {
		return nil
	}
	return a.pluginMgr.List()
}

func (a *App) GetPlugin(id string) (*plugins.Plugin, error) {
	if a.pluginMgr == nil {
		return nil, fmt.Errorf("plugin manager not initialized")
	}
	p, ok := a.pluginMgr.Get(id)
	if !ok {
		return nil, fmt.Errorf("plugin %q not found", id)
	}
	return p, nil
}

func (a *App) CreatePlugin(req plugins.CreatePluginRequest) (*plugins.Plugin, error) {
	if a.pluginMgr == nil {
		return nil, fmt.Errorf("plugin manager not initialized")
	}
	wsFolder := ""
	if ws := a.workspaceMgr.Current(); ws != nil {
		folders := ws.GetFolders()
		if len(folders) > 0 {
			wsFolder = folders[0]
		}
	}
	p, err := a.pluginMgr.CreatePlugin(req, wsFolder)
	if err != nil {
		return nil, err
	}
	a.emitEvent("plugins:changed", map[string]interface{}{"id": p.ID})
	return p, nil
}

func (a *App) TogglePlugin(id string, enabled bool) error {
	if a.pluginMgr == nil {
		return fmt.Errorf("plugin manager not initialized")
	}
	err := a.pluginMgr.TogglePlugin(id, enabled)
	if err == nil {
		a.emitEvent("plugins:changed", map[string]interface{}{"id": id, "enabled": enabled})
	}
	return err
}

func (a *App) DeletePlugin(id string) error {
	if a.pluginMgr == nil {
		return fmt.Errorf("plugin manager not initialized")
	}
	err := a.pluginMgr.DeletePlugin(id)
	if err == nil {
		a.emitEvent("plugins:changed", map[string]interface{}{"id": id})
	}
	return err
}

func (a *App) ReloadPlugins() []*plugins.Plugin {
	if a.pluginMgr == nil {
		return nil
	}
	a.pluginMgr.Reload()
	return a.pluginMgr.List()
}
