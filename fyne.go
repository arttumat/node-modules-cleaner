package main

import (
	"fmt"
	"image/color"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

func deleteDir(path string) (DeletionSuccessMsg, error) {
	err := os.RemoveAll(path)
	if err != nil {
		log.Println(err)
	}
	return DeletionSuccessMsg{
		DeletedSize: 0,
	}, err
}

func getDirsInPath(startingPath string) ([]dirInfo, error) {
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
		return nil, err
	}
	return dirs, nil
}

func getNodeDirsFyne(path string) ([]dirInfo, int64, error) {
	var dirs = []dirInfo{}
	err := filepath.WalkDir(path, func(path string, item fs.DirEntry, err error) error {
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
		return nil, 0, err
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
			return nil, 0, err
		}
		dirs[i].Size = dirSize
	}
	return dirs, totalSize, nil
}

func mainFyne() {
	a := app.New()
	win := a.NewWindow("node_modules cleaner")
	win.Resize(fyne.NewSize(640, 480))

	title := canvas.NewText("Cleaner", color.White)
	title.TextStyle = fyne.TextStyle{
		Bold: true,
	}
	title.Alignment = fyne.TextAlignCenter
	title.TextSize = 24

	input := widget.NewEntry()
	input.SetPlaceHolder("Enter path...")
	loading := widget.NewProgressBarInfinite()
	loading.Resize(fyne.NewSize(loading.Size().Width, 30))
	loading.Hide()
	var directories []dirInfo

	homeButton := widget.NewButton("Get home directory", func() {
		// get home directory
		loading.Show()
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return
		}
		dirs, err := getDirsInPath(homeDir)
		if err != nil {
			return
		}
		directories = dirs
		loading.Hide()
	})
	openPathButton := widget.NewButton("Open path", func() {
		// open path
		loading.Show()
		dirs, err := getDirsInPath(input.Text)
		if err != nil {
			return
		}
		directories = dirs
		loading.Hide()
	})
	searchButton := widget.NewButton("Search for node_modules in path", func() {
		loading.Show()
		dirs, totalSize, err := getNodeDirsFyne(input.Text)
		if err != nil {
			return
		}
		fmt.Println(totalSize)
		directories = dirs
		loading.Hide()
	})

	var selectedDir dirInfo
	var selectedDirIndex int

	list := widget.NewList(
		func() int {
			return len(directories)
		},
		func() fyne.CanvasObject {
			item := widget.NewLabel("Template")
			item.Wrapping = fyne.TextTruncate
			return item
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(fmt.Sprintf("%s\t-\t%.2f MB", directories[i].Path, float64(directories[i].Size)/(1<<20)))
		})
	searchSelectedDirButton := widget.NewButton("Search selected directory", func() {
		loading.Show()
		dirs, totalSize, err := getNodeDirsFyne(selectedDir.Path)
		if err != nil {
			return
		}
		fmt.Println(totalSize)
		directories = dirs
		loading.Hide()
	})
	searchSelectedDirButton.Disable()

	cleanSelectedDirButton := widget.NewButton("Clean selected directory", func() {
		loading.Show()
		_, err := deleteDir(selectedDir.Path)
		if err != nil {
			return
		}
		directories = directories[:selectedDirIndex]
		loading.Hide()
	})
	cleanSelectedDirButton.Disable()
	list.OnSelected = func(id widget.ListItemID) {
		/* deleteDir(directories[id].Path, loading) */
		selectedDir = directories[id]
		selectedDirIndex = id
		searchSelectedDirButton.Enable()
		cleanSelectedDirButton.Enable()
	}

	hBox1 := container.New(layout.NewGridLayout(3), homeButton, openPathButton, searchButton)
	hBox2 := container.New(layout.NewGridLayout(2), searchSelectedDirButton, cleanSelectedDirButton)
	vBox := container.New(layout.NewVBoxLayout(), title, input, hBox1, hBox2)
	center := container.New(layout.NewMaxLayout(), list)
	mainGrid := container.New(layout.NewGridLayout(1), vBox, loading, center)

	win.SetContent(mainGrid)
	win.ShowAndRun()
}
