package regression

import (
	"sync"
	"testing"

	"github.com/jb843051627/saltern-watch/internal/service"
)

func TestBug01_ConcurrentReadingCacheUpdate(t *testing.T) {
	c := service.NewReadingCache()
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				id := int64(g*100 + i)
				c.Update(service.PondSnapshot{PondID: id, Be: 10.5})
				if _, ok := c.Get(id); !ok {
					t.Errorf("snapshot %d missing right after update", id)
				}
				_ = c.All()
			}
		}(g)
	}
	wg.Wait()
	if got := len(c.All()); got != 400 {
		t.Fatalf("cache size = %d, want 400", got)
	}
}
