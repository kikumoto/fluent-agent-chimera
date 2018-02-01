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
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"testing"
	"time"

	chimera "github.com/kikumoto/fluent-agent-chimera"
	pdebug "github.com/lestrrat/go-pdebug"
	"github.com/mattn/go-scan"
	"github.com/stretchr/testify/assert"
)

func TestMonitorServer(t *testing.T) {
	if pdebug.Enabled {
		g := pdebug.Marker("TestMonitorServer")
		defer g.End()
	}

	config := &chimera.Config{
		Monitor: &chimera.ConfigMonitor{
			Host: "localhost",
			Port: 24225,
		},
	}
	c, ctx := chimera.NewCircumstances()
	monitor, err := chimera.NewMonitor(config)
	if !assert.NoError(t, err, "chimera.NewMonitor should succeed") {
		return
	}
	c.RunProcess(ctx, monitor, false)

	expectedSents := make(map[string]int64)
	tags := []string{"foo", "bar", "dummy.test"}
	for _, tag := range tags {
		n := rand.Intn(200)
		for i := 0; i < n; i++ {
			c.MonitorCh <- &chimera.SentStat{
				Tag:   tag,
				Sents: 1,
			}
			expectedSents[tag] += 1
		}
	}
	time.Sleep(1 * time.Second)

	resp, err := http.Get(fmt.Sprintf("http://%s/", monitor.Addr))
	if !assert.NoError(t, err, "Monitor data should be got.") {
		return
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !assert.Equal(t, "application/json", ct, "invalid content-type", ct) {
		return
	}
	body, _ := ioutil.ReadAll(resp.Body)
	if pdebug.Enabled {
		pdebug.Printf("%s\n", body)
	}
	js := bytes.NewReader(body)
	for tag, n := range expectedSents {
		js.Seek(int64(0), os.SEEK_SET)
		var got int64
		scan.ScanJSON(js, "/sent/"+tag+"/sents", &got)
		if !assert.Equal(t, n, got, "/sent/%s/messages got %d expected %d", tag, got, n) {
			return
		}
	}
}
