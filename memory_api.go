package main

import (
	"fmt"

	"github.com/hasdev/forge-ade/internal/memory"
)

// ---------------------------------------------------------------------------
// Memory API
// ---------------------------------------------------------------------------

func (a *App) ListMemories() []memory.Entry {
	if a.memoryMgr == nil {
		return nil
	}
	return a.memoryMgr.List()
}

func (a *App) SaveMemory(entry memory.Entry) error {
	if a.memoryMgr == nil {
		return fmt.Errorf("memory manager unavailable")
	}
	if err := a.memoryMgr.Save(entry); err != nil {
		return err
	}
	a.emitEvent("memory:changed", map[string]interface{}{"id": entry.ID, "key": entry.Key})
	return nil
}

func (a *App) DeleteMemory(id string) error {
	if a.memoryMgr == nil {
		return fmt.Errorf("memory manager unavailable")
	}
	if err := a.memoryMgr.Delete(id); err != nil {
		return err
	}
	a.emitEvent("memory:changed", map[string]interface{}{"id": id})
	return nil
}

func (a *App) ReloadMemories() []memory.Entry {
	if a.memoryMgr == nil {
		return nil
	}
	a.memoryMgr.Reload()
	return a.memoryMgr.List()
}
