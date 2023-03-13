package task

import (
	"context"
	"sync"

	E "github.com/sagernet/sing/common/exceptions"
)

type taskItem struct {
	Name string
	Run  func(ctx context.Context) error
}

type Group struct {
	tasks    []taskItem
	cleanup  func()
	fastFail bool
}

func (g *Group) Append(name string, f func(ctx context.Context) error) {
	g.tasks = append(g.tasks, taskItem{
		Name: name,
		Run:  f,
	})
}

func (g *Group) Append0(f func(ctx context.Context) error) {
	g.tasks = append(g.tasks, taskItem{
		Run: f,
	})
}

func (g *Group) Cleanup(f func()) {
	g.cleanup = f
}

func (g *Group) FastFail() {
	g.fastFail = true
}

func (g *Group) Run(ctx context.Context) error {
	var retAccess sync.Mutex
	var retErr error

	taskCount := int8(len(g.tasks))
	taskCtx, taskFinish := context.WithCancel(context.Background())
	var mixedCtx context.Context
	var mixedFinish context.CancelFunc
	if ctx.Done() != nil || g.fastFail {
		mixedCtx, mixedFinish = context.WithCancel(ctx)
	} else {
		mixedCtx, mixedFinish = taskCtx, taskFinish
	}

	for _, task := range g.tasks {
		currentTask := task
		go func() {
			err := currentTask.Run(mixedCtx)
			retAccess.Lock()
			if err != nil {
				retErr = E.Append(retErr, err, func(err error) error {
					if currentTask.Name == "" {
						return err
					}
					return E.Cause(err, currentTask.Name)
				})
				if g.fastFail {
					mixedFinish()
				}
			}
			taskCount--
			currentCount := taskCount
			retAccess.Unlock()
			if currentCount == 0 {
				taskFinish()
			}
		}()
	}

	var upstreamErr error

	select {
	case <-ctx.Done():
		upstreamErr = ctx.Err()
	case <-taskCtx.Done():
		mixedFinish()
	case <-mixedCtx.Done():
	}

	if g.cleanup != nil {
		g.cleanup()
	}

	<-taskCtx.Done()

	taskFinish()
	mixedFinish()

	retErr = E.Append(retErr, upstreamErr, func(err error) error {
		return E.Cause(err, "upstream")
	})

	return retErr
}
