package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	CountPerPage = 15
)

type model struct {
	spinner     spinner.Model
	quitting    bool
	loadingWd   bool
	viewingWd   bool
	loadingNode bool
	viewingNode bool
	deleting    bool
	cursor      int
	deleted     map[int]struct{}
	err         error
	result      string
	totalSize   int64
	dirs        []dirInfo
	curPage     int
	totalPage   int
}

type dirInfo struct {
	ModTime time.Time
	Path    string
	Size    int64
}

type GotNodeDirsMsg struct {
	Dirs      []dirInfo
	TotalSize int64
}

type GotWdDirsMsg struct {
	Dirs []dirInfo
}

type DeletionSuccessMsg struct {
	RemainingSize int64
}

func initialModel() model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return model{
		spinner:     s,
		loadingWd:   true,
		loadingNode: false,
		deleting:    false,
		quitting:    false,
		err:         nil,
		result:      "",
		dirs:        []dirInfo{},
		deleted:     make(map[int]struct{}),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		spinner.Tick,
		m.getDirsInWd(),
	)
}

func (m model) getDirsInWd() tea.Cmd {
	return func() tea.Msg {
		startingPath := os.Args[1]
		fmt.Printf("Current path: %s\n", startingPath)
		var dirs = []dirInfo{}
		err := filepath.WalkDir(startingPath, func(path string, item fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if item.IsDir() && path != startingPath {
				var dir dirInfo
				fileInfo, _ := item.Info()
				dir = dirInfo{
					ModTime: fileInfo.ModTime(),
					Path:    path,
					Size:    fileInfo.Size(),
				}
				dirs = append(dirs, dir)
				return filepath.SkipDir
			}
			return err
		})
		if err != nil {
			m.err = err
		}
		return GotWdDirsMsg{Dirs: dirs}
	}
}

func (m model) getNodeDirs(searchPath string) tea.Cmd {
	return func() tea.Msg {
		m.cursor = 0
		m.curPage = 0
		var dirs = []dirInfo{}
		err := filepath.WalkDir(searchPath, func(path string, item fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			var dir dirInfo
			fileInfo, _ := item.Info()
			if item.IsDir() && item.Name() == "node_modules" {
				dir = dirInfo{
					ModTime: fileInfo.ModTime(),
					Path:    path,
					Size:    fileInfo.Size(),
				}
				dirs = append(dirs, dir)
				return filepath.SkipDir
			}
			return err
		})
		if err != nil {
			m.err = err
		}
		var totalSize int64
		for i, dir := range dirs {
			var dirSize int64
			err := filepath.WalkDir(dir.Path, func(path string, info fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() {
					fileInfo, _ := info.Info()
					totalSize += fileInfo.Size()
					dirSize += fileInfo.Size()
				}
				return err
			})
			if err != nil {
				m.err = err
			}
			dirs[i].Size = dirSize
		}
		return GotNodeDirsMsg{Dirs: dirs, TotalSize: totalSize}
	}
}

