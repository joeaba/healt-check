package solanahc

import (
  "fmt"
)

type HealthCheckError struct {
  Node string
  Err error
}

func (r *HealthCheckError) Error() string {
  return fmt.Sprintf("[%s] health-check error, %v", r.Node, r.Err)
}

func NewError(node string, e error) *HealthCheckError {
  return &HealthCheckError{
    Node: node,
    Err: e,
  }
}

