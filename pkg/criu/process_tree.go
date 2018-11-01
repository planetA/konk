package criu

import (
	"fmt"
	"log"
	"strings"

	"github.com/shirou/gopsutil/process"
)

func getProcessTree(p *process.Process) ([]*process.Process, error) {
	tree := make([]*process.Process, 1)
	tree[0] = p

	uncolored := make([]*process.Process, 1)
	uncolored[0] = p

	for {
		if len(uncolored) == 0 {
			break
		}

		head, rest := uncolored[0], uncolored[1:]

		children, err := head.Children()
		if err != nil && err != process.ErrorNoChildren {
			return nil, err
		}

		rest = append(rest, children...)
		tree = append(tree, children...)

		uncolored = rest
	}

	return tree, nil
}

func getOpenFiles(plist []*process.Process) ([]process.OpenFilesStat, error) {
	flist := make([]process.OpenFilesStat, 0)
	for _, p := range plist {
		openFiles, err := p.OpenFiles()
		if err != nil {
			return nil, err
		}

		flist = append(flist, openFiles...)
	}

	return flist, nil
}

func getOpenFilesPrefix(pid int, prefix string) ([]string, error) {
	root, err := process.NewProcess(int32(pid))
	if err != nil {
		return nil, fmt.Errorf("NewProcess (%v): %v", pid, err)
	}

	log.Println("Root: ", root)

	tree, err := getProcessTree(root)
	if err != nil {
		return nil, fmt.Errorf("Failed to gather process tree: %v", err)
	}

	log.Println("Process tree: ", tree)

	openFiles, err := getOpenFiles(tree)
	if err != nil {
		return nil, fmt.Errorf("Failed to get open files: %v", err)
	}

	filtered := make([]string, 0)
	for _, f := range openFiles {
		if !strings.HasPrefix(f.Path, prefix) {
			continue
		}
		filtered = append(filtered, f.Path)
	}

	return filtered, nil
}
