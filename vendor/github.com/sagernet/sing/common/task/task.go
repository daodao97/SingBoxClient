package task

import (
	"context"
	"sync"

	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

type taskItem struct {
	Name string
	Run  func(ctx context.Context) error
}

type errTaskSucceed struct{}

func (e errTaskSucceed) Error() string {
	return "task succeed"
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

func (g *Group) Run(contextList ...context.Context) error {
	return g.RunContextList(contextList)
}

func (g *Group) RunContextList(contextList []context.Context) error {
	if len(contextList) == 0 {
		contextList = append(contextList, context.Background())
	}

	taskContext, taskFinish := common.ContextWithCancelCause(context.Background())
	taskCancelContext, taskCancel := common.ContextWithCancelCause(context.Background())

	var errorAccess sync.Mutex
	var returnError error
	taskCount := int8(len(g.tasks))

	for _, task := range g.tasks {
		currentTask := task
		go func() {
			err := currentTask.Run(taskCancelContext)
			errorAccess.Lock()
			if err != nil {
				if currentTask.Name != "" {
					err = E.Cause(err, currentTask.Name)
				}
				returnError = E.Errors(returnError, err)
				if g.fastFail {
					taskCancel(err)
				}
			}
			taskCount--
			currentCount := taskCount
			errorAccess.Unlock()
			if currentCount == 0 {
				taskCancel(errTaskSucceed{})
				taskFinish(errTaskSucceed{})
			}
		}()
	}

	selectedContext, upstreamErr := common.SelectContext(append([]context.Context{taskContext}, contextList...))
	if selectedContext != 0 {
		returnError = E.Append(returnError, upstreamErr, func(err error) error {
			return E.Cause(err, "upstream")
		})
	}

	if g.cleanup != nil {
		g.cleanup()
	}

	<-taskContext.Done()
	return returnError
}
