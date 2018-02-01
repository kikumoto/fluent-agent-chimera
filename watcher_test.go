package chimera

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	pdebug "github.com/lestrrat/go-pdebug"
	"github.com/stretchr/testify/assert"
)

var (
	TestFiles = []string{
		"recursive/test1/foo_20180101.log.gz",
		"recursive/test1/foo_20180102.log",
		"recursive/test1/foo_20180103.log",
		"recursive/test1/bar20180102.log",
		"recursive/test1/bar_xxxxxx.log",
		"recursive/test1/log-20180102",
		"recursive/test1/test11/foo_20180102.log",
		"recursive/test2/",
		"recursive/test3/baz_20180101.log",
		"recursive/test3/baz_20180102.log",
		"recursive/test3/baz_2018-01-02.log",
		"recursive/test3/test33/hoge_20180102.log",
		"nonrecursive/foo_20180102.log",
		"nonrecursive/test1/bar20180101.log",
	}
)

func prepareFiles(tmpdir string) {
	for _, f := range TestFiles {
		if strings.HasSuffix(f, "/") {
			createDir(tmpdir, f)
		} else {
			createFile(tmpdir, f)
		}
	}
}

func createDir(tmpdir string, d string) {
	fullpath := filepath.Join(tmpdir, d)
	if _, err := os.Stat(fullpath); os.IsNotExist(err) {
		os.MkdirAll(fullpath, 0777)
	}
}

func createFile(tmpdir string, f string) {
	fullpath := filepath.Join(tmpdir, f)
	parent := filepath.Dir(fullpath)
	if _, err := os.Stat(parent); os.IsNotExist(err) {
		os.MkdirAll(parent, 0777)
	}
	ioutil.WriteFile(fullpath, []byte{}, os.ModePerm)
}

func newConfigLogfiles(tmpdir string) []*ConfigLogfile {
	return []*ConfigLogfile{
		&ConfigLogfile{
			Basedir:          filepath.Join(tmpdir, "recursive"),
			Recursive:        true,
			TargetFileRegexp: &Regexp{Regexp: regexp.MustCompile(`^.+/recursive/.*(\d{8})(?:\.log)?$`)},
			FileTimeFormat:   "20060102",
		},
		&ConfigLogfile{
			Basedir:          filepath.Join(tmpdir, "nonrecursive"),
			Recursive:        false,
			TargetFileRegexp: &Regexp{Regexp: regexp.MustCompile(`^.+/nonrecursive/.*(\d{8})(?:\.log)?$`)},
			FileTimeFormat:   "20060102",
		},
	}
}

func TestWatcher(t *testing.T) {
	if pdebug.Enabled {
		g := pdebug.Marker("TestWatcher")
		defer g.End()
	}

	tmpdir, _ := ioutil.TempDir(os.TempDir(), "chimera-test")
	defer os.RemoveAll(tmpdir)
	prepareFiles(tmpdir)

	c, ctx := NewCircumstances()
	watcher, err := NewWatcher(newConfigLogfiles(tmpdir))
	if !assert.NoError(t, err, "Watcher should be created.") {
		return
	}
	c.RunProcess(ctx, watcher, false)
	c.StartProcess.Wait()

	testInitialStatus(t, watcher, tmpdir)
	testCreateFile(t, watcher, tmpdir)
	testCreateDir(t, watcher, tmpdir)
	testDeleteFile(t, watcher, tmpdir)
	testDeleteDir(t, watcher, tmpdir)

	c.Shutdown()
}

func testInitialStatus(t *testing.T, w *Watcher, tmpdir string) {
	if pdebug.Enabled {
		g := pdebug.Marker("testInitialStatus")
		defer g.End()
	}
	expectDir := []string{
		"recursive",
		"recursive/test1",
		"recursive/test1/test11",
		"recursive/test2",
		"recursive/test3",
		"recursive/test3/test33",
		"nonrecursive",
	}
	expectFile := []string{
		"recursive/test1/foo_20180103.log",
		"recursive/test1/bar20180102.log",
		"recursive/test1/log-20180102",
		"recursive/test1/test11/foo_20180102.log",
		"recursive/test3/baz_20180102.log",
		"recursive/test3/test33/hoge_20180102.log",
		"nonrecursive/foo_20180102.log",
	}
	check(t, w, tmpdir, expectDir, expectFile)
}

func testCreateFile(t *testing.T, w *Watcher, tmpdir string) {
	if pdebug.Enabled {
		g := pdebug.Marker("testCreateFile")
		defer g.End()
	}
	createFile(tmpdir, "recursive/test1/bar20180103.log")
	createFile(tmpdir, "recursive/test1/log-20180101")
	createFile(tmpdir, "recursive/test2/test20180103.log")
	createFile(tmpdir, "recursive/test3/test33/hoge_20180103.log")
	createFile(tmpdir, "recursive/test3/test33/hoge_2018-01-04.log")
	createFile(tmpdir, "nonrecursive/foo_20180103.log")
	createFile(tmpdir, "nonrecursive/test1/bar20180103.log")
	time.Sleep(500 * time.Millisecond)

	expectDir := []string{
		"recursive",
		"recursive/test1",
		"recursive/test1/test11",
		"recursive/test2",
		"recursive/test3",
		"recursive/test3/test33",
		"nonrecursive",
	}
	expectFile := []string{
		"recursive/test1/foo_20180103.log",
		"recursive/test1/bar20180103.log",
		"recursive/test1/log-20180102",
		"recursive/test1/test11/foo_20180102.log",
		"recursive/test2/test20180103.log",
		"recursive/test3/baz_20180102.log",
		"recursive/test3/test33/hoge_20180103.log",
		"nonrecursive/foo_20180103.log",
	}
	check(t, w, tmpdir, expectDir, expectFile)
}

