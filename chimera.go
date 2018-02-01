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
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	MessageChannelBufferLen = 1
	MonitorChannelBufferLen = 256
)

var (
	LineSeparator = []byte{'\n'}
)

type Process interface {
	Run(context.Context, *Circumstances)
}

type Signal struct {
	message string
}

func (s Signal) Error() string {
	return s.message
}

func (s Signal) String() string {
	return s.message
}

func (s Signal) Signal() {
}

func NewSignal(message string) Signal {
	return Signal{message}
}

type FluentMessage struct {
	Tag           string
	Timestamp     time.Time
	FieldName     string
	Message       []byte
	PathFieldName string
	Path          string
	HostFieldName string
	Host          string
}

type Circumstances struct {
	processCancel func()
	MessageCh     chan *FluentMessage
	MonitorCh     chan Stat
	InputProcess  sync.WaitGroup
	OutputProcess sync.WaitGroup
	StartProcess  sync.WaitGroup
}

func NewCircumstances() (*Circumstances, context.Context) {
	ctx, cancel := context.WithCancel(context.Background())
	return &Circumstances{
		processCancel: cancel,
		MessageCh:     make(chan *FluentMessage, MessageChannelBufferLen),
		MonitorCh:     make(chan Stat, MonitorChannelBufferLen),
	}, ctx
}

func (c *Circumstances) RunProcess(ctx context.Context, p Process, nowait bool) {
	if !nowait {
		c.StartProcess.Add(1)
	}
	go p.Run(ctx, c)
}

func Run(config *Config) *Circumstances {
	c, ctx := NewCircumstances()

	if config.ReadBufferSize > 0 {
		ReadBufferSize = config.ReadBufferSize
		log.Println("[info] set ReadBufferSize", ReadBufferSize)
	}

	// start monitor server
	monitor, err := NewMonitor(config)
	if err != nil {
		log.Println("[error] Couldn't start monitor server.", err)
	} else {
		c.RunProcess(ctx, monitor, false)
	}

	// start out_forward
	outForward, err := NewOutForward(config.Server, config.SubSecondTime)
	if err != nil {
		log.Println("[error]", err)
	} else {
		c.RunProcess(ctx, outForward, false)
	}

	// start watcher
	if len(config.Logs) > 0 {
		watcher, err := NewWatcher(config.Logs)
		if err != nil {
			log.Println("[error]", err)
		}
		c.RunProcess(ctx, watcher, false)
	}

	c.StartProcess.Wait()
	return c
}

func (c *Circumstances) Shutdown() {
	c.processCancel()
	c.InputProcess.Wait()
	close(c.MessageCh)
	c.OutputProcess.Wait()
}

func Rel2Abs(filename string) (string, error) {
	if filepath.IsAbs(filename) {
		return filename, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		log.Println("[error] Couldn't get current working dir.", err)
		return "", err
	}
	return filepath.Join(cwd, filename), nil
}
