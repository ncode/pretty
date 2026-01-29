package cmd

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestParseHostSpec(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    HostSpec
		wantErr bool
	}{
		{"host_only", "host1", HostSpec{Host: "host1", Port: defaultPort}, false},
		{"host_with_port", "host1:2222", HostSpec{Host: "host1", Port: 2222, PortSet: true}, false},
		{"trims_spaces", " host1 ", HostSpec{Host: "host1", Port: defaultPort}, false},
		{"empty", "", HostSpec{}, true},
		{"missing_port", "host1:", HostSpec{}, true},
		{"bad_port", "host1:abc", HostSpec{}, true},
		{"port_out_of_range", "host1:70000", HostSpec{}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseHostSpec(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("unexpected spec: %+v", got)
			}
		})
	}
}

func TestParseHostSpecUserAtHost(t *testing.T) {
	got, err := parseHostSpec("deploy@host1:2222")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := HostSpec{Host: "host1", Port: 2222, User: "deploy", PortSet: true, UserSet: true}
	if got != want {
		t.Fatalf("unexpected spec: %+v", got)
	}
}

func TestParseHostSpecIPv6User(t *testing.T) {
	got, err := parseHostSpec("admin@[2001:db8::1]:2222")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := HostSpec{Host: "2001:db8::1", Port: 2222, User: "admin", PortSet: true, UserSet: true}
	if got != want {
		t.Fatalf("unexpected spec: %+v", got)
	}
}

func TestParseHostsFile(t *testing.T) {
	data := []byte("host1\n\nhost2:2222\n  host3  \n")
	specs, err := parseHostsFile(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []HostSpec{
		{Host: "host1", Port: defaultPort},
		{Host: "host2", Port: 2222, PortSet: true},
		{Host: "host3", Port: defaultPort},
	}
	if !reflect.DeepEqual(specs, want) {
		t.Fatalf("unexpected specs: %+v", specs)
	}
}

func TestParseHostsFileInvalidPort(t *testing.T) {
	_, err := parseHostsFile([]byte("host1:abc\n"))
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("expected line number in error, got %v", err)
	}
}

func TestParseGroupSpecsFromViper(t *testing.T) {
	v := viper.New()
	v.SetConfigType("yaml")
	err := v.ReadConfig(strings.NewReader(
		"groups:\n" +
			"  testbed:\n" +
			"    user: deploy\n" +
			"    hosts:\n" +
			"      - host1\n" +
			"      - host2:2222\n",
	))
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}

	raw := v.Get("groups.testbed")
	specs, err := parseGroupSpecs(raw, "testbed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []HostSpec{
		{Host: "host1", Port: defaultPort, User: "deploy", UserSet: true},
		{Host: "host2", Port: 2222, User: "deploy", PortSet: true, UserSet: true},
	}
	if !reflect.DeepEqual(specs, want) {
		t.Fatalf("unexpected specs: %+v", specs)
	}
}

func TestParseGroupSpecsUserPrecedence(t *testing.T) {
	v := viper.New()
	v.SetConfigType("yaml")
	err := v.ReadConfig(strings.NewReader(
		"groups:\n" +
			"  prod:\n" +
			"    user: deploy\n" +
			"    hosts:\n" +
			"      - admin@host1:2222\n" +
			"      - host2:2223\n",
	))
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}

	specs, err := parseGroupSpecs(v.Get("groups.prod"), "prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []HostSpec{
		{Host: "host1", Port: 2222, User: "admin", PortSet: true, UserSet: true},
		{Host: "host2", Port: 2223, User: "deploy", PortSet: true, UserSet: true},
	}
	if !reflect.DeepEqual(specs, want) {
		t.Fatalf("unexpected specs: %+v", specs)
	}
}

func TestParseGroupSpecsRejectsStringList(t *testing.T) {
	raw := []interface{}{"host1", "host2"}
	_, err := parseGroupSpecs(raw, "legacy")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "hosts") {
		t.Fatalf("expected wrapper format error, got %v", err)
	}
}

func TestParseGroupSpecsInvalidPort(t *testing.T) {
	raw := map[string]interface{}{
		"hosts": []interface{}{"host1:70000"},
	}
	_, err := parseGroupSpecs(raw, "bad")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseGroupSpecsMissingHost(t *testing.T) {
	raw := map[string]interface{}{
		"user": "deploy",
	}
	_, err := parseGroupSpecs(raw, "bad")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseArgsHosts(t *testing.T) {
	specs, err := parseArgsHosts([]string{"host1", "host2:2222"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []HostSpec{
		{Host: "host1", Port: defaultPort},
		{Host: "host2", Port: 2222, PortSet: true},
	}
	if !reflect.DeepEqual(specs, want) {
		t.Fatalf("unexpected specs: %+v", specs)
	}
}

func TestHostDisplayNameIncludesPort(t *testing.T) {
	spec := HostSpec{Host: "localhost", Port: 2222}
	got := hostDisplayName(spec)
	if got != "localhost:2222" {
		t.Fatalf("unexpected host label: %s", got)
	}
}

func TestPromptFlagIsRegistered(t *testing.T) {
	if RootCmd.PersistentFlags().Lookup("prompt") == nil {
		t.Fatalf("expected --prompt flag to be registered")
	}
}

func TestPromptFlagOverridesConfig(t *testing.T) {
	flag := RootCmd.PersistentFlags().Lookup("prompt")
	if flag == nil {
		t.Fatalf("expected --prompt flag to be registered")
	}

	viper.Reset()
	viper.SetConfigType("yaml")
	if err := viper.ReadConfig(strings.NewReader("prompt: config\n")); err != nil {
		t.Fatalf("unexpected config read error: %v", err)
	}
	if err := viper.BindPFlag("prompt", flag); err != nil {
		t.Fatalf("unexpected bind error: %v", err)
	}

	if err := RootCmd.PersistentFlags().Set("prompt", "cli"); err != nil {
		t.Fatalf("unexpected flag set error: %v", err)
	}
	defer RootCmd.PersistentFlags().Set("prompt", "")

	got := viper.GetString("prompt")
	if got != "cli" {
		t.Fatalf("expected CLI prompt to override config, got %q", got)
	}
}
