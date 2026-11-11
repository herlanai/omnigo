// Package client exposes the canonical TypeScript SDK to the project generator.
package client

import _ "embed"

//go:embed index.ts
var TypeScript string
