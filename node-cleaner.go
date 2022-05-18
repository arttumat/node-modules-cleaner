package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	spinner   spinner.Model
	quitting  bool
	loading   bool
	deleting  bool
	err       error
	result    string
	totalSize int64
	dirs      []dirInfo
}

type dirInfo struct {
	ModTime time.Time
	Path    string
	Size    int64
}

type GotDirsMsg struct {
	Dirs      []dirInfo
	TotalSize int64
}

type DeletionSuccessMsg struct {
	RemainingSize int64
}

func initialModel() model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return model{
		spinner:  s,
		loading:  false,
		deleting: false,
		quitting: false,
		err:      nil,
		result:   "",
		dirs:     []dirInfo{},
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) getDirs() tea.Cmd {
	return func() tea.Msg {
		home, _ := os.UserHomeDir()
		var dirs = []dirInfo{}
		err := filepath.Walk(home, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			var dir dirInfo
			if info.IsDir() && info.Name() == "node_modules" {
				dir = dirInfo{
					ModTime: info.ModTime(),
					Path:    path,
					Size:    info.Size(),
				}
				dirs = append(dirs, dir)
			}
			return err
		})
		if err != nil {
			m.err = err
		}
		var totalSize int64
		for _, dir := range dirs {
			err := filepath.Walk(dir.Path, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() {
					totalSize += info.Size()
				}
				return err
			})
			if err != nil {
				m.err = err
			}
		}
		return GotDirsMsg{Dirs: dirs, TotalSize: totalSize}
	}
}

func (m model) deleteDirs() tea.Cmd {
	return func() tea.Msg {
		for _, dir := range m.dirs {
			// if not modified in the last 2 months, delete
			if dir.ModTime.Before(time.Now().AddDate(0, -2, 0)) {
				err := os.RemoveAll(dir.Path)
				if err != nil {
					m.err = err
				}
				// remove the dir from the list
				m.dirs = append(m.dirs[:0], m.dirs[1:]...)
			}
		}
		// get the total size of the remaining directories
		var remainingSize int64
		for _, dir := range m.dirs {
			err := filepath.Walk(dir.Path, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() {
					remainingSize += info.Size()
				}
				return err
			})
			if err != nil {
				m.err = err
			}
		}
		return DeletionSuccessMsg{
			RemainingSize: remainingSize,
		}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// Is it a key press?
	case tea.KeyMsg:

		// Cool, what was the actual key pressed?
		switch msg.String() {

		// These keys should exit the program.
		case "ctrl+c", "q", "n":
			m.quitting = true
			return m, tea.Quit
		// Press enter to list all node_modules directories
		case "enter":
			m.loading = true
			return m, tea.Batch(
				spinner.Tick,
				m.getDirs(),
			)
		case "y":
			m.deleting = true
			return m, tea.Batch(
				spinner.Tick,
				m.deleteDirs(),
			)
		}
	case GotDirsMsg:
		m.loading = false
		// print total size in gigabytes
		m.result = fmt.Sprintf("Total size: %.2f GB\n\nDelete everything not modified in the last 2 months? y/n", float64(msg.TotalSize)/(1<<30))
		m.dirs = msg.Dirs
		m.totalSize = msg.TotalSize
		return m, nil

	case DeletionSuccessMsg:
		m.deleting = false
		m.result = fmt.Sprintf("Deleted %.2f GB\n\n", float64(m.totalSize-msg.RemainingSize)/(1<<30))
		return m, nil
	}

	if m.loading || m.deleting {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	// Return the updated model to the Bubble Tea runtime for processing.
	// Note that we're not returning a command.
	return m, nil
}

func (m model) View() string {
	if m.loading {
		return fmt.Sprintf("%s Loading directories", m.spinner.View())
	}
	if m.deleting {
		return fmt.Sprintf("%s Deleting directories", m.spinner.View())
	}
	if len(m.result) > 0 {
		return m.result
	} else {
		return "\n\nPress enter to list all node_modules directories\n\nPress q to quit.\n"
	}
}

func main() {
	p := tea.NewProgram(
		initialModel(),
		tea.WithAltScreen(),       // use the full size of the terminal in its "alternate screen buffer"
		tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
	)
	if err := p.Start(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
