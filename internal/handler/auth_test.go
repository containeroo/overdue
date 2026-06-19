package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAPI_authorized(t *testing.T) {
	t.Parallel()
	t.Run("allows when auth disabled", func(t *testing.T) {
		t.Parallel()

		api, _ := testAPI("", testLogger())

		assert.True(t, api.authorized(httptest.NewRequest(http.MethodGet, "/", nil)))
	})

	t.Run("allows matching bearer token", func(t *testing.T) {
		t.Parallel()

		api, _ := testAPI("secret", testLogger())
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer secret")

		assert.True(t, api.authorized(req))
	})

	t.Run("rejects missing bearer token", func(t *testing.T) {
		t.Parallel()

		api, _ := testAPI("secret", testLogger())

		assert.False(t, api.authorized(httptest.NewRequest(http.MethodGet, "/", nil)))
	})
}

func TestBearerToken(t *testing.T) {
	t.Parallel()
	t.Run("extracts bearer token", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "secret", bearerToken("Bearer secret"))
	})

	t.Run("trims spaces", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "secret", bearerToken("Bearer   secret  "))
	})

	t.Run("ignores other schemes", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, bearerToken("Basic secret"))
	})

	t.Run("ignores empty header", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, bearerToken(""))
	})
}
