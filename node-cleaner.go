package main

import (
	"flag"
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
	loadingWd   bool
	viewingWd   bool
	loadingNode bool
	viewingNode bool
	deleting    map[int]struct{}
	cursor      int
	deleted     map[int]struct{}
	err         error
	result      string
	totalSize   int64
	direct      bool
	dirs        []dirInfo
	curPage     int
	totalPage   int
}

type dirInfo struct {
	ModTime time.Time
	Path    string
	Size    int64
}

func initialModel(direct bool) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return model{
		spinner:     s,
		loadingWd:   !direct,
		loadingNode: direct,
		deleting:    make(map[int]struct{}),
		direct:      direct,
		err:         nil,
		result:      "",
		dirs:        []dirInfo{},
		deleted:     make(map[int]struct{}),
	}
}

func (m model) Init() tea.Cmd {
	if m.direct {
		return tea.Batch(
			spinner.Tick,
			m.getNodeDirs(os.Args[2]),
		)
	} else {
		return tea.Batch(
			spinner.Tick,
			m.getDirsInWd(),
		)
	}
}

func (m model) getDirsInWd() tea.Cmd {
	return func() tea.Msg {
		startingPath := os.Args[1]
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

func (m model) deleteDir(path string) tea.Cmd {
	return func() tea.Msg {
		m.deleting[m.cursor] = struct{}{}
		err := os.RemoveAll(path)
		if err != nil {
			m.err = err
		}
		return DeletionSuccessMsg{
			DeletedSize: m.dirs[m.cursor].Size,
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
		// The "up" and "k" keys move the cursor up
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		// The "down" and "j" keys move the cursor down
		case "down", "j":
			if len(m.dirs) < CountPerPage {
				if m.cursor < len(m.dirs)-1 {
					m.cursor++
				}
			} else {
				if m.cursor < CountPerPage-1 {
					m.cursor++
				}
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
		delete(m.deleting, m.cursor)
		m.deleted[m.cursor] = struct{}{}
		return m, nil
	}

	if m.loadingNode || m.loadingWd {
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
	if len(m.dirs) > 0 {
		// Iterate over our choices
		var s string
		s += fmt.Sprintf("current page: %d, total pages: %d \n", m.curPage, m.totalPage)
		if m.direct {
			s += fmt.Sprintln("Direct mode")
		}
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

			deleting := " "
			if _, ok := m.deleting[i]; ok {
				var style = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
				deleting = style.Render(fmt.Sprintf("%s DELETING", m.spinner.View()))
			}

			// Render the row
			if m.viewingNode {
				var style = lipgloss.NewStyle().Align(lipgloss.Right)
				s += fmt.Sprintln(style.Render(fmt.Sprintf("%s [%s%s] %s\t %.2f MB", cursor, checked, deleting, dir.Path, float64(dir.Size)/(1<<20))))
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
		return "Something went wrong, please try again.\n\nMake sure to format the call in one of the following formats:\n\n" +
			"Directly looking up node_modules folders in given path:\tnode-cleaner -d <path>\n" +
			"List folders in given path and look for folders in the selected directory:\tnode-cleaner <path>\n"
	}
}

func main() {
	direct := flag.Bool("direct", false, "directly look up node_modules")
	gui := flag.Bool("gui", false, "use gui")
	flag.Parse()
	if *gui {
		mainFyne()
	} else {
		p := tea.NewProgram(
			initialModel(*direct),
			tea.WithAltScreen(),       // use the full size of the terminal in its "alternate screen buffer"
			tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
		)
		if err := p.Start(); err != nil {
			fmt.Printf("Alas, there's been an error: %v", err)
			os.Exit(1)
		}
	}
}
