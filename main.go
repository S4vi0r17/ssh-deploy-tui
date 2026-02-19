package main

import (
	"fmt"
	"os"

	"ssh-deploy-tui/internal/config"
	"ssh-deploy-tui/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		fmt.Printf("Error cargando configuracion: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(ui.NewModel(cfg), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
