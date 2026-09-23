package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPriceAlertOpenAPIValidationAcceptsFractionalDecimal(t *testing.T) {
	validator, err := openAPIValidator()
	if err != nil {
		t.Fatal(err)
	}
	next := http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) { response.WriteHeader(http.StatusNoContent) })
	for _, test := range []struct {
		name, target string
		want         int
	}{
		{name: "fraction", target: "1.2", want: http.StatusNoContent},
		{name: "maximum precision", target: "99999999999999999999.123456789012345678", want: http.StatusNoContent},
		{name: "too many integer digits", target: "999999999999999999999", want: http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/instruments/BTCUSDT/alerts", strings.NewReader(`{"target":"`+test.target+`"}`))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			validator(next).ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.want, response.Body.String())
			}
		})
	}
}
