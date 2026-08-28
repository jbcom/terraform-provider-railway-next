// SPDX-License-Identifier: MPL-2.0

package tools

// Docs generation lives in `generate.sh`, which explains why it cannot be a
// single `tfplugindocs` invocation: this provider is not under the `hashicorp/`
// namespace, and `tfplugindocs` assumes it is.
//
//go:generate ./generate.sh
