/*
This file was imported from https://github.com/lestrrat/go-fluent-client and modified.

Original License:

MIT License

Copyright (c) 2017 lestrrat

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package chimera_test

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"sync"

	fluent "github.com/lestrrat/go-fluent-client"
	msgpack "github.com/lestrrat/go-msgpack"
	pdebug "github.com/lestrrat/go-pdebug"
	"github.com/pkg/errors"
)

type server struct {
	cleanup  func()
	done     chan struct{}
	listener net.Listener
	ready    chan struct{}
	useJSON  bool
	Network  string
	Address  string
	Payload  []*fluent.Message
}

func newServer(useJSON bool) (*server, error) {
	dir, err := ioutil.TempDir(os.TempDir(), "sock-")
	if err != nil {
		return nil, errors.Wrap(err, `failed to create temporary directory`)
	}

	file := filepath.Join(dir, "test-server.sock")

	l, err := net.Listen("unix", file)
	if err != nil {
		return nil, errors.Wrap(err, `failed to listen to unix socket`)
	}

	s := &server{
		Network:  "unix",
		Address:  file,
		useJSON:  useJSON,
		done:     make(chan struct{}),
		ready:    make(chan struct{}),
		listener: l,
		cleanup: func() {
			l.Close()
			os.RemoveAll(dir)
		},
	}
	return s, nil
}

func (s *server) Close() error {
	if f := s.cleanup; f != nil {
		f()
	}
	return nil
}

func (s *server) Ready() <-chan struct{} {
	return s.ready
}

func (s *server) Done() <-chan struct{} {
	return s.done
}

func (s *server) Run(ctx context.Context) {
	if pdebug.Enabled {
		defer pdebug.Printf("bail out of server.Run")
	}
	defer close(s.done)

	go func() {
		select {
		case <-ctx.Done():
			if pdebug.Enabled {
				pdebug.Printf("context.Context is done, closing listeners")
			}
			s.listener.Close()
		}
	}()

	if pdebug.Enabled {
		pdebug.Printf("server started")
	}
	var once sync.Once
	for {
		if pdebug.Enabled {
			pdebug.Printf("server loop")
		}
		select {
		case <-ctx.Done():
			if pdebug.Enabled {
				pdebug.Printf("cancel detected in server.Run")
			}
			return
		default:
		}

		readerCh := make(chan *fluent.Message)
		go func(ch chan *fluent.Message) {
			if pdebug.Enabled {
				defer pdebug.Printf("bailing out of server reader")
			}
		ACCEPT:
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				once.Do(func() { close(s.ready) })
				conn, err := s.listener.Accept()
				if err != nil {
					if pdebug.Enabled {
						pdebug.Printf("Failed to accept: %s", err)
					}
					return
				}

				if pdebug.Enabled {
					pdebug.Printf("Accepted new connection")
				}

				var dec func(interface{}) error
				if s.useJSON {
					dec = json.NewDecoder(conn).Decode
				} else {
					dec = msgpack.NewDecoder(conn).Decode
				}

				for {
					if pdebug.Enabled {
						pdebug.Printf("waiting for next message...")
					}
					// conn.SetReadDeadline(time.Now().Add(5 * time.Second))
					var v fluent.Message
					if err := dec(&v); err != nil {
						var decName string
						if s.useJSON {
							decName = "json"
						} else {
							decName = "msgpack"
						}
						if pdebug.Enabled {
							pdebug.Printf("test server: failed to decode %s: %s", decName, err)
						}
						if errors.Cause(err) == io.EOF {
							if pdebug.Enabled {
								pdebug.Printf("test server: EOF detected")
							}
							conn.Close()
							continue ACCEPT
						}
						continue
					}

					if pdebug.Enabled {
						pdebug.Printf("Read new fluet.Message")
					}
					select {
					case <-ctx.Done():
						if pdebug.Enabled {
							pdebug.Printf("bailing out of read loop")
						}
						return
					case ch <- &v:
						if pdebug.Enabled {
							pdebug.Printf("Sent new message to read channel")
						}
					}
				}
			}
		}(readerCh)

		for {
			var v *fluent.Message
			select {
			case <-ctx.Done():
				if pdebug.Enabled {
					pdebug.Printf("bailout")
				}
				return
			case v = <-readerCh:
				if pdebug.Enabled {
					pdebug.Printf("new payload: %#v", v)
				}
			}

			// This is some silly stuff, but msgpack would return
			// us map[interface{}]interface{} instead of map[string]interface{}
			// we force the usage of map[string]interface here, so testing is easier
			switch v.Record.(type) {
			case map[interface{}]interface{}:
				newMap := map[string]interface{}{}
				for key, val := range v.Record.(map[interface{}]interface{}) {
					newMap[key.(string)] = val
				}
				v.Record = newMap
			}
			s.Payload = append(s.Payload, v)
		}
	}
}
