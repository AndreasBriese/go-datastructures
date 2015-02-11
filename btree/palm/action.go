/*
Copyright 2014 Workiva, LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package palm

import (
	//"log"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/Workiva/go-datastructures/queue"
)

type gets Keys
type adds Keys

type actions []action

type action interface {
	operation() operation
	keys() Keys
	complete()
	addNode(int64, *node)
	nodes() []*node
}

type getAction struct {
	result    Keys
	completer *sync.WaitGroup
}

func (ga *getAction) complete() {
	ga.completer.Done()
}

func (ga *getAction) operation() operation {
	return get
}

func (ga *getAction) keys() Keys {
	return ga.result
}

func (ga *getAction) addNode(i int64, n *node) {
	return // not necessary for gets
}

func (ga *getAction) nodes() []*node {
	return nil
}

func newGetAction(keys Keys) *getAction {
	result := make(Keys, len(keys))
	copy(result, keys) // don't want to mutate passed in keys
	ga := &getAction{
		result:    result,
		completer: new(sync.WaitGroup),
	}
	ga.completer.Add(1)
	return ga
}

type insertAction struct {
	result    Keys
	completer *sync.WaitGroup
	ns        []*node
}

func (ia *insertAction) complete() {
	ia.completer.Done()
}

func (ia *insertAction) operation() operation {
	return add
}

func (ia *insertAction) keys() Keys {
	return ia.result
}

func (ia *insertAction) addNode(i int64, n *node) {
	ia.ns[i] = n
}

func (ia *insertAction) nodes() []*node {
	return ia.ns
}

func newInsertAction(keys Keys) *insertAction {
	result := make(Keys, len(keys))
	copy(result, keys)
	ia := &insertAction{
		result:    result,
		completer: new(sync.WaitGroup),
		ns:        make([]*node, len(keys)),
	}
	ia.completer.Add(1)
	return ia
}

func executeInParallel(q *queue.RingBuffer, fn func(interface{})) {
	if q == nil {
		return
	}

	todo, done := q.Len(), uint64(0)
	if todo == 0 {
		return
	}

	goRoutines := minUint64(todo, uint64(runtime.NumCPU()-1))

	var wg sync.WaitGroup
	wg.Add(1)

	for i := uint64(0); i < goRoutines; i++ {
		go func() {
			for {
				ifc, err := q.Get()
				if err != nil {
					return
				}
				fn(ifc)

				if atomic.AddUint64(&done, 1) == todo {
					wg.Done()
					break
				}
			}
		}()
	}
	wg.Wait()
	q.Dispose()
}

func minUint64(choices ...uint64) uint64 {
	min := choices[0]
	for i := 1; i < len(choices); i++ {
		if choices[i] < min {
			min = choices[i]
		}
	}

	return min
}

type interfaces []interface{}

func executeInterfacesInParallel(ifs interfaces, fn func(interface{})) {
	if len(ifs) == 0 {
		return
	}

	done := int64(-1)
	numCPU := uint64(runtime.NumCPU())
	if numCPU > 1 {
		numCPU--
	}

	numCPU = minUint64(numCPU, uint64(len(ifs)))

	var wg sync.WaitGroup
	wg.Add(int(numCPU))

	for i := uint64(0); i < numCPU; i++ {
		go func() {
			defer wg.Done()

			for {
				i := atomic.AddInt64(&done, 1)
				if i >= int64(len(ifs)) {
					return
				}

				fn(ifs[i])
			}
		}()
	}

	wg.Wait()
}
