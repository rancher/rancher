// Copyright © 2018 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: BSD-2-Clause

package fluentd

type Stack struct {
	top    *node
	length int
}

type node struct {
	value interface{}
	prev  *node
}

func NewStack() *Stack {
	return &Stack{nil, 0}
}

func (s *Stack) Len() int {
	return s.length
}

func (s *Stack) Peek() interface{} {
	if s.length == 0 {
		return nil
	}
	return s.top.value
}

func (s *Stack) Pop() interface{} {
	if s.length == 0 {
		return nil
	}

	top := s.top
	s.top = top.prev
	s.length--
	return top.value
}

func (s *Stack) Push(value interface{}) {
	n := &node{
		value: value,
		prev:  s.top,
	}
	s.top = n
	s.length++
}
