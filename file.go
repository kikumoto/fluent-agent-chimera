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
	"bytes"
	"io"
	"log"
	"os"
	"time"
)

const (
	OpenRetryInterval = 1 * time.Second
	SEEK_TAIL         = int64(-1)
	SEEK_HEAD         = int64(0)
)

var (
	ReadBufferSize = 64 * 1024
)

type File struct {
	*os.File
	Path          string
	Tag           string
	Position      int64
	readBuf       []byte
	contBuf       []byte
	lastStat      os.FileInfo
	FieldName     string
	FileStat      *FileStat
	PathFieldName string
	HostFieldName string
	Host          string
}

func openFile(path string, startPos int64) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	file := &File{
		f,
		path,
		"",
		startPos,
		make([]byte, ReadBufferSize),
		make([]byte, 0),
		stat,
		"",
		&FileStat{},
		"",
		"",
		"",
	}

	if startPos == SEEK_TAIL {
		// seek to end of file
		size := file.lastStat.Size()
		pos, _ := file.Seek(size, os.SEEK_SET)
		file.Position = pos
	} else {
		pos, _ := file.Seek(startPos, os.SEEK_SET)
		file.Position = pos
	}
	log.Println("[debug]", file.Path, "Seeked to", file.Position)
	return file, nil
}

func (f *File) restrict() error {
	var err error
	f.lastStat, err = f.Stat()
	if err != nil {
		log.Println("[error]", f.Path, "stat failed", err)
		return err
	}
	if size := f.lastStat.Size(); size < f.Position {
		pos, _ := f.Seek(int64(0), os.SEEK_SET)
		f.Position = pos
		log.Println("[info]", f.Path, "was truncated. Seeked to", pos)
	}
	return nil
}

func (f *File) tailAndSend(messageCh chan *FluentMessage, monitorCh chan Stat) error {
	for {
		n, err := io.ReadAtLeast(f, f.readBuf, 1)
		if n == 0 || err == io.EOF {
			return err
		} else if err != nil {
			return err
		}
		f.Position += int64(n)
		sendBuf := make([]byte, 0)
		if f.readBuf[n-1] == '\n' {
			// f.readBuf is just terminated by '\n'
			if len(f.contBuf) > 0 {
				sendBuf = append(sendBuf, f.contBuf...)
				f.contBuf = make([]byte, 0)
			}
			sendBuf = append(sendBuf, f.readBuf[0:n-1]...)
		} else {
			blockLen := bytes.LastIndex(f.readBuf[0:n], LineSeparator)
			if blockLen == -1 {
				// whole of f.readBuf is continuous line
				f.contBuf = append(f.contBuf, f.readBuf[0:n]...)
				continue
			} else {
				// bottom line of f.readBuf is continuous line
				if len(f.contBuf) > 0 {
					sendBuf = append(sendBuf, f.contBuf...)
				}
				sendBuf = append(sendBuf, f.readBuf[0:blockLen]...)
				f.contBuf = make([]byte, n-blockLen-1)
				copy(f.contBuf, f.readBuf[blockLen+1:n])
			}
		}

		// send message
		t := time.Now()
		for _, msg := range bytes.Split(sendBuf, LineSeparator) {
			messageCh <- &FluentMessage{
				Message:       msg,
				Tag:           f.Tag,
				Timestamp:     t,
				FieldName:     f.FieldName,
				PathFieldName: f.PathFieldName,
				Path:          f.Path,
				HostFieldName: f.HostFieldName,
				Host:          f.Host,
			}
			monitorCh <- f.UpdateStat()
		}
	}
}

func (f *File) UpdateStat() *FileStat {
	f.FileStat.File = f.Path
	f.FileStat.Position = f.Position
	f.FileStat.Tag = f.Tag
	return f.FileStat
}
