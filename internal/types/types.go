package types

import (
	"sync"
)

type FileProcess struct {
	Path     string
	Status   string
	Logs     []string
	Retries  int
	Selected bool
	Mutex    sync.Mutex
}

type FileUpdate struct {
	Path   string
	Status string
	Log    string
}

type Row struct {
	Key  string
	Data []string
}

type TableItem struct {
	Type   string // "directory" or "file"
	Path   string
	File   *FileProcess
	Indent int
}
