package utils

import (
	"net/http"
	"testing"
)

func TestInternalClientCAs(t *testing.T) {
	client := GetInternalHTTPClient()
	if client == nil {
		t.Fatal("Expected non-nil HTTP client")
	}

	if client.Transport == nil {
		t.Fatal("Expected non-nil Transport in HTTP client")
	}

	if client.Transport.(*http.Transport).TLSClientConfig == nil {
		t.Fatal("Expected non-nil TLSClientConfig in Transport")
	}

	if client.Transport.(*http.Transport).TLSClientConfig.RootCAs == nil {
		t.Fatal("Expected non-nil RootCAs in TLSClientConfig")
	}
}