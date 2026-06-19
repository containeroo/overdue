package dispatch

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNone_Notify(t *testing.T) {
	t.Parallel()
	t.Run("returns nil", func(t *testing.T) {
		t.Parallel()

		err := None{}.Notify(context.Background(), testEvent())

		require.NoError(t, err)
	})
}
