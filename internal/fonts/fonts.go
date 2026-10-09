// Package fonts embeds the typefaces used for screenshots and README
// animations so renders look the same on every machine: JetBrains Mono
// Regular for text and Noto Emoji Regular (monochrome) for emoji glyphs
// JetBrains Mono lacks. Both are under the SIL Open Font License 1.1, see
// OFL.txt and OFL-NotoEmoji.txt.
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

// NotoEmojiTTF is the monochrome emoji face the PNG rasterizer falls back
// to for glyphs missing from JetBrains Mono. It is the static Regular
// instance served by Google Fonts.
//
//go:embed NotoEmoji-Regular.ttf
var NotoEmojiTTF []byte

// NotoEmojiWOFF is the same face as a web font, embedded into SVGs only on
// request (svg.Options.EmbedEmoji) because of its size.
//
//go:embed NotoEmoji-Regular.woff
var NotoEmojiWOFF []byte

// Family is the CSS font-family name of the text face.
const Family = "JetBrains Mono"

// EmojiFamily is the CSS font-family name of the bundled emoji face.
const EmojiFamily = "Noto Emoji"

// Metrics of JetBrains Mono in em units (unitsPerEm 1000).
const (
	CapHeight = 0.73 // cap height / em
	Advance   = 0.6  // glyph advance / em
)
