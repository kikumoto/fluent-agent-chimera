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
	"io"
	"log"
	"time"

	fsnotify "github.com/fsnotify/fsnotify"
)

const (
	ActiveTailInterval   = 200 * time.Millisecond
	InactiveTailInterval = 1000 * time.Millisecond
)

type InTail struct {
	filename      string
	tag           string
	fieldName     string
	pathFieldName string
	hostFieldName string
	host          string
	lastReadAt    time.Time
	messageCh     chan *FluentMessage
	monitorCh     chan Stat
	eventCh       chan fsnotify.Event
	position      int64
	tailInterval  time.Duration
}

func NewInTail(path string, config *ConfigLogfile, eventCh chan fsnotify.Event, position int64) (*InTail, error) {
	filename, err := Rel2Abs(path)
	if err != nil {
		return nil, err
	}
	return &InTail{
		filename:      filename,
		tag:           config.Tag,
		fieldName:     config.FieldName,
		pathFieldName: config.PathFieldName,
		hostFieldName: config.HostFieldName,
		host:          config.Host,
		lastReadAt:    time.Now(),
		eventCh:       eventCh,
		position:      position,
		tailInterval:  InactiveTailInterval,
	}, nil
}

func (t *InTail) Run(ctx context.Context, c *Circumstances) {
	c.InputProcess.Add(1)
	defer c.InputProcess.Done()
	defer close(t.eventCh)

	t.messageCh = c.MessageCh
	t.monitorCh = c.MonitorCh

	log.Println("[debug] Trying trail file", t.filename)
	f, err := t.newTrailFile(t.position, ctx)
	if err != nil {
		if _, ok := err.(Signal); ok {
			log.Println("[info]", err)
		} else {
			log.Println("[error]", err)
		}
		return
	}
	for {
		err := t.watchFileEvent(f, ctx)
		if err != nil {
			if _, ok := err.(Signal); ok {
				log.Println("[info]", err)
			} else {
				log.Println("[warning]", err)
			}
			return
		}
	}
}

func (t *InTail) newTrailFile(startPos int64, ctx context.Context) (*File, error) {
	seekTo := startPos
	first := true
	for {
		f, err := openFile(t.filename, seekTo)
		if err == nil {
			f.Tag = t.tag
			f.FieldName = t.fieldName
			f.PathFieldName = t.pathFieldName
			f.HostFieldName = t.hostFieldName
			f.Host = t.host
			log.Println("[info] Trailing file:", f.Path, "tag:", f.Tag)
			t.monitorCh <- f.UpdateStat()
			return f, nil
		}
		t.monitorCh <- &FileStat{
			Tag:      t.tag,
			File:     t.filename,
			Position: int64(-1),
			Error:    monitorError(err),
		}
		if first {
			log.Println("[warn]", err, "Retrying...")
		}
		first = false
		seekTo = SEEK_HEAD
		select {
		case <-ctx.Done():
			return nil, t.shutdownSignal()
		case <-time.NewTimer(OpenRetryInterval).C:
		}
	}
}

func (t *InTail) shutdownSignal() Signal {
	t.monitorCh <- &FileStat{
		File:  t.filename,
		Close: true,
	}
	return Signal{"Shutdown trail file: " + t.filename}
}

func (t *InTail) watchFileEvent(f *File, ctx context.Context) error {
	tm := time.NewTimer(t.tailInterval)
	select {
	case <-ctx.Done():
		tm.Stop()
		f.tailAndSend(t.messageCh, t.monitorCh)
		f.Close()
		return t.shutdownSignal()
	case ev := <-t.eventCh:
		tm.Stop()
		t.tailInterval = ActiveTailInterval
		log.Println("[debug] in_tail receives event", ev)
		break
	case <-tm.C:
	}

	err := f.restrict()
	if err != nil {
		return err
	}
	if time.Now().Before(t.lastReadAt.Add(t.tailInterval)) {
		return nil
	}
	err = f.tailAndSend(t.messageCh, t.monitorCh)
	t.lastReadAt = time.Now()
	t.tailInterval = InactiveTailInterval

	if err != io.EOF {
		log.Println("[error] tailAndSend error: ", err)
		return err
	}
	return nil
}
