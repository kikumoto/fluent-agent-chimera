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
	"log"
	"os"
	"regexp"

	"github.com/BurntSushi/toml"
)

const (
	DefaultNetwork       = "tcp"
	DefaultAddress       = "127.0.0.1:24224"
	DefaultFieldName     = "message"
	DefaultPathFieldName = "path"
	DefaultHostFieldName = "host"
	DefaultLogLevel      = "info"
)

type Config struct {
	TagPrefix      string
	FieldName      string
	PathFieldName  string
	HostFieldName  string
	Host           string
	ReadBufferSize int
	SubSecondTime  bool
	Server         *ConfigServer
	Logs           []*ConfigLogfile
	Monitor        *ConfigMonitor
	LogLevel       string
}

type ConfigServer struct {
	Network string
	Address string
}

type ConfigLogfile struct {
	Tag              string
	Basedir          string
	Recursive        bool
	TargetFileRegexp *Regexp
	FileTimeFormat   string
	FieldName        string
	PathFieldName    string
	HostFieldName    string
	Host             string
}

type ConfigMonitor struct {
	Host string
	Port int
}

type Regexp struct {
	*regexp.Regexp
}

func ReadConfig(filename string) (*Config, error) {
	var config Config
	log.Println("[info] Loading config file:", filename)
	if _, err := toml.DecodeFile(filename, &config); err != nil {
		return nil, err
	}
	config.Restrict()
	return &config, nil
}

func (cs *ConfigServer) Restrict(c *Config) {
	if cs.Network == "" {
		cs.Network = DefaultNetwork
	}
	if cs.Address == "" {
		cs.Address = DefaultAddress
	}
}

func (cl *ConfigLogfile) Restrict(c *Config) {
	if cl.FieldName == "" {
		cl.FieldName = c.FieldName
	}
	if cl.PathFieldName == "" {
		cl.PathFieldName = c.PathFieldName
	}
	if cl.HostFieldName == "" {
		cl.HostFieldName = c.HostFieldName
	}
	if cl.Host == "" {
		cl.Host = c.Host
	}
	if c.TagPrefix != "" {
		cl.Tag = c.TagPrefix + "." + cl.Tag
	}
}

func (cr *ConfigMonitor) Restrict(c *Config) {
	if cr.Port == 0 {
		cr.Port = DefaultMonitorPort
	}
	if cr.Host == "" {
		cr.Host = DefaultMonitorHost
	}
}

func (c *Config) Restrict() {
	if c.FieldName == "" {
		c.FieldName = DefaultFieldName
	}
	if c.PathFieldName == "" {
		c.PathFieldName = DefaultPathFieldName
	}
	if c.HostFieldName == "" {
		c.HostFieldName = DefaultHostFieldName
	}
	if c.Host == "" {
		c.Host, _ = os.Hostname()
	}
	if c.Server != nil {
		c.Server.Restrict(c)
	}
	for _, subconf := range c.Logs {
		subconf.Restrict(c)
	}
	if c.Monitor != nil {
		c.Monitor.Restrict(c)
	}
	if c.LogLevel == "" {
		c.LogLevel = DefaultLogLevel
	}
}

func (r *Regexp) UnmarshalText(text []byte) error {
	var err error
	s := string(text)
	r.Regexp, err = regexp.Compile(s)
	return err
}
