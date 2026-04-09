// SPDX-License-Identifier: Apache-2.0

package credentials

// ZeroBytes overwrites every byte in b with zero. It is a building block for
// zero-on-close semantics: callers that hold sensitive credential material in
// a byte slice should call ZeroBytes before releasing or reusing the slice so
// that the secret does not linger in heap memory.
//
// ZeroBytes is a no-op when b is nil or empty.
func ZeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
