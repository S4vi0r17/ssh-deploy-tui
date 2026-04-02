package main

import (
	"fmt"
	"os"

	"os/user"
	"path/filepath"

	"sdt/internal/config"
	"sdt/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func getConfigPath() string {
	// 1. If config.yaml exists in current directory, use it
	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml"
	}
	// 2. Otherwise, use ~/.config/sdt/config.yaml
	if u, err := user.Current(); err == nil {
		return filepath.Join(u.HomeDir, ".config", "sdt", "config.yaml")
	}
	return "config.yaml"
}

func main() {
	configPath := getConfigPath()
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Printf("Error cargando configuracion (%s): %v\n", configPath, err)
		os.Exit(1)
	}

	p := tea.NewProgram(ui.NewModel(cfg), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
