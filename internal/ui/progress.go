package ui

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/A-Djihadi/devstation/internal/plan"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// Progress affiche la progression des étapes d'un plan.
type Progress interface {
	// Handle traite un événement du plan ; à brancher comme callback de
	// plan.Execute.
	Handle(ev plan.Event)
	// Close termine l'affichage et écrit le récapitulatif de l'exécution.
	Close(s plan.Summary, logPath string)
}

// NewProgress construit un affichage de progression. titles associe l'ID
// d'étape à son titre affiché. Si interactive est vrai, un programme
// bubbletea anime un spinner et une barre de progression ; sinon la sortie
// est du texte simple, une ligne par changement d'état significatif.
func NewProgress(out io.Writer, t Theme, titles map[string]string, interactive bool) Progress {
	if interactive {
		return newProgInteractive(out, t, titles)
	}
	return &progText{out: out, theme: t, titles: titles, lastPct: map[string]int{}}
}

// progText affiche la progression en texte simple, sans séquence ANSI.
type progText struct {
	mu      sync.Mutex
	out     io.Writer
	theme   Theme
	titles  map[string]string
	lastPct map[string]int
}

// progTitle retourne le titre affiché d'une étape.
func (p *progText) progTitle(id string) string {
	if t, ok := p.titles[id]; ok && t != "" {
		return t
	}
	return id
}

// Handle traite un événement en mode texte.
func (p *progText) Handle(ev plan.Event) {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch ev.Status {
	case plan.Running:
		if ev.Total > 0 {
			pct := int(ev.Done * 100 / ev.Total)
			if pct > 100 {
				pct = 100
			}
			// Au plus une ligne par tranche de 25 %.
			bucket := pct / 25
			if last, ok := p.lastPct[ev.StepID]; ok && last == bucket {
				return
			}
			p.lastPct[ev.StepID] = bucket
			detail := fmt.Sprintf("%d%%", pct)
			fmt.Fprintln(p.out, p.theme.StepLine(plan.Running, p.progTitle(ev.StepID), detail))
			return
		}
		fmt.Fprintln(p.out, p.theme.StepLine(plan.Running, p.progTitle(ev.StepID), ev.Msg))
	case plan.Done, plan.Skipped, plan.Failed, plan.Blocked:
		fmt.Fprintln(p.out, p.theme.StepLine(ev.Status, p.progTitle(ev.StepID), ""))
	}
}

// Close écrit le récapitulatif de l'exécution.
func (p *progText) Close(s plan.Summary, logPath string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintln(p.out, p.theme.Summary(s, logPath))
}

// progInteractive affiche la progression via un programme bubbletea.
type progInteractive struct {
	mu    sync.Mutex
	prog  *tea.Program
	out   io.Writer
	theme Theme
}

// progLineMsg transporte un événement du plan vers le programme bubbletea.
type progLineMsg struct {
	ev plan.Event
}

// progCloseMsg demande l'arrêt du programme bubbletea.
type progCloseMsg struct{}

// progModel est le modèle bubbletea de l'affichage interactif.
type progModel struct {
	theme   Theme
	titles  map[string]string
	spinner spinner.Model
	bar     progress.Model
	lines   []string
	current string
	done    bool
}

// Init implémente tea.Model et démarre le spinner.
func (m progModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update implémente tea.Model et traite les messages bubbletea.
func (m progModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case progLineMsg:
		ev := msg.ev
		title := ev.StepID
		if t, ok := m.titles[ev.StepID]; ok && t != "" {
			title = t
		}
		switch ev.Status {
		case plan.Running:
			if ev.Total > 0 {
				pct := float64(ev.Done) / float64(ev.Total)
				if pct > 1 {
					pct = 1
				}
				m.current = title
				return m, m.bar.SetPercent(pct)
			}
			m.current = title
			return m, nil
		case plan.Done, plan.Skipped, plan.Failed, plan.Blocked:
			m.lines = append(m.lines, m.theme.StepLine(ev.Status, title, ""))
			m.current = ""
			return m, nil
		}
		return m, nil
	case progress.FrameMsg:
		pm, cmd := m.bar.Update(msg)
		m.bar = pm.(progress.Model)
		return m, cmd
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case progCloseMsg:
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

// View implémente tea.Model et rend l'état courant de l'affichage.
func (m progModel) View() string {
	var b strings.Builder
	for _, l := range m.lines {
		b.WriteString(l)
		b.WriteString("\n")
	}
	if m.current != "" {
		b.WriteString(m.spinner.View())
		b.WriteString(" ")
		b.WriteString(m.current)
		b.WriteString("\n")
		b.WriteString(m.bar.View())
		b.WriteString("\n")
	}
	return b.String()
}

// newProgInteractive construit l'affichage interactif et démarre le programme.
func newProgInteractive(out io.Writer, t Theme, titles map[string]string) *progInteractive {
	sp := spinner.New()
	bar := progress.New(progress.WithDefaultGradient())
	m := progModel{theme: t, titles: titles, spinner: sp, bar: bar}
	p := tea.NewProgram(m, tea.WithOutput(out))
	pi := &progInteractive{prog: p, out: out, theme: t}
	go func() {
		_, _ = p.Run()
	}()
	return pi
}

// Handle envoie l'événement au programme bubbletea.
func (p *progInteractive) Handle(ev plan.Event) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.prog == nil {
		return
	}
	p.prog.Send(progLineMsg{ev: ev})
}

// Close arrête le programme puis écrit le récapitulatif.
func (p *progInteractive) Close(s plan.Summary, logPath string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.prog != nil {
		p.prog.Send(progCloseMsg{})
		p.prog.Wait()
		p.prog = nil
	}
	out := p.out
	if out == nil {
		out = io.Discard
	}
	fmt.Fprintln(out, p.theme.Summary(s, logPath))
}
