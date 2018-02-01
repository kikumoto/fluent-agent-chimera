# fluent-agent-chimera

A Fluent log agent.

[![Build status](https://travis-ci.org/kikumoto/fluent-agent-chimera.svg?branch=master)](https://travis-ci.org/kikumoto/fluent-agent-chimera)

This agent modified [fluent-agent-hydra](https://github.com/fujiwara/fluent-agent-hydra).

## Features

- Tailing log files (like in_tail)
    * enable to handle multiple files which is matched by regexp with dateformat pattern in a directory.
    * enable to handle rotating file.
    * if new directory is created ant new file in the directory is created, that is trailed automatically.
- Forwarding messages to external fluentd（like out_forward）
    * A fluentd server can be used. So you may use with a fluentd server or a fluent-agent-hydra in localhost.
    * enable to use unix domain socket because this agent uses [go-fluent-client](https://github.com/lestrrat/go-fluent-client).
- Stats monitor httpd server
    * serve an agent stats by JSON format.
- Supports sub-second time

## Installation

[Binary releases](https://github.com/kikumoto/fluent-agent-chimera/releases)

or

```
go get github.com/kikumoto/fluent-agent-chimera/cmd/fluent-agent-chimera/
```

## Usage

```
fluent-agent-chimera -c /path/to/config.toml
```

An example of config.toml.

```toml
# global settings
TagPrefix = "nginx"              # "nginx.access", "nginx.error"
FieldName = "message"            # default "message"
ReadBufferSize = 1048576         # default 64KB.
SubSecondTime = true             # default false. for Fluentd 0.14 or later only
# FilenameFieldName = "filepath" # default filepath
# HostFieldName = "hostname"     # default hostname
# HostFieldValue = "xxxx"        # default values got from "hostname" command
# LogLevel = "warn"                #default info

[Server]
# fluentd server info
#   chimera uses https://github.com/lestrrat/go-fluent-client.
Network = "tcp"                  # "unix" for unix domain socket
Address = "127.0.0.1:24224"      # filename when Network = "unix"

[[Logs]]
Tag = "batch"
Basedir = "/path/to/batchdir"
Recursive = true                 # default false

# Specify a regular expression to match a file targeted for monitoring. 
# The regular expression must have only one group to match on the date and time.
TargetFileRegexp = "^.+/sample_dir/.*(\\d{8})(?:.*\\.log)?$"
FileTimeFormat = "20060102"

[[Logs]]
Tag = "test"
Basedir = "/another/dir/demo_dir"
Recursive = false
TargetFileRegexp = "^.+/sample_dir/.*(\\d{4}-\\d{2}-\\d{2})(?:.*\\.log)?$"
FileTimeFormat = "2006-01-02"

[Monitor]
Host = "localhost"
Port = 24223
```

## Stats monitor

### Chimera application stats

`curl -s [Monitor.Host]:[Monitor.Port]/ | jq .`

An example of response.

```json
$ curl -s http://localhost:24223/ | jq .
{
  "sent": {
    "test": {
      "sents": 1
    }
  },
  "files": {
    "/path/to/batchdir/sample_dir/batch/hoge_20180122.log": {
      "tag": "test",
      "position": 32,
      "error": ""
    },
    "/path/to/batchdir/sample_dir/job/sample_20180124.log": {
      "tag": "test",
      "position": 4,
      "error": ""
    }
  },
  "server": {
    "alive": false,
    "error": ""
  }
}
```

You can retrieve data respectively, like following

`curl -s [Monitor.Host]:[Monitor.Port]/sent | jq .`

`curl -s [Monitor.Host]:[Monitor.Port]/files | jq .`

`curl -s [Monitor.Host]:[Monitor.Port]/server | jq .`

.


### system stats

`curl -s [Monitor.Host]:[Monitor.Port]/system | jq .`

An example of response.

```json
{
  "time": 1516702899684754000,
  "go_version": "go1.9.2",
  "go_os": "darwin",
  "go_arch": "amd64",
  "cpu_num": 4,
  "goroutine_num": 15,
  "gomaxprocs": 4,
  "cgo_call_num": 5,
  "memory_alloc": 2707952,
  "memory_total_alloc": 2707952,
  "memory_sys": 8034552,
  "memory_lookups": 29,
  "memory_mallocs": 6417,
  "memory_frees": 166,
  "memory_stack": 557056,
  "heap_alloc": 2707952,
  "heap_sys": 4685824,
  "heap_idle": 917504,
  "heap_inuse": 3768320,
  "heap_released": 0,
  "heap_objects": 6251,
  "gc_next": 4473924,
  "gc_last": 0,
  "gc_num": 0,
  "gc_per_second": 0,
  "gc_pause_per_second": 0,
  "gc_pause": []
}
```
