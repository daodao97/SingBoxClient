package common

import (
	"context"
	"reflect"
)

func SelectContext(contextList []context.Context) (int, error) {
	chosen, _, _ := reflect.Select(Map(Filter(contextList, func(it context.Context) bool {
		return it.Done() != nil
	}), func(it context.Context) reflect.SelectCase {
		return reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(it.Done()),
		}
	}))
	return chosen, contextList[chosen].Err()
}
