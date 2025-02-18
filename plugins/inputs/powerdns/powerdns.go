//go:generate ../../../tools/readme_config_includer/generator
package powerdns

import (
	"bufio"
	_ "embed"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
)

// DO NOT REMOVE THE NEXT TWO LINES! This is required to embed the sampleConfig data.
//
//go:embed sample.conf
var sampleConfig string

type Powerdns struct {
	UnixSockets []string

	Log telegraf.Logger `toml:"-"`
}

var defaultTimeout = 5 * time.Second

func (*Powerdns) SampleConfig() string {
	return sampleConfig
}

func (p *Powerdns) Gather(acc telegraf.Accumulator) error {
	if len(p.UnixSockets) == 0 {
		return p.gatherServer("/var/run/pdns.controlsocket", acc)
	}

	for _, serverSocket := range p.UnixSockets {
		if err := p.gatherServer(serverSocket, acc); err != nil {
			acc.AddError(err)
		}
	}

	return nil
}

func (p *Powerdns) gatherServer(address string, acc telegraf.Accumulator) error {
	conn, err := net.DialTimeout("unix", address, defaultTimeout)
	if err != nil {
		return err
	}

	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(defaultTimeout)); err != nil {
		return err
	}

	// Read and write buffer
	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	// Send command
	if _, err := fmt.Fprint(conn, "show * \n"); err != nil {
		return err
	}
	if err := rw.Flush(); err != nil {
		return err
	}

	// Read data
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 1024)
	for {
		n, err := rw.Read(tmp)
		if err != nil {
			if err != io.EOF {
				return err
			}

			break
		}
		buf = append(buf, tmp[:n]...)
	}

	metrics := string(buf)

	// Process data
	fields := p.parseResponse(metrics)

	// Add server socket as a tag
	tags := map[string]string{"server": address}

	acc.AddFields("powerdns", fields, tags)

	return nil
}

func (p *Powerdns) parseResponse(metrics string) map[string]interface{} {
	values := make(map[string]interface{})

	s := strings.Split(metrics, ",")

	for _, metric := range s[:len(s)-1] {
		m := strings.Split(metric, "=")
		if len(m) < 2 {
			continue
		}

		i, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil {
			p.Log.Errorf("error parsing integer for metric %q: %s", metric, err.Error())
			continue
		}
		values[m[0]] = i
	}

	return values
}

func init() {
	inputs.Add("powerdns", func() telegraf.Input {
		return &Powerdns{}
	})
}
