// SPDX-License-Identifier: Apache-2.0

package credentials

import "runtime"

// ZeroBytes overwrites every byte in b with zero and prevents the compiler
// from eliding the writes via a [runtime.KeepAlive] fence.
//
// Credential implementations must call ZeroBytes on all secret material
// before Close returns.
//
// ZeroBytes is safe to call with a nil or zero-length slice.
func ZeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
	runtime.KeepAlive(b)
}