func testCreateDir(t *testing.T, w *Watcher, tmpdir string) {
	if pdebug.Enabled {
		g := pdebug.Marker("testCreateDir")
		defer g.End()
	}
	createDir(tmpdir, "recursive/test1/test11/test111/")
	createFile(tmpdir, "recursive/test1/test11/test111/hoge_20170103.log")
	createDir(tmpdir, "nonrecursive/test2/")
	createDir(tmpdir, "nonrecursive/test1/test11/")
	time.Sleep(500 * time.Millisecond)

	expectDir := []string{
		"recursive",
		"recursive/test1",
		"recursive/test1/test11",
		"recursive/test1/test11/test111",
		"recursive/test2",
		"recursive/test3",
		"recursive/test3/test33",
		"nonrecursive",
	}
	expectFile := []string{
		"recursive/test1/foo_20180103.log",
		"recursive/test1/bar20180103.log",
		"recursive/test1/log-20180102",
		"recursive/test1/test11/foo_20180102.log",
		"recursive/test1/test11/test111/hoge_20170103.log",
		"recursive/test2/test20180103.log",
		"recursive/test3/baz_20180102.log",
		"recursive/test3/test33/hoge_20180103.log",
		"nonrecursive/foo_20180103.log",
	}
	check(t, w, tmpdir, expectDir, expectFile)
}

func testDeleteFile(t *testing.T, w *Watcher, tmpdir string) {
	if pdebug.Enabled {
		g := pdebug.Marker("testDeleteFile")
		defer g.End()
	}
	os.Remove(filepath.Join(tmpdir, "recursive/test1/foo_20180102.log"))
	os.Remove(filepath.Join(tmpdir, "recursive/test1/test11/test111/hoge_20170103.log"))
	os.Remove(filepath.Join(tmpdir, "recursive/test2/test20180103.log"))
	os.Remove(filepath.Join(tmpdir, "recursive/test3/baz_20180102.log"))
	createFile(tmpdir, "recursive/test3/baz_20180103.log")
	time.Sleep(500 * time.Millisecond)

	expectDir := []string{
		"recursive",
		"recursive/test1",
		"recursive/test1/test11",
		"recursive/test1/test11/test111",
		"recursive/test2",
		"recursive/test3",
		"recursive/test3/test33",
		"nonrecursive",
	}
	expectFile := []string{
		"recursive/test1/foo_20180103.log",
		"recursive/test1/bar20180103.log",
		"recursive/test1/log-20180102",
		"recursive/test1/test11/foo_20180102.log",
		"recursive/test3/baz_20180103.log",
		"recursive/test3/test33/hoge_20180103.log",
		"nonrecursive/foo_20180103.log",
	}
	check(t, w, tmpdir, expectDir, expectFile)
}
func testDeleteDir(t *testing.T, w *Watcher, tmpdir string) {
	if pdebug.Enabled {
		g := pdebug.Marker("testDeleteDir")
		defer g.End()
	}
	os.RemoveAll(filepath.Join(tmpdir, "recursive/test2"))
	os.RemoveAll(filepath.Join(tmpdir, "recursive/test3"))
	time.Sleep(500 * time.Millisecond)

	expectDir := []string{
		"recursive",
		"recursive/test1",
		"recursive/test1/test11",
		"recursive/test1/test11/test111",
		"nonrecursive",
	}
	expectFile := []string{
		"recursive/test1/foo_20180103.log",
		"recursive/test1/bar20180103.log",
		"recursive/test1/log-20180102",
		"recursive/test1/test11/foo_20180102.log",
		"nonrecursive/foo_20180103.log",
	}
	check(t, w, tmpdir, expectDir, expectFile)
}

func check(t *testing.T, w *Watcher, tmpdir string, expectDir []string, expectFile []string) {
	if !assert.Equal(t, len(expectDir), len(w.watchingDir)) {
		log.Println("=== START:DUMP watchingDir ===")
		for _, d := range w.watchingDir {
			log.Println("  watchingDir: ", d.Name)
		}
		log.Println("=== E N D:DUMP watchingDir ===")
		return
	}
	if !assert.Equal(t, len(expectFile), len(w.watchingFile)) {
		log.Println("=== START:DUMP watchingFile ===")
		for _, f := range w.watchingFile {
			log.Println("  watchingFile: ", f.Name)
		}
		log.Println("=== E N D:DUMP watchingFile ===")
		return
	}
	if !assert.Equal(t, len(expectFile), len(w.reverseMap)) {
		log.Println("=== START:DUMP reverseMap ===")
		for f, name := range w.reverseMap {
			log.Println("  reverseMap: ", f, name)
		}
		log.Println("=== E N D:DUMP reverseMap ===")
		return
	}
	for _, d := range expectDir {
		path := filepath.Join(tmpdir, d)
		_, ok := w.watchingDir[path]
		if !assert.True(t, ok, "watchingDir should include "+path) {
			return
		}
	}
	for _, f := range expectFile {
		path := filepath.Join(tmpdir, f)
		name, ok := w.reverseMap[path]
		if !assert.True(t, ok, "reverseMap should include "+path) {
			return
		}
		_, ok = w.watchingFile[name]
		if !assert.True(t, ok, "watchingFile should include "+name) {
			return
		}
	}
}
