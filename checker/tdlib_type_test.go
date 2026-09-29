//go:build tdlib

package checker

import (
	"reflect"
	"testing"

	"github.com/belaytzev/tdmeter/config"
	tdlib "github.com/zelenin/go-tdlib/client"
)

func TestProxyType(t *testing.T) {
	tests := []struct {
		name string
		in   config.ProxyConfig
		want tdlib.ProxyType
	}{
		{"mtproto", config.ProxyConfig{Type: config.ProxyTypeMTProto, Secret: "ee00"}, &tdlib.ProxyTypeMtproto{Secret: "ee00"}},
		{"socks5", config.ProxyConfig{Type: config.ProxyTypeSOCKS5, Username: "u", Password: "p"}, &tdlib.ProxyTypeSocks5{Username: "u", Password: "p"}},
		{"http", config.ProxyConfig{Type: config.ProxyTypeHTTP, Username: "u", Password: "p", HTTPOnly: true}, &tdlib.ProxyTypeHttp{Username: "u", Password: "p", HttpOnly: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := proxyType(tt.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("proxyType() = %#v, want %#v", got, tt.want)
			}
		})
	}

	if _, err := proxyType(config.ProxyConfig{Type: "vpn"}); err == nil {
		t.Fatal("expected error for unknown type")
	}
}
