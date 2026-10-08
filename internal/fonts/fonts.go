// Package fonts embeds JetBrains Mono Regular (SIL Open Font License 1.1,
// see OFL.txt) for rendering screenshots and README animations with the
// same typeface users see in a well-configured terminal.
package fonts

import _ "embed"

// JetBrainsMonoTTF is the TrueType face used by the PNG rasterizer.
//
//go:embed JetBrainsMono-Regular.ttf
var JetBrainsMonoTTF []byte

// JetBrainsMonoWOFF2 is the compressed web face embedded into SVGs.
//
//go:embed JetBrainsMono-Regular.woff2
var JetBrainsMonoWOFF2 []byte

// Family is the CSS font-family name.
const Family = "JetBrains Mono"

// Metrics of JetBrains Mono in em units (unitsPerEm 1000).
const (
	CapHeight = 0.73 // cap height / em
	Advance   = 0.6  // glyph advance / em
)
