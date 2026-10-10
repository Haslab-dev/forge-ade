package app

import (
	"fmt"

	"github.com/hasdev/forge-ade/internal/skills"
)

func (a *App) ListSkills() []skills.Skill {
	if a.skillMgr == nil {
		return nil
	}
	return a.skillMgr.List()
}

func (a *App) CreateSkill(req skills.CreateSkillRequest) (*skills.Skill, error) {
	if a.skillMgr == nil {
		return nil, fmt.Errorf("skill manager not initialized")
	}
	wsFolder := ""
	if ws := a.workspaceMgr.Current(); ws != nil {
		folders := ws.GetFolders()
		if len(folders) > 0 {
			wsFolder = folders[0]
		}
	}
	sk, err := a.skillMgr.CreateSkill(req, wsFolder)
	if err != nil {
		return nil, err
	}
	a.emitEvent("skills:changed", map[string]interface{}{"name": sk.Name})
	return sk, nil
}

func (a *App) ReloadSkills() []skills.Skill {
	if a.skillMgr == nil {
		return nil
	}
	a.skillMgr.Reload()
	return a.skillMgr.List()
}

func (a *App) DeleteSkill(name string) error {
	if a.skillMgr == nil {
		return fmt.Errorf("skill manager not initialized")
	}
	err := a.skillMgr.DeleteSkill(name)
	if err == nil {
		a.emitEvent("skills:changed", map[string]interface{}{"name": name})
	}
	return err
}
