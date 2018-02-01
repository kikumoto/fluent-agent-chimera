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

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/hashicorp/logutils"
	"github.com/kikumoto/fluent-agent-chimera"
)

var (
	trapSignals = []os.Signal{
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT}
)

func main() {
	var (
		configFile  string
		help        bool
		showVersion bool
	)
	flag.StringVar(&configFile, "c", "", "configuration file path")
	flag.BoolVar(&help, "h", false, "show help message")
	flag.BoolVar(&help, "help", false, "show help message")
	flag.BoolVar(&showVersion, "v", false, "show version")
	flag.BoolVar(&showVersion, "version", false, "show version")
	flag.Parse()

	if showVersion {
		_showVersion()
	}
	if help {
		usage()
	}
	if pprofile := os.Getenv("PPROF"); pprofile != "" {
		f, err := os.Create(pprofile)
		if err != nil {
			log.Fatal("[error] Can't create profiling stat file.", err)
		}
		log.Println("[info] StartCPUProfile() stat file", f.Name())
		pprof.StartCPUProfile(f)
	}

	var (
		config *chimera.Config
		err    error
	)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, trapSignals...)
	if configFile != "" {
		config, err = chimera.ReadConfig(configFile)
		if err != nil {
			log.Println("[error] Can't load config", err)
			os.Exit(2)
		}
	} else {
		usage()
	}

	filter := &logutils.LevelFilter{
		Levels: []logutils.LogLevel{"debug", "info", "warn", "error"},
		MinLevel: logutils.LogLevel(config.LogLevel),
		Writer: os.Stdout,
	}
	log.SetOutput(filter)

	circumstances := chimera.Run(config)
	go func() {
		circumstances.InputProcess.Wait()
		sigCh <- chimera.NewSignal("all input processes terminated")
	}()

	// waiting for all input processes are terminated or got os signal
	sig := <-sigCh

	log.Println("[info] SIGNAL", sig, "shutting down")
	pprof.StopCPUProfile()

	go func() {
		time.Sleep(3 * time.Second) // at least wait 3 sec
		sig, ok := <-sigCh
		if !ok {
			return // closed
		}
		log.Println("[warn] SIGNAL", sig, "before shutdown completed. aborted")
		os.Exit(1)
	}()

	circumstances.Shutdown()
	os.Exit(0)
}

func usage() {
	fmt.Println("Usage of fluent-agent-chimera")
	fmt.Println("")
	fmt.Println("  fluent-agent-chimera -c config.toml")
	fmt.Println("")
	flag.PrintDefaults()
	os.Exit(1)
}

func _showVersion() {
	fmt.Println("version:", version)
	fmt.Println("revision:", revision)
	fmt.Printf("compiler:%s %s\n", runtime.Compiler, runtime.Version())
	fmt.Println("build:", buildDate)
	os.Exit(0)
}
