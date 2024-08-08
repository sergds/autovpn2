package executor

import (
	"context"
	"errors"
	"log"
	"time"
)

const ERR_FINISHED = "executor finished"
const ERR_NOTSTART = "executor is not running"

type Executor struct {
	Steps       []*Step
	currentstep int
	ctx         context.Context
	running     bool
	updateschan chan *ExecutorUpdate
	stepchan    chan *ExecutorUpdate
	lasterr     chan error
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

func (e *Executor) Start(updates chan *ExecutorUpdate) {
	if e.running {
		return
	}
	e.running = true
	e.ctx = context.Background()
	e.currentstep = 0
	e.stepchan = make(chan *ExecutorUpdate)
	e.updateschan = updates
	go func() {
		for {
			msg := <-e.stepchan
			if msg == nil { // HACK!
				break
			}
			e.updateschan <- msg
			if msg.CurrentStep == "error" {
				e.running = false
				e.lasterr <- errors.New(msg.StepMessage)
				log.Println("shutting down stepchan pump: " + msg.StepMessage)
				break
			}
		}
	}()
}

func (e *Executor) IsRunning() bool {
	return e.running
}

func (e *Executor) GetLastError() error {
	select {
	case lerr := <-e.lasterr:
		return lerr
	case <-time.After(time.Millisecond * 100):
		return nil
	}
}

func (e *Executor) Tick() error {
	if len(e.lasterr) != 0 {
		return <-e.lasterr
	}
	if !e.running {
		return errors.New("executor is not running")
	}
	if e.currentstep >= len(e.Steps) {
		e.running = false
		e.stepchan <- nil
		return errors.New(ERR_FINISHED)
	}
	e.updateschan <- &ExecutorUpdate{CurrentStep: e.Steps[e.currentstep].Id}
	e.ctx = e.Steps[e.currentstep].Exec(e.ctx, e.stepchan)
	e.currentstep++
	return nil
}
