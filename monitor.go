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

package chimera

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	stats_api "github.com/fukata/golang-stats-api-handler"
)

const (
	DefaultMonitorPort = 24223
	DefaultMonitorHost = "localhost"
)

type Stats struct {
	Sent   map[string]*SentStat `json:"sent"`
	Files  map[string]*FileStat `json:"files"`
	Server *ServerStat          `json:"server"`
	mu     sync.Mutex
}

type Stat interface {
	ApplyTo(*Stats)
}

type ServerStat struct {
	Alive bool   `json:"alive"`
	Error string `json:"error"`
}

type SentStat struct {
	Tag   string `json:"-"`
	Sents int64  `json:"sents"`
}

type FileStat struct {
	Tag      string `json:"tag"`
	File     string `json:"-"`
	Position int64  `json:"position"`
	Error    string `json:"error"`
	Close    bool   `json:"-"`
}

func (s *FileStat) ApplyTo(ss *Stats) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if s.Close {
		delete(ss.Files, s.File)
	} else {
		ss.Files[s.File] = s
	}
}

func (s *ServerStat) ApplyTo(ss *Stats) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.Server = s
}

func (s *SentStat) ApplyTo(ss *Stats) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if _s, ok := ss.Sent[s.Tag]; ok {
		_s.Sents += s.Sents
	} else {
		ss.Sent[s.Tag] = s
	}
}

func (ss *Stats) WriteJSON(w http.ResponseWriter, v interface{}) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	encoder := json.NewEncoder(w)
	encoder.Encode(v)
}

func (ss *Stats) Run(ch chan Stat) {
	for {
		s := <-ch
		s.ApplyTo(ss)
	}
}

type Monitor struct {
	stats     *Stats
	address   string
	Addr      net.Addr
	listener  net.Listener
	monitorCh chan Stat
}

func NewMonitor(config *Config) (*Monitor, error) {
	stats := &Stats{
		Sent:   make(map[string]*SentStat),
		Files:  make(map[string]*FileStat),
		Server: &ServerStat{},
	}
	monitor := &Monitor{
		stats: stats,
	}
	if config.Monitor == nil {
		return monitor, nil
	}
	monitorAddress := fmt.Sprintf("%s:%d", config.Monitor.Host, config.Monitor.Port)
	listener, err := net.Listen("tcp", monitorAddress)
	if err != nil {
		log.Println("[error]", err)
		return nil, err
	}
	monitor.listener = listener
	monitor.Addr = listener.Addr()
	return monitor, nil
}

func (m *Monitor) Run(ctx context.Context, c *Circumstances) {
	c.OutputProcess.Add(1)
	defer c.OutputProcess.Done()
	go m.stats.Run(c.MonitorCh)

	c.StartProcess.Done()

	if m.listener == nil {
		return
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		m.stats.WriteJSON(w, m.stats)
	})
	http.HandleFunc("/sent", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		m.stats.WriteJSON(w, m.stats.Sent)
	})
	http.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		m.stats.WriteJSON(w, m.stats.Files)
	})
	http.HandleFunc("/server", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		m.stats.WriteJSON(w, m.stats.Server)
	})
	http.HandleFunc("/system", stats_api.Handler)

	go http.Serve(m.listener, nil)
	log.Printf("[info] Monitor server listening http://%s/\n", m.listener.Addr())
}

func monitorError(err error) string {
	return fmt.Sprintf("[%s] %s", time.Now(), err)
}
