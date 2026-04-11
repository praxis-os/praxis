// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"fmt"
	"strings"

	praxiserrors "github.com/praxis-os/praxis/errors"
)

// logicalNameSeparator is the delimiter the adapter uses to namespace
// MCP tool names to the LLM: `{LogicalName}{logicalNameSeparator}{mcpToolName}`.
// It is reserved and must not appear inside a [Server.LogicalName];
// see D111.
const logicalNameSeparator = "__"

// validateServers enforces the construction-time invariants on the
// server slice passed to [New]. A nil return value indicates the
// slice is well-formed. A non-nil return is a typed
// [praxiserrors.SystemError] whose Kind() is
// [praxiserrors.ErrorKindSystem] — construction failures are
// framework/configuration errors, not tool errors.
//
// Invariants enforced (see T30.4 + T30.6):
//
//  1. The slice is non-empty.
//  2. len(servers) <= [MaxServers].
//  3. For every entry:
//     a. LogicalName is non-empty.
//     b. LogicalName length is in [1, 64].
//     c. LogicalName matches `^[a-zA-Z0-9_-]+$`.
//     d. LogicalName does not contain the `__` substring reserved
//     for the tool-name namespace delimiter (D111).
//     e. Transport is non-nil.
//  4. LogicalName values are unique across the whole slice.
//
// The function stops at the first violation and reports it with a
// human-readable message prefixed by the offending server index so
// callers can locate the problem in their construction code.
func validateServers(servers []Server) error {
	if len(servers) == 0 {
		return systemError("New: servers slice is empty; at least one Server is required")
	}
	if len(servers) > MaxServers {
		return systemError(fmt.Sprintf(
			"New: servers slice has %d entries; the per-Invoker cap is %d (see MaxServers and D115 cardinality contract)",
			len(servers), MaxServers,
		))
	}

	seen := make(map[string]int, len(servers))
	for i, s := range servers {
		if err := validateLogicalName(i, s.LogicalName); err != nil {
			return err
		}
		if s.Transport == nil {
			return systemError(fmt.Sprintf(
				"New: servers[%d] (%q): Transport is nil; every Server must specify a TransportStdio or TransportHTTP",
				i, s.LogicalName,
			))
		}
		if prev, ok := seen[s.LogicalName]; ok {
			return systemError(fmt.Sprintf(
				"New: servers[%d] (%q): duplicate LogicalName (first seen at servers[%d]); LogicalNames must be unique across a single New call",
				i, s.LogicalName, prev,
			))
		}
		seen[s.LogicalName] = i
	}

	return nil
}

// validateLogicalName checks a single [Server.LogicalName] value
// against the rules documented on [Server.LogicalName]. The index
// argument is used only to locate the offending entry in the
// returned error message.
func validateLogicalName(index int, name string) error {
	if name == "" {
		return systemError(fmt.Sprintf(
			"New: servers[%d]: LogicalName is empty; every Server must set a non-empty LogicalName",
			index,
		))
	}
	if strings.Contains(name, logicalNameSeparator) {
		return systemError(fmt.Sprintf(
			"New: servers[%d] (%q): LogicalName contains the reserved %q separator used for tool-name namespacing (D111)",
			index, name, logicalNameSeparator,
		))
	}
	if !logicalNameRegexp.MatchString(name) {
		return systemError(fmt.Sprintf(
			"New: servers[%d] (%q): LogicalName must match %s (non-empty, length 1-64, ASCII alphanumeric plus hyphen and underscore)",
			index, name, logicalNameRegexp.String(),
		))
	}
	return nil
}

// systemError is a small helper that constructs a typed
// [praxiserrors.SystemError] without a wrapped cause. All
// construction-time validation errors come from this function so
// they uniformly satisfy the [praxiserrors.TypedError] contract with
// [praxiserrors.ErrorKindSystem].
func systemError(message string) error {
	return praxiserrors.NewSystemError(message, nil)
}
