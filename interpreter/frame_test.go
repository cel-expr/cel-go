// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package interpreter

import (
	"context"
	"testing"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

func TestFrameCheckInterrupt(t *testing.T) {
	tests := []struct {
		name     string
		setupCtx func(ctx context.Context, f *ExecutionFrame) (context.CancelFunc, func(int))
		checks   []bool
	}{
		{
			name:   "nil context",
			checks: []bool{false, false},
		},
		{
			name: "zero frequency not canceled",
			setupCtx: func(ctx context.Context, f *ExecutionFrame) (context.CancelFunc, func(int)) {
				f.SetContext(ctx, 0)
				return nil, nil
			},
			checks: []bool{false, false},
		},
		{
			name: "frequency one not canceled",
			setupCtx: func(ctx context.Context, f *ExecutionFrame) (context.CancelFunc, func(int)) {
				f.SetContext(ctx, 1)
				return nil, nil
			},
			checks: []bool{false, false},
		},
		{
			name: "frequency one canceled dynamically",
			setupCtx: func(ctx context.Context, f *ExecutionFrame) (context.CancelFunc, func(int)) {
				c, cancel := context.WithCancel(ctx)
				f.SetContext(c, 1)
				return nil, func(step int) {
					if step == 1 {
						cancel()
					}
				}
			},
			checks: []bool{false, true, true},
		},
		{
			name: "frequency two not canceled",
			setupCtx: func(ctx context.Context, f *ExecutionFrame) (context.CancelFunc, func(int)) {
				f.SetContext(ctx, 2)
				return nil, nil
			},
			checks: []bool{false, false, false},
		},
		{
			name: "frequency two canceled dynamically",
			setupCtx: func(ctx context.Context, f *ExecutionFrame) (context.CancelFunc, func(int)) {
				c, cancel := context.WithCancel(ctx)
				f.SetContext(c, 2)
				return nil, func(step int) {
					if step == 1 {
						cancel()
					}
				}
			},
			checks: []bool{false, true, true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			frame := mustNewExecutionFrame(t, EmptyActivation())
			defer frame.Close()

			var cleanup context.CancelFunc
			var stepHook func(int)
			if tc.setupCtx != nil {
				cleanup, stepHook = tc.setupCtx(context.Background(), frame)
			}
			if cleanup != nil {
				defer cleanup()
			}

			for i, want := range tc.checks {
				if stepHook != nil {
					stepHook(i)
				}
				got := frame.CheckInterrupt()
				if got != want {
					t.Errorf("CheckInterrupt() call %d got %t, want %t", i+1, got, want)
				}
			}
		})
	}
}

func TestFrameResolveName(t *testing.T) {
	baseAct, err := NewActivation(map[string]any{"x": 1})
	if err != nil {
		t.Fatalf("NewActivation(x) failed: %v", err)
	}
	childAct, err := NewActivation(map[string]any{"y": 2})
	if err != nil {
		t.Fatalf("NewActivation(y) failed: %v", err)
	}

	tests := []struct {
		name      string
		setup     func() *ExecutionFrame
		varName   string
		wantVal   any
		wantFound bool
	}{
		{
			name: "resolve in base activation",
			setup: func() *ExecutionFrame {
				return mustNewExecutionFrame(t, baseAct)
			},
			varName:   "x",
			wantVal:   1,
			wantFound: true,
		},
		{
			name: "missing in base activation",
			setup: func() *ExecutionFrame {
				return mustNewExecutionFrame(t, baseAct)
			},
			varName:   "y",
			wantVal:   nil,
			wantFound: false,
		},
		{
			name: "resolve in child activation",
			setup: func() *ExecutionFrame {
				f := mustNewExecutionFrame(t, baseAct)
				return f.Push(childAct)
			},
			varName:   "y",
			wantVal:   2,
			wantFound: true,
		},
		{
			name: "resolve in parent activation from child",
			setup: func() *ExecutionFrame {
				f := mustNewExecutionFrame(t, baseAct)
				return f.Push(childAct)
			},
			varName:   "x",
			wantVal:   1,
			wantFound: true,
		},
		{
			name: "missing in hierarchical activation",
			setup: func() *ExecutionFrame {
				f := mustNewExecutionFrame(t, baseAct)
				return f.Push(childAct)
			},
			varName:   "z",
			wantVal:   nil,
			wantFound: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			frame := tc.setup()
			defer func() {
				curr := frame
				for curr.parent != nil {
					curr = curr.Pop()
				}
				curr.Close()
			}()

			gotVal, gotFound := frame.ResolveName(tc.varName)
			if gotFound != tc.wantFound || gotVal != tc.wantVal {
				t.Errorf("ResolveName(%q) got (%v, %t), want (%v, %t)", tc.varName, gotVal, gotFound, tc.wantVal, tc.wantFound)
			}
		})
	}
}

