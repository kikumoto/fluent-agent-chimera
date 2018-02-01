package chimera_test

import (
	"os"
	"testing"

	chimera "github.com/kikumoto/fluent-agent-chimera"
	pdebug "github.com/lestrrat/go-pdebug"
	"github.com/stretchr/testify/assert"
)

func TestReadConfig(t *testing.T) {
	if pdebug.Enabled {
		g := pdebug.Marker("TestReadConfig")
		defer g.End()
	}
	hostname, _ := os.Hostname()

	config, err := chimera.ReadConfig("./config_test.toml")
	if !assert.NoError(t, err, `chimera.ReadConfig should succeed`) {
		return
	}
	if pdebug.Enabled {
		pdebug.Printf("config => %v\n", config)
	}

	if !assert.Equal(t, "web", config.TagPrefix, "invalid TagPrefix") {
		return
	}
	if !assert.Equal(t, "message", config.FieldName, "invalid FieldName") {
		return
	}
	if !assert.Equal(t, "path", config.PathFieldName, "invalid PathFieldName") {
		return
	}
	if !assert.Equal(t, "host", config.HostFieldName, "invalid HostFieldName") {
		return
	}
	if !assert.Equal(t, hostname, config.Host, "invalid Host") {
		return
	}
	if !assert.Equal(t, 1024, config.ReadBufferSize, "invalid ReadBufferSize") {
		return
	}
	if !assert.Equal(t, true, config.SubSecondTime, "invalid SubSecondTime") {
		return
	}

	s := config.Server
	if !assert.True(
		t,
		s.Network == "tcp" && s.Address == "127.0.0.1:24225",
		"invalid server got %v",
		s,
	) {
		return
	}

	if !assert.Equal(t, 2, len(config.Logs), "invlalid logs %v", config.Logs) {
		return
	}

	var c *chimera.ConfigLogfile
	c = config.Logs[0]
	if !assert.True(
		t,
		c.Tag == "web.app1.batch" &&
			c.Basedir == "/var/log/app1/batch" &&
			c.Recursive &&
			c.FileTimeFormat == "20060102" &&
			c.FieldName == "message" &&
			c.PathFieldName == "path" &&
			c.HostFieldName == "host" &&
			c.Host == hostname,
		"invlalid Logs[0] got %v",
		c,
	) {
		return
	}
	if ret := c.TargetFileRegexp.FindStringSubmatch("/path/to/batch/sample20171231.log"); !assert.True(
		t,
		len(ret) == 2 &&
			ret[1] == "20171231",
		"invalid Logs[0].TargetFileRegexp: %v, match: %v",
		c.TargetFileRegexp,
		ret,
	) {
		return
	}

	c = config.Logs[1]
	if !assert.True(
		t,
		c.Tag == "web.app2.weblog" &&
			c.Basedir == "/var/log/app2/weblog" &&
			!c.Recursive &&
			c.FileTimeFormat == "2006/01/02" &&
			c.FieldName == "msg" &&
			c.PathFieldName == "file" &&
			c.HostFieldName == "server" &&
			c.Host == "thishost",
		"invlalid Logs[1] got %v",
		c,
	) {
		return
	}
	if ret := c.TargetFileRegexp.FindStringSubmatch("/path/to/weblog/access2017/12/31.log"); !assert.True(
		t,
		len(ret) == 2 &&
			ret[1] == "2017/12/31",
		"invalid Logs[0].TargetFileRegexp: %v, match: %v",
		c.TargetFileRegexp,
		ret,
	) {
		return
	}
}
