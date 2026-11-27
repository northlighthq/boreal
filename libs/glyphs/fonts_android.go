//go:build android

package glyphs

var fallbackFiles = []string{
	"NotoSansSymbols-Regular-Subsetted2.ttf",
	"NotoSansSymbols-Regular-Subsetted.ttf",
	"DroidSansMono.ttf",
	"NotoSansCJK-Regular.ttc",
	"Roboto-Regular.ttf",
}

var emojiFiles = []string{"NotoColorEmoji.ttf"}

func fontDirs() []string {
	return []string{"/system/fonts", "/product/fonts"}
}
