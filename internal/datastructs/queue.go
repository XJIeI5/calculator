package queue

import (
	"errors"
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