func TestFrameParent(t *testing.T) {
	baseAct, err := NewActivation(map[string]any{"x": 1})
	if err != nil {
		t.Fatalf("NewActivation(x) failed: %v", err)
	}
	childAct, err := NewActivation(map[string]any{"y": 2})
	if err != nil {
		t.Fatalf("NewActivation(y) failed: %v", err)
	}

	tests := []struct {
		name  string
		setup func() (*ExecutionFrame, func())
		want  Activation
	}{
		{
			name: "base frame has no parent activation",
			setup: func() (*ExecutionFrame, func()) {
				f := mustNewExecutionFrame(t, baseAct)
				return f, func() { f.Close() }
			},
			want: nil,
		},
		{
			name: "pushed frame returns parent activation",
			setup: func() (*ExecutionFrame, func()) {
				f := mustNewExecutionFrame(t, baseAct)
				child := f.Push(childAct)
				return child, func() {
					child.Pop()
					f.Close()
				}
			},
			want: baseAct,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			frame, cleanup := tc.setup()
			defer cleanup()

			if got := frame.Parent(); got != tc.want {
				t.Errorf("Parent() got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFrameUnwrap(t *testing.T) {
	baseAct, err := NewActivation(map[string]any{"x": 1})
	if err != nil {
		t.Fatalf("NewActivation(x) failed: %v", err)
	}
	frame := mustNewExecutionFrame(t, baseAct)
	defer frame.Close()

	if got := frame.Unwrap(); got != baseAct {
		t.Errorf("Unwrap() got %v, want %v", got, baseAct)
	}

	childAct, err := NewActivation(map[string]any{"y": 2})
	if err != nil {
		t.Fatalf("NewActivation(y) failed: %v", err)
	}
	childFrame := frame.Push(childAct)
	defer childFrame.Pop()

	if got := childFrame.Unwrap(); got != childFrame.Activation {
		t.Errorf("Unwrap() got %v, want %v", got, childFrame.Activation)
	}
}

func TestFrameAsPartialActivation(t *testing.T) {
	baseAct, err := NewActivation(map[string]any{"x": 1})
	if err != nil {
		t.Fatalf("NewActivation(x) failed: %v", err)
	}
	partAct, err := NewPartialActivation(map[string]any{"y": 2})
	if err != nil {
		t.Fatalf("NewPartialActivation(y) failed: %v", err)
	}

	tests := []struct {
		name      string
		setup     func() *ExecutionFrame
		wantFound bool
	}{
		{
			name: "non-partial activation returns false",
			setup: func() *ExecutionFrame {
				return mustNewExecutionFrame(t, baseAct)
			},
			wantFound: false,
		},
		{
			name: "partial activation returns true",
			setup: func() *ExecutionFrame {
				return mustNewExecutionFrame(t, partAct)
			},
			wantFound: true,
		},
		{
			name: "hierarchical activation wrapping partial activation returns true",
			setup: func() *ExecutionFrame {
				f := mustNewExecutionFrame(t, partAct)
				return f.Push(baseAct)
			},
			wantFound: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			frame := tc.setup()
			defer func() {
				curr := frame
				for curr.parent != nil {
					curr = curr.Pop()
				}
				curr.Close()
			}()

			gotAct, gotFound := frame.AsPartialActivation()
			if gotFound != tc.wantFound {
				t.Errorf("AsPartialActivation() got found=%t, want found=%t", gotFound, tc.wantFound)
			}
			if tc.wantFound && gotAct == nil {
				t.Errorf("AsPartialActivation() returned nil for found partial activation")
			}
		})
	}
}

func TestFramePushPop(t *testing.T) {
	baseAct, err := NewActivation(map[string]any{"x": 1})
	if err != nil {
		t.Fatalf("NewActivation(x) failed: %v", err)
	}
	childAct, err := NewActivation(map[string]any{"y": 2})
	if err != nil {
		t.Fatalf("NewActivation(y) failed: %v", err)
	}

	frame := mustNewExecutionFrame(t, baseAct)
	defer frame.Close()

	childFrame := frame.Push(childAct)
	if childFrame == nil {
		t.Fatal("push() returned nil")
	}
	if childFrame.parent != frame {
		t.Errorf("push() parent got %v, want %v", childFrame.parent, frame)
	}

	popped := childFrame.Pop()
	if popped != frame {
		t.Errorf("pop() got %v, want %v", popped, frame)
	}
}

func TestFrameClose(t *testing.T) {
	frame := mustNewExecutionFrame(t, EmptyActivation())
	ctx := context.Background()
	frame.SetContext(ctx, 1)

	frameCtx := frame.ctx.ctx

	select {
	case <-frameCtx.Done():
		t.Fatal("context canceled before Close()")
	default:
	}

	frame.Close()

	select {
	case <-frameCtx.Done():
	default:
		t.Error("context not canceled after Close()")
	}
}

func TestFrameDoubleClose(t *testing.T) {
	frame := mustNewExecutionFrame(t, EmptyActivation())
	ctx := context.Background()
	frame.SetContext(ctx, 1)

	// Close the frame the first time.
	frame.Close()

	// Closing it again should be a no-op and not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("double Close() panicked: %v", r)
		}
	}()
	frame.Close()
}

func TestFrameLifecycleAndPooling(t *testing.T) {
	vars := map[string]any{"a": 1, "b": 2}
	frame := mustNewExecutionFrame(t, vars)
	val, found := frame.ResolveName("a")
	if !found || val != 1 {
		t.Errorf("ResolveName('a') got %v, %t; want 1, true", val, found)
	}

	// Wrap in a hierarchical activation (e.g. simulating defaultVars setup in program.go)
	parentAct, err := NewActivation(map[string]any{"c": 3})
	if err != nil {
		t.Fatalf("NewActivation failed: %v", err)
	}
	frame.Activation = NewHierarchicalActivation(parentAct, frame.Activation)

	val, found = frame.ResolveName("c")
	if !found || val != 3 {
		t.Errorf("ResolveName('c') got %v, %t; want 3, true", val, found)
	}

	val, found = frame.ResolveName("a")
	if !found || val != 1 {
		t.Errorf("ResolveName('a') got %v, %t; want 1, true", val, found)
	}

	// Close the frame. This should release the pooled evalActivation under the hierarchical activation.
	frame.Close()

	// Verify that we can obtain a clean frame from the pool.
	newFrame := mustNewExecutionFrame(t, map[string]any{"x": 10})
	defer newFrame.Close()
	val, found = newFrame.ResolveName("x")
	if !found || val != 10 {
		t.Errorf("ResolveName('x') got %v, %t; want 10, true", val, found)
	}
	val, found = newFrame.ResolveName("a")
	if found {
		t.Errorf("ResolveName('a') found on fresh frame: %v", val)
	}
}

func TestFrameSetContext(t *testing.T) {
	ctx := context.Background()
	f := mustNewExecutionFrame(t, EmptyActivation())
	defer f.Close()

	if err := f.SetContext(ctx, 1); err != nil {
		t.Errorf("SetContext failed: %v", err)
	}
}

func TestFrameSetContextTwiceError(t *testing.T) {
	ctx := context.Background()
	f := mustNewExecutionFrame(t, EmptyActivation())
	defer f.Close()

	if err := f.SetContext(ctx, 1); err != nil {
		t.Fatalf("SetContext failed first time: %v", err)
	}

	if err := f.SetContext(ctx, 1); err == nil {
		t.Error("expected SetContext to return an error when called twice, got nil")
	}
}

func TestFrameSetContextChildError(t *testing.T) {
	ctx := context.Background()
	f := mustNewExecutionFrame(t, EmptyActivation())
	defer f.Close()

	child := f.Push(EmptyActivation())
	defer child.Pop()

	if err := child.SetContext(ctx, 1); err == nil {
		t.Error("expected SetContext on a child frame to return an error, got nil")
	}
}

func TestNewExecutionFrameInvalidInput(t *testing.T) {
	f, err := NewExecutionFrame(123)
	if err == nil {
		f.Close()
		t.Error("NewExecutionFrame with int input did not return error")
	}
}

func TestFramePopBaseFrame(t *testing.T) {
	f := mustNewExecutionFrame(t, EmptyActivation())
	defer f.Close()

	popped := f.Pop()
	if popped != f {
		t.Errorf("pop() on base frame got %v, want %v", popped, f)
	}
}

func TestLazyVariableResolution(t *testing.T) {
	lazyRefValCalled := 0
	lazyAnyCalled := 0

	vars := map[string]any{
		"lazy_ref": func() ref.Val {
			lazyRefValCalled++
			return types.IntOne
		},
		"lazy_any": func() any {
			lazyAnyCalled++
			return 2
		},
		"normal": 3,
	}

	frame := mustNewExecutionFrame(t, vars)
	defer frame.Close()

	tests := []struct {
		name        string
		lookupName  string
		wantFound   bool
		wantVal     any
		wantRefCall int
		wantAnyCall int
	}{
		{
			name:       "missing variable",
			lookupName: "missing",
			wantFound:  false,
		},
		{
			name:       "normal variable",
			lookupName: "normal",
			wantFound:  true,
			wantVal:    3,
		},
		{
			name:        "lazy ref.Val first call",
			lookupName:  "lazy_ref",
			wantFound:   true,
			wantVal:     types.IntOne,
			wantRefCall: 1,
		},
		{
			name:        "lazy ref.Val second call cached",
			lookupName:  "lazy_ref",
			wantFound:   true,
			wantVal:     types.IntOne,
			wantRefCall: 1,
		},
		{
			name:        "lazy any first call",
			lookupName:  "lazy_any",
			wantFound:   true,
			wantVal:     2,
			wantRefCall: 1,
			wantAnyCall: 1,
		},
		{
			name:        "lazy any second call cached",
			lookupName:  "lazy_any",
			wantFound:   true,
			wantVal:     2,
			wantRefCall: 1,
			wantAnyCall: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			val, found := frame.ResolveName(tc.lookupName)
			if found != tc.wantFound {
				t.Fatalf("ResolveName(%q) found = %t, want = %t", tc.lookupName, found, tc.wantFound)
			}
			if found {
				if val != tc.wantVal {
					t.Errorf("ResolveName(%q) value = %v, want = %v", tc.lookupName, val, tc.wantVal)
				}
			}
			if lazyRefValCalled != tc.wantRefCall {
				t.Errorf("lazyRefValCalled = %d, want = %d", lazyRefValCalled, tc.wantRefCall)
			}
			if lazyAnyCalled != tc.wantAnyCall {
				t.Errorf("lazyAnyCalled = %d, want = %d", lazyAnyCalled, tc.wantAnyCall)
			}
		})
	}
}

func mustNewExecutionFrame(t testing.TB, input any) *ExecutionFrame {
	t.Helper()
	f, err := NewExecutionFrame(input)
	if err != nil {
		t.Fatalf("NewExecutionFrame() failed: %v", err)
	}
	return f
}
