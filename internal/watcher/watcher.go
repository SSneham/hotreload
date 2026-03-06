package watcher

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
)

var ignoredDirs = map[string]struct{}{
	".git":         {},
	"node_modules": {},
	"bin":          {},
	"build":        {},
	"tmp":          {},
}

var ignoredFileSuffixes = []string{
	".swp",
	".tmp",
	".log",
}

type Watcher struct {
	root       string
	fsWatcher  *fsnotify.Watcher
	events     chan string
	done       chan struct{}
	watchedMu  sync.Mutex
	watchedSet map[string]struct{}
}

func NewWatcher(root string) (*Watcher, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root path: %w", err)
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("stat root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("root is not a directory: %s", absRoot)
	}

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}

	return &Watcher{
		root:       absRoot,
		fsWatcher:  fsw,
		events:     make(chan string, 128),
		done:       make(chan struct{}),
		watchedSet: make(map[string]struct{}),
	}, nil
}

func (w *Watcher) Events() <-chan string {
	return w.events
}

func (w *Watcher) Start() error {
	if err := w.addRecursive(w.root); err != nil {
		return err
	}

	go w.run()
	return nil
}

func (w *Watcher) run() {
	defer close(w.events)
	defer close(w.done)

	for {
		select {
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}

			if w.ignorePath(event.Name) {
				continue
			}

			// Start watching directories created after startup.
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					_ = w.addRecursive(event.Name)
					continue
				}
			}

			// Stop watching directories that got removed or renamed.
			if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
				w.removeWatchIfDir(event.Name)
			}

			if shouldEmit(event.Op) {
				select {
				case w.events <- event.Name:
				default:
				}
			}
		case _, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
		}
	}
}

func (w *Watcher) addRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			return nil
		}

		if path != root && w.ignoreDirName(d.Name()) {
			return filepath.SkipDir
		}

		return w.addWatch(path)
	})
}

func (w *Watcher) addWatch(path string) error {
	w.watchedMu.Lock()
	if _, exists := w.watchedSet[path]; exists {
		w.watchedMu.Unlock()
		return nil
	}
	w.watchedMu.Unlock()

	if err := w.fsWatcher.Add(path); err != nil {
		return fmt.Errorf("watch %s: %w", path, err)
	}

	w.watchedMu.Lock()
	w.watchedSet[path] = struct{}{}
	w.watchedMu.Unlock()
	return nil
}

func (w *Watcher) removeWatchIfDir(path string) {
	w.watchedMu.Lock()
	_, exists := w.watchedSet[path]
	if exists {
		delete(w.watchedSet, path)
	}
	w.watchedMu.Unlock()

	if exists {
		_ = w.fsWatcher.Remove(path)
	}
}

func (w *Watcher) ignorePath(path string) bool {
	base := filepath.Base(path)
	if w.ignoreDirName(base) {
		return true
	}

	for _, suffix := range ignoredFileSuffixes {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	return false
}

func (w *Watcher) ignoreDirName(name string) bool {
	_, ignored := ignoredDirs[name]
	return ignored
}

func shouldEmit(op fsnotify.Op) bool {
	return op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename|fsnotify.Chmod) != 0
}
