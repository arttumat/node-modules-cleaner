package main

type GotNodeDirsMsg struct {
	Dirs      []dirInfo
	TotalSize int64
}

type GotWdDirsMsg struct {
	Dirs []dirInfo
}

type DeletionSuccessMsg struct {
	DeletedSize int64
}
