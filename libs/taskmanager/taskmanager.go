// Package implements add-on for "github.com/robfig/cron"
// It presents synchronous access for tasks.
// Main target - prevent executing similar tasks concurrently.
// Always no more than one task will be executing and no more than one task will be stayed in queue.
// Similar tasks in queue will be skipped.

package taskmanager

import (
	"context"
	"sync"

	"github.com/robfig/cron"
)

// TaskManager main object used for manage planned tasks.
type TaskManager struct {
	cron *cron.Cron
	wg   *sync.WaitGroup
	ctx  context.Context
}

// New TaskManager constructor
func New(ctx context.Context, wg *sync.WaitGroup) *TaskManager {
	tm := &TaskManager{
		cron: cron.New(),
		wg:   wg,
		ctx:  ctx,
	}
	go func() {
		select {
		case <-ctx.Done():
			tm.cron.Stop()
			return
		}
	}()
	return tm
}

// AddTask adds a func to the Cron to be run on the given schedule.
func (tm *TaskManager) AddTask(spec string, cmd func()) error {
	var i int
	m := sync.Mutex{}

	err := tm.cron.AddFunc(spec, func() {
		tm.wg.Add(1)
		defer tm.wg.Done()

		if i >= 2 {
			return
		}
		i++

		m.Lock()
		defer m.Unlock()

		select {
		case <-tm.ctx.Done():
			return
		default:
			cmd()
			i--
		}
	})
	if err != nil {
		return err
	}

	return nil
}

// Start taskManager
func (tm *TaskManager) Start() {
	tm.cron.Start()
}
