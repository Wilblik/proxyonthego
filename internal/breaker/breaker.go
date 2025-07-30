package breaker

import (
	"sync/atomic"
	"time"
)

type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

type CircuitBreaker struct {
	failureThreshold int64
	resetTimeout time.Duration
	state atomic.Int32
	failures atomic.Int64
	lastFailureTime atomic.Int64
}

func New(failureThreshold int64, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		failureThreshold: failureThreshold,
		resetTimeout: resetTimeout,
	}
}

func (cb *CircuitBreaker) Ready() bool {
	currState := State(cb.state.Load())
	if currState == StateClosed || currState == StateHalfOpen {
		return true
	}
	lastFailure := cb.lastFailureTime.Load()
	timeFromLastFailure := time.Now().UnixNano() - lastFailure
	if timeFromLastFailure > int64(cb.resetTimeout) {
		return cb.state.CompareAndSwap(int32(StateOpen), int32(StateHalfOpen))
	}
	return false
}

func (cb *CircuitBreaker) RecordSuccess() {
	if State(cb.state.Load()) == StateClosed && cb.failures.Load() == 0 {
		return;
	}
	if cb.state.CompareAndSwap(int32(StateHalfOpen), int32(StateClosed)) {
		cb.failures.Store(0)
	}
	cb.failures.Store(0)
}

func (cb *CircuitBreaker) RecordFailure() {
	if cb.state.CompareAndSwap(int32(StateHalfOpen), int32(StateOpen)) {
		cb.lastFailureTime.Store(time.Now().UnixNano())
		return
	}
	count := cb.failures.Add(1)
	if count >= cb.failureThreshold {
		cb.state.CompareAndSwap(int32(StateClosed), int32(StateOpen))
		cb.lastFailureTime.Store(time.Now().UnixNano())
	}
}
