package clickhouse

import (
	"os"
	"testing"

	ch "github.com/ClickHouse/clickhouse-go/v2"
)

func TestApplyDefaults(t *testing.T) {
	c := &Config{}
	ApplyDefaults(c)
	if c.Host != "127.0.0.1" || c.Port != 9000 {
		t.Fatalf("host/port: %+v", c)
	}
	if c.Auth.Database != "default" || c.Auth.Username != "default" {
		t.Fatalf("auth defaults: %+v", c.Auth)
	}
	if c.DialTimeoutSeconds != 30 || c.HealthCheck.TimeoutSeconds != 5 {
		t.Fatalf("timeouts: %+v", c)
	}
}

func TestValidateBadProtocol(t *testing.T) {
	c := &Config{Protocol: "grpc"}
	ApplyDefaults(c)
	if err := Validate(c); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolvedAddrsFromHostPort(t *testing.T) {
	c := &Config{Host: "127.0.0.1", Port: 9000}
	ApplyDefaults(c)
	addrs, err := resolvedAddrs(c)
	if err != nil || len(addrs) != 1 || addrs[0] != "127.0.0.1:9000" {
		t.Fatalf("addrs=%v err=%v", addrs, err)
	}
}

func TestCompressionMapping(t *testing.T) {
	for _, tc := range []struct {
		method string
		ok     bool
	}{
		{"lz4", true},
		{"none", true},
		{"bogus", false},
	} {
		_, err := compressionFromConfig(CompressionConfig{Method: tc.method})
		if tc.ok && err != nil {
			t.Fatalf("%s: %v", tc.method, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("%s: expected error", tc.method)
		}
	}
}

func TestTLSMinVersion(t *testing.T) {
	_, err := buildTLS(TLSConfig{Enabled: true, MinVersion: "1.1"})
	if err == nil {
		t.Fatal("expected error for bad tls version")
	}
}

func TestBuildDriverOptionsMapsStrategyAndProtocol(t *testing.T) {
	c := &Config{
		Host:             "10.0.0.1",
		Port:             9000,
		Protocol:         "http",
		ConnOpenStrategy: "round_robin",
		Auth: AuthConfig{
			Database: "db1",
			Username: "u1",
			Password: "p1",
		},
		Settings: map[string]any{"max_execution_time": 120},
	}
	ApplyDefaults(c)
	if err := Validate(c); err != nil {
		t.Fatal(err)
	}
	o, err := BuildDriverOptions(c)
	if err != nil {
		t.Fatal(err)
	}
	if o.Protocol.String() != "http" {
		t.Fatalf("protocol %v", o.Protocol)
	}
	if o.ConnOpenStrategy != ch.ConnOpenRoundRobin {
		t.Fatalf("strategy %v", o.ConnOpenStrategy)
	}
	if o.Settings["max_execution_time"] != 120 {
		t.Fatalf("settings %v", o.Settings)
	}
}

func TestIntegrationPing(t *testing.T) {
	if os.Getenv("OWL_CLICKHOUSE_INTEGRATION") != "1" {
		t.Skip("set OWL_CLICKHOUSE_INTEGRATION=1 and ensure ClickHouse is reachable")
	}
	c := &Config{}
	ApplyDefaults(c)
	c.HealthCheck.Enabled = true
	if err := Validate(c); err != nil {
		t.Fatal(err)
	}
	cli, err := NewClient(c, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cli.Close() }()
	if err := HealthCheck(cli, c); err != nil {
		t.Fatal(err)
	}
}
