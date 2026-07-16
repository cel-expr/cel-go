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

package test

import (
	"context"
	"time"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// FakeRPC returns a blocking async function which simulates an RPC that succeeds after a short
// delay, or fails if the provided per-call timeout elapses first.
func FakeRPC(timeout time.Duration) func(context.Context, ...ref.Val) ref.Val {
	return func(ctx context.Context, args ...ref.Val) ref.Val {
		rpcCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		select {
		case <-time.After(20 * time.Millisecond):
			in := args[0].(types.String)
			return in.Add(types.String(" success!"))
		case <-rpcCtx.Done():
			return types.NewErr(rpcCtx.Err().Error())
		}
	}
}
