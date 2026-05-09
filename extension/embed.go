//go:build release

package extensionassets

import "embed"

// Bundles contains the Paperback repository assets produced by the extension build.
//
//go:embed bundles bundles/Mangashelf
var Bundles embed.FS
