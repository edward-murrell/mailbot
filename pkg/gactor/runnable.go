package gactor

import (
	"context"
	"errors"
	"sync"
)

type Runnable func(ctx context.Context) error

type Tasks map[string]Runnable

func (t Tasks) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	var errs []error

	runTask := func(ctx context.Context, runnable Runnable) {
		wg.Add(1)
		defer wg.Done()
		err := runnable(ctx)
		if err != nil {
			cancel()
			errs = append(errs, err)
		}
	}

	for _, task := range t {
		go runTask(ctx, task)
	}
	wg.Wait()
	return errors.Join(errs...)
}
