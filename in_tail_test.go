/*
This file was imported from https://github.com/fujiwara/fluent-agent-hydra and modified.

Original License:

Copyright 2014 Fujiwara Shunichiro / KAYAC Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package chimera_test

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	chimera "github.com/kikumoto/fluent-agent-chimera"
	pdebug "github.com/lestrrat/go-pdebug"
	"github.com/stretchr/testify/assert"
)

var (
	EOFMarker      = "__EOF__"
	RotateMarker   = "__ROTATE__"
	TruncateMarker = "__TRUNCATE__"
	Logs           = []string{
		"single line\n",
		"multi line 1\nmulti line 2\nmultiline 3\n",
		"continuous line 1",
		"continuous line 2\n",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n",   // 80 bytes == hydra.ReadBufferSize for testing
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n",  // 81 bytes
		"ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc\n", // 82byte
		"dddddddddddddddddddddddddddddddddddddddd",
		"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee\n", // continuous line 80 bytes
		RotateMarker + "\n",
		"foo\n",
		"bar\n",
		"baz\n",
		TruncateMarker + "\n",
		"FOOOO\n",
		"BAAAR\n",
		"BAZZZZZZZ\n",
		EOFMarker + "\n",
	}
)

const (
	ReadBufferSizeForTest = 80
)

func TestTrail(t *testing.T) {
	if pdebug.Enabled {
		g := pdebug.Marker("TestTrail")
		defer g.End()
	}

	chimera.ReadBufferSize = ReadBufferSizeForTest

	tmpdir, _ := ioutil.TempDir(os.TempDir(), "chimera-test")
	file, _ := os.OpenFile(filepath.Join(tmpdir, "logfile20180101.log"), os.O_CREATE|os.O_WRONLY, 0644)
	defer os.RemoveAll(tmpdir)

	var fileWriterProcess sync.WaitGroup
	fileWriterProcess.Add(1)
	go fileWriter(t, file, Logs, &fileWriterProcess)

	configLogFile := &chimera.ConfigLogfile{
		Tag:              "test",
		Basedir:          tmpdir,
		Recursive:        true,
		TargetFileRegexp: &chimera.Regexp{Regexp: regexp.MustCompile(`^.+/logfile(\d{8})\..*$`)},
		FileTimeFormat:   "20060102",
		FieldName:        "message",
		PathFieldName:    "path",
		HostFieldName:    "host",
		Host:             "hostname",
	}
	conifgLogs := []*chimera.ConfigLogfile{configLogFile}
	c, ctx := chimera.NewCircumstances()
	watcher, err := chimera.NewWatcher(conifgLogs)
	if !assert.NoError(t, err, `chimera.NewWatcher should succeed`) {
		return
	}
	c.RunProcess(ctx, watcher, false)
	c.StartProcess.Wait()

	resultCh := make(chan string)
	go receiver(t, c.MessageCh, resultCh)

	received := <-resultCh
	sent := strings.Join(Logs, "")
	if !assert.Equal(t, sent, received, "received messages should be same as sent ones.") {
		log.Print(sent)
		log.Print(received)
	}

	c.Shutdown()
	fileWriterProcess.Wait()
}

func fileWriter(t *testing.T, file *os.File, logs []string, fileWriterProcess *sync.WaitGroup) {
	defer fileWriterProcess.Done()

	if pdebug.Enabled {
		g := pdebug.Marker("fileWriter")
		defer g.End()
	}

	filename := file.Name()
	dir := filepath.Dir(filename)
	if pdebug.Enabled {
		pdebug.Printf("fileWriter: first filename: %v\n", filename)
	}
	time.Sleep(1 * time.Second) // wait for start Tail...

	for _, line := range logs {
		if strings.Index(line, RotateMarker) != -1 {
			log.Println("fileWriter: rotate file")
			file.Close()
			file, _ = os.OpenFile(filepath.Join(dir, "logfile20180102.log"), os.O_CREATE|os.O_WRONLY, 0644)
			filename = file.Name()
			if pdebug.Enabled {
				pdebug.Printf("fileWriter: second filename: %v\n", filename)
			}
		} else if strings.Index(line, TruncateMarker) != -1 {
			time.Sleep(1 * time.Second)
			log.Println("fileWriter: truncate(file, 0)")
			os.Truncate(filename, 0)
			file.Seek(int64(0), os.SEEK_SET)
		}
		_, err := file.WriteString(line)
		log.Print("fileWriter: wrote ", line)
		if err != nil {
			log.Println("write failed", err)
		}
		time.Sleep(1 * time.Millisecond)
	}
	file.Close()
}

func receiver(t *testing.T, ch chan *chimera.FluentMessage, resultCh chan string) {
	defer close(resultCh)
	receive := ""
	filename := "logfile20180101.log"
	for {
		message := <-ch
		assert.Equal(t, "test", message.Tag, "invalid Tag.")
		assert.Equal(t, "path", message.PathFieldName, "invalid PathFieldName.")
		assert.Equal(t, "host", message.HostFieldName, "invalid HostFieldName.")
		assert.Equal(t, "hostname", message.Host, "invalid Host.")

		if strings.Index(string(message.Message), RotateMarker) != -1 {
			filename = "logfile20180102.log"
		}
		assert.NotEqual(t, -1, strings.Index(message.Path, filename), "message.Path should include "+filename)

		receive = receive + string(message.Message) + string(chimera.LineSeparator)
		if strings.Index(string(message.Message), EOFMarker) != -1 {
			resultCh <- receive
			return
		}
	}
}
