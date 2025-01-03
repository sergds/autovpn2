package executor

import (
	"context"
	"errors"
	"log"
	"time"
)

const ERR_FINISHED = "executor finished"
const ERR_NOTSTART = "executor is not running"

// Executor just does what's on the label.
// In Start() you provide (chan *ExecutorUpdate) update channel to get status updates (including step errors! more on them later) carefully relayed from steps via stepchan pump goroutine.
// on every Tick() It executes every step in order they were added until it reaches the end. Tick() May return unhandled step error or one of two executor's own errors (ERR_FINISHED or ERR_NOTSTART).
// Errors are pretty descriptive: ERR_FINISHED means it ran out of steps to execute (you're done with task), ERR_NOTSTART means you forgot to Start()
// When step returns an ExecutorUpdate with "error" step, the stepchan pump relays it to your update channel, stops executor, and shutdowns itself. if you don't do GetLastError (bad for you), the error will still be returned to you by Tick() and then the executor behaves like a stopped one.
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

func (e *Executor) SetContext(ctx context.Context) {
	if !e.running {
		e.ctx = ctx
	}
}

func (e *Executor) AddStep(step *Step) {
	if !e.running {
		e.Steps = append(e.Steps, step)
	}
}

func (e *Executor) Start(updates chan *ExecutorUpdate) {
	if e.running {
		return
	}
	e.running = true
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