func (m model) deleteDirs() tea.Cmd {
	return func() tea.Msg {
		for _, dir := range m.dirs {
			// if not modified in the last 2 months, delete
			if dir.ModTime.Before(time.Now().AddDate(0, -2, 0)) {
				/* err := os.RemoveAll(dir.Path)
				if err != nil {
					m.err = err
				} */
				// remove the dir from the list
				m.dirs = append(m.dirs[:0], m.dirs[1:]...)
			}
		}
		// get the total size of the remaining directories
		var remainingSize int64
		for _, dir := range m.dirs {
			err := filepath.WalkDir(dir.Path, func(path string, info fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				fileInfo, _ := info.Info()
				if !info.IsDir() {
					remainingSize += fileInfo.Size()
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

func (m model) deleteDir(path string) tea.Cmd {
	return func() tea.Msg {
		/* err := os.RemoveAll(path)
		   if err != nil {
		       m.err = err
		   } */
		fmt.Printf("Deleted %s\n", path)
		return nil
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
			if m.viewingWd {
				m.loadingNode = true
				return m, tea.Batch(
					spinner.Tick,
					m.getNodeDirs(m.dirs[m.cursor].Path),
				)
			}
			if m.viewingNode {
				return m, m.deleteDir(m.dirs[m.cursor].Path)
			}
		case "y":
			m.deleting = true
			return m, tea.Batch(
				spinner.Tick,
				m.deleteDirs(),
			)
		// The "up" and "k" keys move the cursor up
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		// The "down" and "j" keys move the cursor down
		case "down", "j":
			if m.cursor < len(m.dirs)-1 {
				m.cursor++
			}
		case "pgdown":
			if m.curPage < m.totalPage-1 {
				m.curPage++
			}
			return m, nil
		case "pgup":
			if m.curPage > 0 {
				m.curPage--
			}
			return m, nil
		}
	case GotNodeDirsMsg:
		m.loadingNode = false
		m.viewingNode = true
		// print total size in gigabytes
		m.result = fmt.Sprintf("Total size: %.2f GB\n\nDelete everything not modified in the last 2 months? y/n", float64(msg.TotalSize)/(1<<30))
		m.dirs = msg.Dirs
		m.totalSize = msg.TotalSize
		m.totalPage = (len(msg.Dirs) + CountPerPage - 1) / CountPerPage
		return m, nil

	case GotWdDirsMsg:
		m.loadingWd = false
		m.viewingWd = true
		m.dirs = msg.Dirs
		m.totalPage = (len(msg.Dirs) + CountPerPage - 1) / CountPerPage
		return m, nil

	case DeletionSuccessMsg:
		m.deleting = false
		m.result = fmt.Sprintf("Deleted %.2f GB\n\n", float64(m.totalSize-msg.RemainingSize)/(1<<30))
		return m, nil
	}

	if m.loadingNode || m.loadingWd || m.deleting {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	// Return the updated model to the Bubble Tea runtime for processing.
	// Note that we're not returning a command.
	return m, nil
}

func (m model) View() string {
	start, end := m.curPage*CountPerPage, (m.curPage+1)*CountPerPage
	if end > len(m.dirs) {
		end = len(m.dirs)
	}
	if m.loadingWd {
		return fmt.Sprintf("%s Loading working directory", m.spinner.View())
	}
	if m.loadingNode {
		return fmt.Sprintf("%s Loading node_modules in selected directory", m.spinner.View())
	}
	if m.deleting {
		return fmt.Sprintf("%s Deleting directories", m.spinner.View())
	}
	if len(m.dirs) > 0 {
		// Iterate over our choices
		var s string
		s += fmt.Sprintf("current page: %d, total pages: %d \n", m.curPage, m.totalPage)
		for i, dir := range m.dirs[start:end] {

			// Is the cursor pointing at this choice?
			cursor := " " // no cursor
			if m.cursor == i {
				var style = lipgloss.NewStyle().Foreground(lipgloss.Color("201"))
				cursor = style.Render(">") // cursor!
			}

			// Is this choice selected?
			checked := " " // not selected
			if _, ok := m.deleted[i]; ok {
				var style = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575"))
				checked = style.Render("DELETED") // selected!
			}

			// Render the row
			if m.viewingNode {
				var style = lipgloss.NewStyle().Align(lipgloss.Right)
				s += fmt.Sprintln(style.Render(fmt.Sprintf("%s [%s] %s\t %.2f MB", cursor, checked, dir.Path, float64(dir.Size)/(1<<20))))
			} else {
				var style = lipgloss.NewStyle().Align(lipgloss.Right)
				s += fmt.Sprintln(style.Render(fmt.Sprintf("%s - %s", cursor, dir.Path)))
			}
		}
		if m.totalPage > 1 {
			var style = lipgloss.NewStyle().Foreground(lipgloss.Color("#3C3C3C"))
			s += style.Render("Pagedown to next page, pageup to prev page.")
			s += "\n"
		}
		return s
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
