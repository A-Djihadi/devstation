//go:build tools

// Package internal épingle dans go.mod les dépendances prévues par le plan :
// le sandbox de test étant hors-ligne, aucune dépendance ne peut être ajoutée ensuite.
package internal

import (
	_ "github.com/charmbracelet/bubbles/progress"
	_ "github.com/charmbracelet/bubbletea"
	_ "github.com/charmbracelet/huh"
	_ "github.com/charmbracelet/lipgloss"
	_ "github.com/danieljoos/wincred"
	_ "github.com/spf13/cobra"
	_ "golang.org/x/sys/windows"
	_ "golang.org/x/sys/windows/registry"
	_ "gopkg.in/yaml.v3"
)
