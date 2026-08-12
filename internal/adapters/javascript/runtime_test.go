package javascript

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestProgramCallUsesIsolatedRuntime(t *testing.T) {
	program, err := Compile("counter", `
var counter = 0;
function execute(input) {
  counter += input.step;
  return counter;
}`)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	for i := 0; i < 2; i++ {
		result, callErr := program.Call(context.Background(), time.Second, "execute", map[string]any{"step": 2})
		if callErr != nil {
			t.Fatalf("Call() error = %v", callErr)
		}
		if result != int64(2) {
			t.Fatalf("Call() result = %#v, want 2", result)
		}
	}
}

func TestProgramCallInterruptsInfiniteLoop(t *testing.T) {
	program, err := Compile("loop", `function execute() { while (true) {} }`)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	_, err = program.Call(context.Background(), 20*time.Millisecond, "execute")
	if err == nil || !strings.Contains(err.Error(), "interrupted") {
		t.Fatalf("Call() error = %v, want interrupted", err)
	}
}

func TestProgramValidateFunctionRejectsMissingEntry(t *testing.T) {
	program, err := Compile("missing", `function other() {}`)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if err := program.ValidateFunction(context.Background(), time.Second, "execute"); err == nil {
		t.Fatal("ValidateFunction() error = nil, want missing function error")
	}
}
