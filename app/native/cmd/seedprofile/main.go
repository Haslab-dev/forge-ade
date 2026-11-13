package main

import (
	"fmt"
	"os"

	"github.com/Haslab-dev/RakitKode/llm"
)

func main() {
	key := os.Getenv("FORGE_AUDIT_KEY")
	if key == "" {
		fmt.Println("FORGE_AUDIT_KEY required")
		os.Exit(1)
	}
	home, _ := os.UserHomeDir()
	c := llm.NewLLMClient(home + "/.forge-ade")
	if err := c.SaveProfile("kenari", key, "https://kenari.id/v1", "muse-glimmer-30b:free"); err != nil {
		fmt.Println("ERR", err)
		os.Exit(1)
	}
	cfg := c.GetConfig()
	fmt.Printf("seeded: provider=%s model=%s base=%s\n", cfg.ProviderID, cfg.Model, cfg.BaseURL)
}
