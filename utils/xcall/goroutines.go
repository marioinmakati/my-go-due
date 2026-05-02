package xcall

import (
	"context"
	"sync"
	"time"
)

type Goroutines struct {
	fns []func()
}

func NewGoroutines() *Goroutines {
	return &Goroutines{}
}

// Add 添加协程函数
func (g *Goroutines) Add(fns ...func()) *Goroutines {
	g.fns = append(g.fns, fns...)
	return g
}

// Run 并发执行所有已注册函数，等待全部完成或超时后返回
// timeout 不传或为 0 则无超时，一直等到所有函数完成
// 超时后直接返回，不强制停止正在运行的 goroutine
func (g *Goroutines) Run(ctx context.Context, timeout ...time.Duration) {
	if len(g.fns) == 0 {
		return
	}

	if len(timeout) > 0 && timeout[0] > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout[0])
		defer cancel()
	}

	var wg sync.WaitGroup
	wg.Add(len(g.fns))

	for i := range g.fns {
		fn := g.fns[i]
		Go(func() {
			defer wg.Done()
			fn()
		})
	}

	// 用独立 goroutine 等待 wg 归零，再通过 channel 通知 select
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// 哪个先到就退出：超时或全部完成
	select {
	case <-ctx.Done():
	case <-done:
	}
}
