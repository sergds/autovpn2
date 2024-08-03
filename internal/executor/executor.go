package executor

import (
	"context"
	"errors"
)

const ERR_FINISHED = "executor finished"

type Executor struct {
	Steps       []*Step
	currentstep int
	ctx         context.Context
	running     bool
	updateschan chan *ExecutorUpdate
	stepchan    chan string
}

type ExecutorUpdate struct {
	CurrentStep string
	StepMessage string
}

func NewExecutor() *Executor {
	return &Executor{currentstep: 0, ctx: context.Background(), running: false, Steps: make([]*Step, 0)}
}

func (e *Executor) AddStep(step *Step) {
	e.Steps = append(e.Steps, step)
}

func (e *Executor) Start() {
	if e.running {
		return
	}
	e.running = true
	e.ctx = context.Background()
	e.currentstep = 0
	e.stepchan = make(chan string)
	go func() {
		for {
			msg := <-e.stepchan
			if msg == "!term" { // HACK!
				break
			}
			e.updateschan <- &ExecutorUpdate{CurrentStep: e.Steps[e.currentstep].Id, StepMessage: msg}
		}
	}()
}

func (e *Executor) IsRunning() bool {
	return e.running
}

func (e *Executor) Tick(updates chan *ExecutorUpdate) error {
	e.updateschan = updates
	if !e.running {
		return errors.New("executor is not running")
	}
	if e.currentstep >= len(e.Steps) {
		e.running = false
		return errors.New(ERR_FINISHED)
	}
	e.updateschan <- &ExecutorUpdate{CurrentStep: e.Steps[e.currentstep].Id}
	e.ctx = e.Steps[e.currentstep].Exec(e.ctx, e.stepchan)
	e.stepchan <- "!term"
	e.currentstep++
	return nil
}
