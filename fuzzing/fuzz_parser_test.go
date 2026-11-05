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

package fuzzing

import (
	"strings"
	"testing"
)

func FuzzParser(f *testing.F) {
	f.Add([]byte("1 + 2"))
	f.Add([]byte("a.b.c == 'test'"))
	f.Add([]byte(strings.Repeat("\n", 1000) + "[0.exists_one(x, 1)]"))
	f.Add([]byte("a" + strings.Repeat(".a", 1000) + "{}"))
	f.Add([]byte(`"\n` + strings.Repeat("\u00e9", 1000) + `"`))
	env, err := getCELFuzzEnv()
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, in []byte) {
		_, _ = env.Parse(string(in))
	})
}

func BenchmarkFuzzParserIter(b *testing.B) {
	in := []byte("a.b.c == 'test'")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		env, err := getCELFuzzEnv()
		if err != nil {
			b.Fatal(err)
		}
		_, _ = env.Parse(string(in))
	}
}
