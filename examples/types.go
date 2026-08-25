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

// Package examples provides example types and functions used in examples.
package examples

// User is a sample user struct with fields tagged for JSON serialization.
type User struct {
	Name  string   `json:"name"`
	Age   int      `json:"age"`
	Roles []string `json:"roles"`
}

// Account is a sample account struct with fields tagged for CEL field access.
type Account struct {
	ID        int64  `cel:"id"`
	OwnerName string `cel:"owner"`
}
