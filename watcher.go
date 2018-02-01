package chimera

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	fsnotify "github.com/fsnotify/fsnotify"
)

type TargetDir struct {
	Name       string
	ConfigLogs []*ConfigLogfile
}

type TargetFile struct {
	Name          string
	Timestamp     time.Time
	EventCh       chan fsnotify.Event
	ConfigLogfile *ConfigLogfile
	Cancel        func()
}

type Watcher struct {
	watcher      *fsnotify.Watcher
	configLogs   []*ConfigLogfile
	watchingDir  map[string]*TargetDir
	watchingFile map[string]*TargetFile
	reverseMap   map[string]string
	initialized  bool
}

func NewWatcher(configLogs []*ConfigLogfile) (*Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Println("[error] Couldn't create file watcher", err)
		return nil, err
	}
	w := &Watcher{
		watcher:    watcher,
		configLogs: configLogs,
		reverseMap: make(map[string]string),
	}
	return w, nil
}

func (w *Watcher) Run(ctx context.Context, c *Circumstances) {
	c.InputProcess.Add(1)
	defer c.InputProcess.Done()

	if err := w.initialize(ctx, c); err != nil {
		log.Println("[warn] failed to start watcher: ", err)
		return
	}

	if len(w.watchingDir) == 0 && len(w.watchingFile) == 0 {
		// no need to watch
		log.Println("[warn] nothing to watch")
		return
	}

	log.Println("[info] start file watcher")
	for {
		select {
		case <-ctx.Done():
			log.Println("[info] shutdown file watcher")
			return
		case ev := <-w.watcher.Events:
			log.Println("[debug] watcher event", ev)
			w.dispatchWatcherEvent(ctx, c, ev)
		case err := <-w.watcher.Errors:
			log.Println("[warn] watcher error", err)
		}
	}
}

func (w *Watcher) dispatchWatcherEvent(ctx context.Context, c *Circumstances, ev fsnotify.Event) {
	switch {
	case ev.Op&fsnotify.Create == fsnotify.Create:
		w.onCreate(ctx, c, ev)
	case ev.Op&fsnotify.Remove == fsnotify.Remove:
		fallthrough
	case ev.Op&fsnotify.Rename == fsnotify.Rename:
		w.onDeleteOrRename(ev)
	case ev.Op&fsnotify.Write == fsnotify.Write:
		w.onModify(ev)
	}
}

func (w *Watcher) onCreate(ctx context.Context, c *Circumstances, ev fsnotify.Event) {
	stat, err := os.Stat(ev.Name)
	if err != nil {
		log.Println("[warn] failed to retrieve Stat for", ev.Name)
	}

	if stat.IsDir() {
		w.onNewDirectory(ctx, c, ev.Name)
	} else {
		w.onNewFile(ctx, c, ev.Name)
	}
}

func (w *Watcher) onNewDirectory(ctx context.Context, c *Circumstances, path string) {
	parent := filepath.Dir(path)
	if target, ok := w.watchingDir[parent]; ok {
		configLogs := make([]*ConfigLogfile, 0, len(target.ConfigLogs))
		for _, c := range target.ConfigLogs {
			if c.Recursive {
				configLogs = append(configLogs, c)
			}
		}
		if len(configLogs) > 0 {
			foundDir, foundFile, err := w.startWatchAndTail(ctx, c, path, configLogs)
			if err != nil {
				log.Println("[warn] something wrong at startWatchAndTail.", err)
				return
			}
			for path, target := range foundDir {
				w.watchingDir[path] = target
			}
			for name, target := range foundFile {
				w.watchingFile[name] = target
				w.reverseMap[target.Name] = name
			}
		}
	} else {
		log.Println("[warn] watchingDir may be corrupted.")
	}
}

func (w *Watcher) onNewFile(ctx context.Context, c *Circumstances, path string) {
	parent := filepath.Dir(path)
	if target, ok := w.watchingDir[parent]; ok {
		foundFile := make(map[string]*TargetFile)
		for _, config := range target.ConfigLogs {
			findFile(path, config, foundFile)
		}
		for name, target := range foundFile {
			if current, ok := w.watchingFile[name]; ok {
				if !target.Timestamp.After(current.Timestamp) {
					continue
				}
				w.unwatchFile(name)
			}
			w.watchingFile[name] = target
			w.reverseMap[target.Name] = name
			w.runTail(ctx, c, target)
		}
	} else {
		log.Println("[warn] watchingDir may be corrupted.")
	}
}

func (w *Watcher) onModify(ev fsnotify.Event) {
	stat, err := os.Stat(ev.Name)
	if err != nil {
		log.Println("[warn] failed to retrieve Stat for", ev.Name)
	}

	if !stat.IsDir() {
		if name, ok := w.reverseMap[ev.Name]; ok {
			if target, ok := w.watchingFile[name]; ok {
				target.EventCh <- ev
			} else {
				log.Println("[warn] reverseMap may be corrupted.")
			}
		}
	}
}

