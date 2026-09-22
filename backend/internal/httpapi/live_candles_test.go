package httpapi

import (
	"net/http/httptest"
	"testing"
)

func TestSameWebSocketOrigin(t *testing.T) {
	tests := []struct {
		name   string
		host   string
		origin string
		want   bool
	}{
		{name: "same HTTPS origin", host: "scanner.example", origin: "https://scanner.example", want: true},
		{name: "same development origin", host: "127.0.0.1:3000", origin: "http://127.0.0.1:3000", want: true},
		{name: "foreign origin", host: "scanner.example", origin: "https://attacker.example", want: false},
		{name: "missing origin", host: "scanner.example", want: false},
		{name: "non HTTP scheme", host: "scanner.example", origin: "file://scanner.example", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest("GET", "http://"+test.host+"/api/v1/live/candles", nil)
			request.Host = test.host
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if got := sameWebSocketOrigin(request); got != test.want {
				t.Fatalf("sameWebSocketOrigin() = %t, want %t", got, test.want)
			}
		})
	}
}
