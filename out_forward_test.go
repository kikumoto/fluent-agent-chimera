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
	"context"
	"testing"
	"time"

	chimera "github.com/kikumoto/fluent-agent-chimera"
	pdebug "github.com/lestrrat/go-pdebug"
	"github.com/stretchr/testify/assert"
)

var (
	TestTag           = "test"
	TestFieldName     = "message"
	TestMessageLines  = []string{"message1", "message2", "message3"}
	TestPathFieldName = "path"
	TestPath          = "/test/xxx20180106.log"
	TestHostFieldName = "host"
	TestHost          = "server.sample.com"
)

func prepareMessages() []*chimera.FluentMessage {
	messages := make([]*chimera.FluentMessage, len(TestMessageLines))
	for i, msg := range TestMessageLines {
		messages[i] = &chimera.FluentMessage{
			Tag:           TestTag,
			Timestamp:     time.Now(),
			FieldName:     TestFieldName,
			Message:       []byte(msg),
			PathFieldName: TestPathFieldName,
			Path:          TestPath,
			HostFieldName: TestHostFieldName,
			Host:          TestHost,
		}
	}
	return messages
}

func newConfigServer(s *server) *chimera.ConfigServer {
	return &chimera.ConfigServer{
		Network: s.Network,
		Address: s.Address,
	}
}

func TestForwardMessages(t *testing.T) {
	if pdebug.Enabled {
		g := pdebug.Marker("TestForwardSingle")
		defer g.End()
	}

	s, err := newServer(false)
	if !assert.NoError(t, err, "newServer should succeed") {
		return
	}
	defer s.Close()

	// This is just to stop the server
	sctx, scancel := context.WithCancel(context.Background())
	defer scancel()

	go s.Run(sctx)
	<-s.Ready()

	c, ctx := chimera.NewCircumstances()
	outForward, err := chimera.NewOutForward(newConfigServer(s), true)
	if err != nil {
		t.Error(err)
	}
	c.RunProcess(ctx, outForward, false)

	messages := prepareMessages()
	for _, msg := range messages {
		c.MessageCh <- msg
	}
	time.Sleep(time.Duration(1) * time.Second)

	c.Shutdown()
	time.Sleep(time.Duration(1) * time.Second)

	if !assert.Equal(t, len(TestMessageLines), len(s.Payload), "sent message counts should be equal received ones.") {
		return
	}
	for i, msg := range s.Payload {
		if !assert.Equal(t, "test", msg.Tag, "Tag should be 'test'.") {
			return
		}

		r, ok := msg.Record.(map[string]interface{})
		if !assert.True(t, ok) {
			return
		}

		path, ok := r["path"].(string)
		if !assert.True(t, ok) {
			return
		}
		if !assert.Equal(t, TestPath, path) {
			return
		}

		host, ok := r["host"].(string)
		if !assert.True(t, ok) {
			return
		}
		if !assert.Equal(t, TestHost, host) {
			return
		}

		m, ok := r["message"].([]byte)
		if !assert.True(t, ok) {
			return
		}
		if !assert.Equal(t, TestMessageLines[i], string(m)) {
			return
		}
	}
}
