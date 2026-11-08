package termview

import (
	"sync"

	"github.com/northlighthq/glyphs"
)

type cacheKey struct {
	fonts *glyphs.Set
	px    int
	line  float32
}

var caches = struct {
	sync.Mutex
	m map[cacheKey]*glyphs.Cache
}{m: map[cacheKey]*glyphs.Cache{}}

func sharedCache(fonts *glyphs.Set, px int, line float32) *glyphs.Cache {
	k := cacheKey{fonts, px, line}

	caches.Lock()
	defer caches.Unlock()

	c, ok := caches.m[k]

	if !ok {
		for old := range caches.m {
			if old.fonts != fonts {
				delete(caches.m, old)
			}
		}

		c = glyphs.NewCache(fonts, float32(px), line)
		caches.m[k] = c
	}

	return c
}
