package queue

import (
	"errors"
	"sync"
)

type Queue[T any] struct {
	data chan T
}

func NewQueue[T any](size int) *Queue[T] {
	return &Queue[T]{data: make(chan T, size)}
}

func (q *Queue[T]) Enqueue(value T) {
	select {
	case q.data <- value:

	default:
		return
	}
}

func (q *Queue[T]) Dequeue() (T, error) {
	var res T
	select {
	case res = <-q.data:

	default:
		return res, errors.New("empty queue")
	}
	return res, nil
}

type CQueue[T any] struct {
	data []T

	lock     *sync.Mutex
	notEmpty sync.Cond
}

func NewCQueue[T any]() *CQueue[T] {
	var lock sync.Mutex
	return &CQueue[T]{
		data:     make([]T, 0),
		notEmpty: *sync.NewCond(&lock),
		lock:     &lock,
	}
}

func (q *CQueue[T]) Enqueue(value T) {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.data = append(q.data, value)
	q.notEmpty.Signal()
}

func (q *CQueue[T]) Dequeue() T {
	q.lock.Lock()
	defer q.lock.Unlock()

	for len(q.data) == 0 {
		q.notEmpty.Wait()
	}

	res := q.data[len(q.data)-1]
	q.data = q.data[:len(q.data)-1]

	return res
}
