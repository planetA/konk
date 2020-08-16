package container

import (
	"os"
	"time"
)

type HelloArgs struct {
	Say string
}

type ImageInfoArgs struct {
	Rank       Rank
	ID         string
	Args       []string
	Generation int // Checkpoint generation number
	Parent     int // Parent checkpoint generation number
}

type LinkInfoArgs struct {
	Filename string
	Link     string
	Size     int64
	Mode     os.FileMode
	ModTime  time.Time
}

type FileInfoArgs struct {
	Filename string
	Size     int64
	Mode     os.FileMode
	ModTime  time.Time
}

type FileDataArgs struct {
	Data []byte
}

type StartPageServerArgs struct {
	CheckpointPath string
}

type RelaunchArgs struct {
}
