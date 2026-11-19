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

package ext

import (
	"testing"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/cost"
)

// TestCostSoundnessBySizingStrategy checks that, at cost model version 1, the tracked cost of an
// expression lies within its estimate under each built-in sizing strategy. The expressions build
// containers whose sizes extension estimators synthesize from integers or string lengths, and so
// must report in the units of the sizing strategy, or depend on laziness and optional elements.
func TestCostSoundnessBySizingStrategy(t *testing.T) {
	exprs := []string{
		// Slices, ranges, flattens, splits and matches whose results are then scanned. Apart from
		// flatten(depth), whose estimator follows the sizing strategy itself, these are priced by
		// their models from model version 1.
		`'x' in ['abcdefgh', 'ijklmnop', 'q', 'r'].slice(0, 2)`,
		`'x' in ['abcdefgh', 'ijklmnop', 'q', 'r'].slice(1, 4)`,
		`size(['abcdefgh', 'ijklmnop'].slice(0, 1)) == 1`,
		`7 in lists.range(5)`,
		`'x' in [['abcdefgh', 'ijklmnop'], ['qrstuvwx']].flatten()`,
		`'x' in [['abcdefgh', 'ijklmnop'], ['qrstuvwx']].flatten(1)`,
		`'x' in [[['abcdefgh'], ['ijklmnop']], [['qrstuvwx']]].flatten(2)`,
		`size([['abcdefgh', 'ijklmnop'], ['qrstuvwx']].flatten(1)) == 3`,
		`'x' in 'banana bandana'.split('a')`,
		`'x' in 'banana bandana'.split('')`,
		`'x' in regex.extractAll('a1b22c333d4444', '[0-9]+')`,
		// String transforms over multi-byte text, whose sizes differ by strategy.
		`'héllo wörld ñandú'.reverse().contains('x')`,
		`'héllo wörld ñandú'.upperAscii().contains('x')`,
		`'héllo wörld ñandú'.lowerAscii().contains('x')`,
		// Other list functions.
		`'x' in ['ccccccc', 'bbbbbbb', 'a'].sort()`,
		`'x' in ['ccccccc', 'bbbbbbb', 'a'].sortBy(e, size(e))`,
		`'x' in ['ccccccc', 'ccccccc', 'a'].distinct()`,
		`'x' in [3, 1, 2].distinct().map(i, string(i))`,
		`'x' in ['ccccccc', 'bbbbbbb', 'a'].reverse()`,
		`[1, 2, 3].hasAll([1, 2])`,
		`['abc', 'déf'].hasExactly(['déf', 'abc'])`,
		`sets.contains(['abc', 'déf'], ['déf'])`,
		`math.greatest([1, 5, 3]) == 5`,
		// putAll over keys which collide, so that the result holds fewer entries than its inputs
		// and a replaced value may be smaller than the one it replaces, and over list values.
		`{'a': 'abcdefgh', 'b': 'ijklmnop'}.putAll({'b': 'q', 'c': 'r'}).exists(k, k == 'x')`,
		`{'a': 'abcdefgh'}.putAll({'a': ''}).all(k, k == 'a')`,
		`{'a': 'abcdefgh'}.putAll({'a': ''})['a'].contains('x')`,
		`'x' in {'a': ['abc', 'def']}.putAll({'b': ['ghi']})['a']`,
		// Other string functions over multi-byte text.
		`'héllo wörld ñandú'.substring(2, 10).contains('x')`,
		`'héllo wörld ñandú'.substring(5).contains('x')`,
		`'héllo wörld ñandú'.charAt(1).contains('x')`,
		`'héllo wörld ñandú'.replace('o', 'öööö').contains('x')`,
		`'  héllo  '.trim().contains('x')`,
		`'héllo wörld'.indexOf('w') > 0`,
		`'héllo wörld'.lastIndexOf('ö') > 0`,
		`['héllo', 'wörld'].join(', ').contains('x')`,
		`['héllo', 'wörld'].join().contains('x')`,
		// Regex and encoders.
		`regex.extract('héllo wörld', 'w.+').orValue('').contains('x')`,
		`regex.replace('héllo wörld', 'o', 'ööö').contains('x')`,
		`base64.encode(b'h\xc3\xa9llo w\xc3\xb6rld').contains('x')`,
		`base64.decode('aGVsbG8gd29ybGQ=').size() > 0`,
		// A lazily bound value which the result does not read.
		`cel.bind(x, ['abc', 'defg'].map(y, y + y), true)`,
		// A lazily bound value which the result reads.
		`cel.bind(x, ['abc', 'defg'].map(y, y + y), 'x' in x)`,
		// Optional elements which are absent.
		`'x' in [?optional.none(), ?optional.none()]`,
		`'x' in [?optional.none(), 'abc']`,
		`'x' in [?optional.of('abcdefgh'), ?optional.none()]`,
		`'x' in {?'a': optional.none(), 'b': 'c'}`,
		// Results which may be empty strings, which the aggregate strategy sizes as 1.
		`'abc'.substring(3) == 'abc'`,
		`'abc'.substring(1, 1) == 'abc'`,
		`[].join() == 'abcdefghij'`,
		// flatten(depth) over list(dyn): values below a non-list level, deeper levels, and
		// optional entries.
		`['aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', ['a']].flatten(1).exists(e, e.contains('bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'))`,
		`[[[1, 2, 3], [4, 5, 6]]].flatten(2).size() == 6`,
		`'x' in ['', ''].flatten(1)`,
		`[[?optional.none(), 1]].flatten(1).size() == 1`,
		`'x' in [[], [], []].flatten(lists.range(1).size() - 1)`,
		// A presence test on a conditional whose selected branch is computed.
		`has((true ? {} : {}).a)`,
		`has((true ? {'a': {'b': 1}} : {}).a.b)`,
		// Comparisons of optionals, which are sized as the values they hold.
		`optional.of('') == optional.of('')`,
		`optional.of('aaaaaaaaaa') == optional.of('aaaaaaaaaa')`,
		`{'a': ''}[?'a'] == optional.of('')`,
	}
	strategies := map[string]cost.SizingStrategy{
		"default":   cost.DefaultSizingStrategy(),
		"aggregate": cost.AggregateSizingStrategy(),
	}
	for name, strategy := range strategies {
		env, err := cel.NewEnv(
			cel.OptionalTypes(), Lists(), Maps(), Strings(), Regex(), Bindings(), Sets(), Math(), Encoders(),
			cel.CostSizingStrategy(strategy),
			cel.CostModelVersion(1),
		)
		if err != nil {
			t.Fatalf("cel.NewEnv() failed: %v", err)
		}
		for _, expr := range exprs {
			t.Run(name+"/"+expr, func(t *testing.T) {
				ast, iss := env.Compile(expr)
				if iss.Err() != nil {
					t.Fatalf("env.Compile(%q) failed: %v", expr, iss.Err())
				}
				est, err := env.EstimateCost(ast, &noopCostEstimator{})
				if err != nil {
					t.Fatalf("env.EstimateCost() failed: %v", err)
				}
				prg, err := env.Program(ast, cel.CostTracking(nil))
				if err != nil {
					t.Fatalf("env.Program() failed: %v", err)
				}
				_, det, err := prg.Eval(cel.NoVars())
				if err != nil {
					t.Fatalf("prg.Eval() failed: %v", err)
				}
				actual := *det.ActualCost()
				t.Logf("estimate [%d, %d], actual %d", est.Min, est.Max, actual)
				if actual < est.Min || actual > est.Max {
					t.Errorf("actual cost %d outside estimate [%d, %d]", actual, est.Min, est.Max)
				}
			})
		}
	}
}
