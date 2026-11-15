package httpapi

import (
	"net/http"
	"testing"
)

func TestSecuredOperationWithoutKnownAccessFailsRouting(t *testing.T) {
	for _, access := range []any{nil, "everyone"} {
		spec, err := GetSpec()
		if err != nil {
			t.Fatal(err)
		}
		operation := spec.Paths.Find("/api/v1/admin/users").Get
		if access == nil {
			delete(operation.Extensions, "x-access")
		} else {
			operation.Extensions["x-access"] = access
		}
		passThrough := func(next http.Handler) http.Handler { return next }
		if err := registerSecuredOperations(http.NewServeMux(), spec, passThrough, http.NotFoundHandler()); err == nil {
			t.Errorf("x-access %v: routing succeeded, want an error", access)
		}
	}
}
