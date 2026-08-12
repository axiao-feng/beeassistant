// Package javascript 提供基于 goja 的受限 JavaScript 执行内核。
package javascript

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dop251/goja"
)

const DefaultTimeout = 200 * time.Millisecond

// Program 是已编译、可在独立 Runtime 中重复执行的 JavaScript 程序。
type Program struct {
	name    string
	program *goja.Program
}

// Compile 编译 JavaScript 源码，语法错误会在保存或装配阶段暴露。
func Compile(name, source string) (*Program, error) {
	program, err := goja.Compile(name, source, true)
	if err != nil {
		return nil, fmt.Errorf("compile javascript %s: %w", name, err)
	}
	return &Program{name: name, program: program}, nil
}

// Call 在独立 goja Runtime 中加载程序并调用指定函数。
func (p *Program) Call(ctx context.Context, timeout time.Duration, function string, args ...any) (any, error) {
	return p.run(ctx, timeout, func(vm *goja.Runtime) (any, error) {
		callable, ok := goja.AssertFunction(vm.Get(function))
		if !ok {
			return nil, fmt.Errorf("javascript %s must define function %s", p.name, function)
		}
		values := make([]goja.Value, len(args))
		for i, arg := range args {
			values[i] = vm.ToValue(arg)
		}
		value, err := callable(goja.Undefined(), values...)
		if err != nil {
			return nil, normalizeError(p.name, err)
		}
		if goja.IsUndefined(value) || goja.IsNull(value) {
			return nil, nil
		}
		return value.Export(), nil
	})
}

// ValidateFunction 加载程序并确认指定入口函数存在，但不调用它。
func (p *Program) ValidateFunction(ctx context.Context, timeout time.Duration, function string) error {
	_, err := p.run(ctx, timeout, func(vm *goja.Runtime) (any, error) {
		if _, ok := goja.AssertFunction(vm.Get(function)); !ok {
			return nil, fmt.Errorf("javascript %s must define function %s", p.name, function)
		}
		return nil, nil
	})
	return err
}

func (p *Program) run(ctx context.Context, timeout time.Duration, callback func(*goja.Runtime) (any, error)) (any, error) {
	if p == nil || p.program == nil {
		return nil, fmt.Errorf("javascript program is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	vm := goja.New()
	finished := make(chan struct{})
	go func() {
		select {
		case <-runCtx.Done():
			vm.Interrupt(runCtx.Err())
		case <-finished:
		}
	}()
	defer close(finished)

	if _, err := vm.RunProgram(p.program); err != nil {
		return nil, normalizeError(p.name, err)
	}
	return callback(vm)
}

func normalizeError(name string, err error) error {
	var interrupted *goja.InterruptedError
	if errors.As(err, &interrupted) {
		if cause, ok := interrupted.Value().(error); ok {
			return fmt.Errorf("javascript %s interrupted: %w", name, cause)
		}
		return fmt.Errorf("javascript %s interrupted", name)
	}
	return fmt.Errorf("execute javascript %s: %w", name, err)
}