func (w *Watcher) onDeleteOrRename(ev fsnotify.Event) {
	if _, ok := w.watchingDir[ev.Name]; ok {
		w.unwatchDir(ev.Name)
	} else if name, ok := w.reverseMap[ev.Name]; ok {
		w.unwatchFile(name)
	}
}

func (w *Watcher) unwatchDir(path string) {
	for name := range w.watchingFile {
		if strings.HasPrefix(name, path+"/") {
			w.unwatchFile(name)
		}
	}
	for dir := range w.watchingDir {
		if strings.HasPrefix(dir, path+"/") {
			w.unwatchDir(dir)
		}
	}
	delete(w.watchingDir, path)
	w.watcher.Remove(path)
	log.Printf("[info] Unwatching Dir: path => %v\n", path)
}

func (w *Watcher) unwatchFile(name string) {
	if target, ok := w.watchingFile[name]; ok {
		target.Cancel()
		delete(w.reverseMap, target.Name)
		delete(w.watchingFile, name)
	} else {
		log.Println("[warn] reverseMap may be corrupted.")
	}
}

func (w *Watcher) initialize(ctx context.Context, c *Circumstances) error {
	defer c.StartProcess.Done()
	defer func() { w.initialized = true }()

	var err error
	w.watchingDir, w.watchingFile, err = w.startWatchAndTail(ctx, c, "", w.configLogs)
	if err != nil {
		return err
	}
	for name, target := range w.watchingFile {
		w.reverseMap[target.Name] = name
	}

	return nil
}

func (w *Watcher) startWatchAndTail(ctx context.Context, c *Circumstances, basedir string, configLogs []*ConfigLogfile) (map[string]*TargetDir, map[string]*TargetFile, error) {
	founDir := make(map[string]*TargetDir)
	foundFile := make(map[string]*TargetFile)

	// search all target
	for _, config := range configLogs {
		dir := basedir
		if basedir == "" {
			dir = config.Basedir
		}
		log.Println("[info] search: ", dir)
		if err := findWatchTargets(dir, config, founDir, foundFile); err != nil {
			return nil, nil, err
		}
	}

	// start in_tail
	for _, target := range foundFile {
		w.runTail(ctx, c, target)
	}

	// start watch
	for path, target := range founDir {
		log.Printf("[info] Watching Dir: path => %v, config => %v\n", path, target.ConfigLogs)
		w.watcher.Add(path)
	}

	return founDir, foundFile, nil
}

func (w *Watcher) runTail(ctx context.Context, c *Circumstances, target *TargetFile) {
	eventCh := make(chan fsnotify.Event)
	position := SEEK_HEAD
	if !w.initialized {
		position = SEEK_TAIL
	}
	tail, err := NewInTail(target.Name, target.ConfigLogfile, eventCh, position)
	if err != nil {
		close(eventCh)
		log.Println("[error]", err)
	} else {
		childCtx, cancel := context.WithCancel(ctx)
		target.Cancel = cancel
		target.EventCh = eventCh
		c.RunProcess(childCtx, tail, true)
	}
}

func findWatchTargets(basedir string, config *ConfigLogfile, foundDir map[string]*TargetDir, foundFile map[string]*TargetFile) error {
	walkDir := func(path string, info os.FileInfo, err error) error {
		log.Println("[debug] walking...: ", path)
		if err != nil {
			log.Println("[warn] walkDir gets err.", err)
			return err
		}

		if info.IsDir() {
			if !config.Recursive && path != basedir {
				return filepath.SkipDir
			}
			findDir(path, config, foundDir)
		} else {
			if err := findFile(path, config, foundFile); err != nil {
				return err
			}
		}

		return nil
	}

	if err := filepath.Walk(basedir, walkDir); err != nil {
		return err
	}
	return nil
}

func findFile(path string, config *ConfigLogfile, foundFile map[string]*TargetFile) error {
	ret := config.TargetFileRegexp.FindStringSubmatchIndex(path)
	if len(ret) > 3 {
		dateStr := path[ret[2]:ret[3]]
		date, err := time.Parse(config.FileTimeFormat, dateStr)
		if err != nil {
			log.Println("[warn] FileTimeFormat and/or TargetFileRegexp is invalid.", err)
			return nil
		}
		baseName := path[ret[0]:ret[2]] + path[ret[3]:] + ":" + config.FileTimeFormat

		current, ok := foundFile[baseName]
		if ok {
			if current.Timestamp.After(date) {
				path = current.Name
			}
		}
		foundFile[baseName] = &TargetFile{
			Name:          path,
			Timestamp:     date,
			ConfigLogfile: config,
		}
	}
	return nil
}

func findDir(path string, config *ConfigLogfile, foundDir map[string]*TargetDir) {
	target, ok := foundDir[path]
	if ok {
		for _, c := range target.ConfigLogs {
			if c == config {
				return
			}
		}
		target.ConfigLogs = append(target.ConfigLogs, config)
	} else {
		newTarget := &TargetDir{
			Name:       path,
			ConfigLogs: []*ConfigLogfile{config},
		}
		foundDir[path] = newTarget
	}
}
