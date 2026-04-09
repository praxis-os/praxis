// SPDX-License-Identifier: Apache-2.0

package credentials_test

import (
	"testing"

	"github.com/praxis-os/praxis/credentials"
)

func TestZeroBytes(t *testing.T) {
	t.Run("nil slice is a no-op", func(_ *testing.T) {
		// Must not panic.
		credentials.ZeroBytes(nil)
	})

	t.Run("empty slice is a no-op", func(_ *testing.T) {
		// Must not panic.
		credentials.ZeroBytes([]byte{})
	})

	t.Run("all bytes are zeroed", func(t *testing.T) {
		b := []byte("super-secret-api-key") //nolint:gosec // G101: test data, not a real credential
		credentials.ZeroBytes(b)
		for i, v := range b {
			if v != 0 {
				t.Errorf("b[%d] = %d, want 0", i, v)
			}
		}
	})

	t.Run("original slice is modified in-place", func(t *testing.T) {
		b := []byte{0xAB, 0xCD, 0xEF}
		ptr := &b[0]
		credentials.ZeroBytes(b)
		// Confirm the same backing array was modified, not a copy.
		if &b[0] != ptr {
			t.Error("ZeroBytes returned a different backing array")
		}
		for i, v := range b {
			if v != 0 {
				t.Errorf("b[%d] = 0x%X, want 0x00", i, v)
			}
		}
	})

	t.Run("single byte slice is zeroed", func(t *testing.T) {
		b := []byte{0xFF}
		credentials.ZeroBytes(b)
		if b[0] != 0 {
			t.Errorf("b[0] = 0x%X, want 0x00", b[0])
		}
	})
}
