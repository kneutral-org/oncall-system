package site

import (
	"testing"
)

func TestExtractSiteCodeFromHostname(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		wantCode string
		wantOk   bool
	}{
		{
			name:     "prefix with hyphen - dfw1-router01",
			hostname: "dfw1-router01",
			wantCode: "dfw1",
			wantOk:   true,
		},
		{
			name:     "prefix with hyphen - nyc2-sw-core-01",
			hostname: "nyc2-sw-core-01",
			wantCode: "nyc2",
			wantOk:   true,
		},
		{
			name:     "middle segment - core-nyc2-sw01",
			hostname: "core-nyc2-sw01",
			wantCode: "nyc2",
			wantOk:   true,
		},
		{
			name:     "domain segment - server.lax1.example.com",
			hostname: "server.lax1.example.com",
			wantCode: "lax1",
			wantOk:   true,
		},
		{
			name:     "suffix with hyphen - router-ord3",
			hostname: "router-ord3",
			wantCode: "ord3",
			wantOk:   true,
		},
		{
			name:     "three letter code - sjc1-router01",
			hostname: "sjc1-router01",
			wantCode: "sjc1",
			wantOk:   true,
		},
		{
			name:     "four letter code with number - lond1-server01",
			hostname: "lond1-server01",
			wantCode: "lond1",
			wantOk:   true,
		},
		{
			name:     "uppercase hostname normalized - DFW1-ROUTER01",
			hostname: "DFW1-ROUTER01",
			wantCode: "dfw1",
			wantOk:   true,
		},
		{
			name:     "empty hostname",
			hostname: "",
			wantCode: "",
			wantOk:   false,
		},
		{
			name:     "no pattern match - server01",
			hostname: "server01",
			wantCode: "",
			wantOk:   false,
		},
		{
			name:     "no pattern match - just numbers",
			hostname: "12345",
			wantCode: "",
			wantOk:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCode, gotOk := ExtractSiteCodeFromHostname(tt.hostname)
			if gotCode != tt.wantCode {
				t.Errorf("ExtractSiteCodeFromHostname() code = %v, want %v", gotCode, tt.wantCode)
			}
			if gotOk != tt.wantOk {
				t.Errorf("ExtractSiteCodeFromHostname() ok = %v, want %v", gotOk, tt.wantOk)
			}
		})
	}
}

func TestExtractSiteCodeFromInstance(t *testing.T) {
	tests := []struct {
		name     string
		instance string
		wantCode string
		wantOk   bool
	}{
		{
			name:     "hostname with port",
			instance: "dfw1-router01:9090",
			wantCode: "dfw1",
			wantOk:   true,
		},
		{
			name:     "hostname without port",
			instance: "nyc2-server01",
			wantCode: "nyc2",
			wantOk:   true,
		},
		{
			name:     "IP address with port should not match",
			instance: "192.168.1.1:9090",
			wantCode: "",
			wantOk:   false,
		},
		{
			name:     "IP address without port should not match",
			instance: "10.0.0.1",
			wantCode: "",
			wantOk:   false,
		},
		{
			name:     "empty instance",
			instance: "",
			wantCode: "",
			wantOk:   false,
		},
		{
			name:     "FQDN with port",
			instance: "server.lax1.example.com:8080",
			wantCode: "lax1",
			wantOk:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCode, gotOk := ExtractSiteCodeFromInstance(tt.instance)
			if gotCode != tt.wantCode {
				t.Errorf("ExtractSiteCodeFromInstance() code = %v, want %v", gotCode, tt.wantCode)
			}
			if gotOk != tt.wantOk {
				t.Errorf("ExtractSiteCodeFromInstance() ok = %v, want %v", gotOk, tt.wantOk)
			}
		})
	}
}

func TestIsIPAddress(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{
			name: "IPv4 address",
			s:    "192.168.1.1",
			want: true,
		},
		{
			name: "IPv4 with zeros",
			s:    "10.0.0.1",
			want: true,
		},
		{
			name: "hostname looks like IP but has letters",
			s:    "192.168.1.abc",
			want: false,
		},
		{
			name: "IPv6 address",
			s:    "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			want: true,
		},
		{
			name: "hostname",
			s:    "server01.example.com",
			want: false,
		},
		{
			name: "simple hostname",
			s:    "router01",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isIPAddress(tt.s); got != tt.want {
				t.Errorf("isIPAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeSiteCode(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{
			name: "already lowercase",
			code: "dfw1",
			want: "dfw1",
		},
		{
			name: "uppercase",
			code: "DFW1",
			want: "dfw1",
		},
		{
			name: "mixed case",
			code: "DfW1",
			want: "dfw1",
		},
		{
			name: "with whitespace",
			code: "  dfw1  ",
			want: "dfw1",
		},
		{
			name: "empty",
			code: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeSiteCode(tt.code); got != tt.want {
				t.Errorf("NormalizeSiteCode() = %v, want %v", got, tt.want)
			}
		})
	}
}
