package shim

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
)

func TestShimWorks(t *testing.T) {
	stdoutBytes := bytes.NewBufferString("")
	stdout = stdoutBytes

	stdin, _ = io.Pipe() // hold the stdin pipe open

	timeout := time.NewTimer(10 * time.Second)
	metricProcessed, _ := runInputPlugin(t, 10*time.Millisecond)

	select {
	case <-metricProcessed:
	case <-timeout.C:
		require.Fail(t, "Timeout waiting for metric to arrive")
	}
	for stdoutBytes.Len() == 0 {
		select {
		case <-timeout.C:
			require.Fail(t, "Timeout waiting to read metric from stdout")
			return
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	out := string(stdoutBytes.Bytes())
	require.Contains(t, out, "\n")
	metricLine := strings.Split(out, "\n")[0]
	require.Equal(t, "measurement,tag=tag field=1i 1234000005678", metricLine)
}

func TestShimStdinSignalingWorks(t *testing.T) {
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()

	stdin = stdinReader
	stdout = stdoutWriter

	timeout := time.NewTimer(10 * time.Second)
	metricProcessed, exited := runInputPlugin(t, 40*time.Second)

	stdinWriter.Write([]byte("\n"))

	select {
	case <-metricProcessed:
	case <-timeout.C:
		require.Fail(t, "Timeout waiting for metric to arrive")
	}

	r := bufio.NewReader(stdoutReader)
	out, err := r.ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "measurement,tag=tag field=1i 1234000005678\n", out)

	stdinWriter.Close()
	// check that it exits cleanly
	<-exited
}

func runInputPlugin(t *testing.T, interval time.Duration) (metricProcessed chan bool, exited chan bool) {
	metricProcessed = make(chan bool)
	exited = make(chan bool)
	inp := &testInput{
		metricProcessed: metricProcessed,
	}

	shim := New()
	shim.AddInput(inp)
	go func() {
		err := shim.Run(interval)
		require.NoError(t, err)
		exited <- true
	}()
	return metricProcessed, exited
}

type testInput struct {
	metricProcessed chan bool
}

func (i *testInput) SampleConfig() string {
	return ""
}

func (i *testInput) Description() string {
	return ""
}

func (i *testInput) Gather(acc telegraf.Accumulator) error {
	acc.AddFields("measurement",
		map[string]interface{}{
			"field": 1,
		},
		map[string]string{
			"tag": "tag",
		}, time.Unix(1234, 5678))
	i.metricProcessed <- true
	return nil
}

func (i *testInput) Start(acc telegraf.Accumulator) error {
	return nil
}

func (i *testInput) Stop() {
}

func TestLoadConfig(t *testing.T) {
	os.Setenv("SECRET_TOKEN", "xxxxxxxxxx")
	os.Setenv("SECRET_VALUE", `test"\test`)

	inputs.Add("test", func() telegraf.Input {
		return &serviceInput{}
	})

	c := "./testdata/plugin.conf"
	inputs, err := LoadConfig(&c)
	require.NoError(t, err)

	inp := inputs[0].(*serviceInput)

	require.Equal(t, "awesome name", inp.ServiceName)
	require.Equal(t, "xxxxxxxxxx", inp.SecretToken)
	require.Equal(t, `test"\test`, inp.SecretValue)
}

type serviceInput struct {
	ServiceName string `toml:"service_name"`
	SecretToken string `toml:"secret_token"`
	SecretValue string `toml:"secret_value"`
}

func (i *serviceInput) SampleConfig() string {
	return ""
}

func (i *serviceInput) Description() string {
	return ""
}

func (i *serviceInput) Gather(acc telegraf.Accumulator) error {
	acc.AddFields("measurement",
		map[string]interface{}{
			"field": 1,
		},
		map[string]string{
			"tag": "tag",
		}, time.Unix(1234, 5678))

	return nil
}

func (i *serviceInput) Start(acc telegraf.Accumulator) error {
	return nil
}

func (i *serviceInput) Stop() {
}
