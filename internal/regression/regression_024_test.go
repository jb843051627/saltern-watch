package regression

import (
	"sync"
	"testing"

	"github.com/jb843051627/saltern-watch/internal/service"
)

func TestBug24_ConcurrentCacheReadersRaceFree(t *testing.T) {
	c := service.NewReadingCache()
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				c.Update(service.PondSnapshot{PondID: int64(i % 32), Be: float64(g)})
			}
		}(g)
	}
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 400; i++ {
				_, _ = c.Get(int64(i % 32))
				_ = c.All()
			}
		}()
	}
	wg.Wait()
}
