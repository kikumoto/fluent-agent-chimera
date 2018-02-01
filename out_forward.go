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
	"fmt"
	"log"
	"time"

	fluent "github.com/lestrrat/go-fluent-client"
)

type OutForward struct {
	logger         fluent.Client
	messageCh      chan *FluentMessage
	monitorCh      chan Stat
	lastPostStatus bool
	lastError      error
	lastErrorAt    time.Time
}

const (
	serverHealthCheckInterval = 3 * time.Second
)

// OutForward ... recieve FluentMessage from channel, and send it to passed fluentd until success.
func NewOutForward(s *ConfigServer, subsecond bool) (*OutForward, error) {
	logger, err := fluent.New(
		fluent.WithBuffered(false),
		fluent.WithNetwork(s.Network),
		fluent.WithAddress(s.Address),
		fluent.WithSubsecond(subsecond),
	)
	if err != nil {
		log.Println("[warn]", err)
	} else {
		log.Println("[info] Network:", s.Network, ",Server:", s.Address, "connected")
	}
	return &OutForward{
		logger: logger,
	}, nil
}

func (f *OutForward) Run(ctx context.Context, c *Circumstances) {
	log.Println("[info] out_forward: starting")
	defer log.Println("[info] out_forward: exiting")

	c.OutputProcess.Add(1)
	defer c.OutputProcess.Done()
	f.messageCh = c.MessageCh
	f.monitorCh = c.MonitorCh

	c.StartProcess.Done()

	go f.checkServerHealth()

	for {
		err := f.outForwardRecieve(ctx)
		if err != nil {
			if _, ok := err.(Signal); ok {
				log.Println("[info]", err)
				return
			} else {
				log.Println("[error]", err)
			}
		}
	}
}

func (f *OutForward) outForwardRecieve(ctx context.Context) error {
	var message *FluentMessage
	var ok, shutdown bool

	select {
	case message, ok = <-f.messageCh:
		shutdown = !ok
	}
	if shutdown {
		log.Println("[info] out_forward: message channel closed")
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		if err := f.logger.Shutdown(timeoutCtx); err != nil {
			log.Println("[warn] Failed to shutdown go-fluent-client properly. force-close it")
			f.logger.Close()
		}
		return Signal{"shutdown out_forward"}
	}

	v := map[string]interface{}{
		message.FieldName:     message.Message,
		message.PathFieldName: message.Path,
		message.HostFieldName: message.Host,
	}

	for {
		err := f.logger.Post(
			message.Tag,
			v,
			fluent.WithTimestamp(message.Timestamp),
		)
		if err != nil {
			log.Println("[warn] failed to send message. retrying... :", err)
			f.lastPostStatus = false
			f.recordError(err)
			f.logger.Close()
		} else {
			f.lastPostStatus = true
			f.monitorCh <- &SentStat{
				Tag:   message.Tag,
				Sents: 1,
			}
			break
		}
		time.Sleep(1 * time.Second)
	}

	return nil
}

func (f *OutForward) checkServerHealth() {
	c := time.Tick(serverHealthCheckInterval)
	for _ = range c {
		f.monitorCh <- &ServerStat{
			Alive: f.lastPostStatus,
			Error: f.lastErrorString(),
		}
	}
}

func (f *OutForward) recordError(err error) {
	f.lastErrorAt = time.Now()
	f.lastError = err
}

func (f *OutForward) lastErrorString() string {
	if f.lastError != nil {
		return fmt.Sprintf("[%s] %s", f.lastErrorAt, f.lastError)
	} else {
		return ""
	}
}
