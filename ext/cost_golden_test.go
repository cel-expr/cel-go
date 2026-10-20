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
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"sync"
	"testing"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/cost"
)

// costGoldenV0File holds the estimate and the runtime cost which cel-go v0.32.0, the last release
// before cost model versions, reported for each cost test in this package, keyed by test name.
// Cost model version 0 exists to reproduce those numbers, so every cost test which ran in v0.32.0
// replays its expression at version 0 and must match.
//
// The file was recorded by running the v0.32.0 tests with their cost helpers instrumented to log
// each estimate and actual cost under t.Name(). It must not be regenerated from this branch.
const costGoldenV0File = "testdata/cost_v0.32.0.json"

type costGolden struct {
	Name      string      `json:"name"`
	Estimates [][2]uint64 `json:"estimates"`
	Actuals   []uint64    `json:"actuals"`
}

// costGoldenV0Accepted lists the deliberate departures from v0.32.0 at version 0, keyed by test
// name and then by the kind of cost, "estimate" or "actual cost". These fixed a cost which let an
// unbounded operation run for less than it costs, so version 0 does not restore them.
var costGoldenV0Accepted = map[string]map[string]string{
	"TestEncodersCosts/json_encode_dyn": {
		"actual cost": "json.encode traverses its argument without the cost tracker observing it; " +
			"v0.32.0 charged 1, and #1430 charges math.MaxUint64 at every version",
	},
}

// costGoldenV0Retired lists release tests which no longer exist under the same name, with the
// reason, so that the coverage check in TestMain accounts for every recorded test.
var costGoldenV0Retired = map[string]string{}

var (
	loadCostGoldenV0 = sync.OnceValues(func() (map[string]costGolden, error) {
		data, err := os.ReadFile(costGoldenV0File)
		if err != nil {
			return nil, err
		}
		var entries []costGolden
		if err := json.Unmarshal(data, &entries); err != nil {
			return nil, err
		}
		goldens := make(map[string]costGolden, len(entries))
		for _, e := range entries {
			goldens[e.Name] = e
		}
		return goldens, nil
	})

	costGoldenV0Mu       sync.Mutex
	costGoldenV0Replayed = map[string]bool{}
)

func costGoldenV0(t *testing.T) (costGolden, bool) {
	t.Helper()
	goldens, err := loadCostGoldenV0()
	if err != nil {
		t.Fatalf("loading %s: %v", costGoldenV0File, err)
	}
	g, found := goldens[t.Name()]
	if found {
		costGoldenV0Mu.Lock()
		costGoldenV0Replayed[t.Name()] = true
		costGoldenV0Mu.Unlock()
	}
	return g, found
}

// reportCostGoldenV0 compares a version 0 cost of the given kind with the one v0.32.0 reported.
//
// A difference fails the test unless costGoldenV0Accepted explains it. A listed difference which
// no longer occurs also fails, so that the ledger only ever shrinks to what is still true.
func reportCostGoldenV0(t *testing.T, kind string, got, want any) {
	t.Helper()
	reason, listed := costGoldenV0Accepted[t.Name()][kind]
	switch {
	case got == want && listed:
		t.Errorf("%s at version 0 matches v0.32.0 (%v); remove %q from costGoldenV0Accepted", kind, got, t.Name())
	case got != want && listed:
		t.Logf("listed difference (%s): %s at version 0 is %v, v0.32.0 reported %v", reason, kind, got, want)
	case got != want:
		t.Errorf("%s at version 0 is %v, v0.32.0 reported %v", kind, got, want)
	}
}

// checkCostGoldenV0Estimate estimates the checked ast at cost model version 0 with the estimator the
// release test used, and compares the estimate with the one v0.32.0 reported.
func checkCostGoldenV0Estimate(t *testing.T, env *cel.Env, ast *cel.Ast, estimator cost.Estimator) {
	t.Helper()
	g, found := costGoldenV0(t)
	if !found || len(g.Estimates) == 0 {
		return
	}
	est, err := env.EstimateCost(ast, estimator, cost.ModelVersion(0))
	if err != nil {
		t.Fatalf("env.EstimateCost(version 0) failed: %v", err)
	}
	reportCostGoldenV0(t, "estimate", [2]uint64{est.Min, est.Max}, g.Estimates[0])
}

// checkCostGoldenV0Actual evaluates the checked ast with cost tracking at cost model version 0, and
// compares the cost with the one v0.32.0 charged.
func checkCostGoldenV0Actual(t *testing.T, env *cel.Env, ast *cel.Ast, in any) {
	t.Helper()
	g, found := costGoldenV0(t)
	if !found || len(g.Actuals) == 0 || !ast.IsChecked() {
		return
	}
	envV0, err := env.Extend(cel.CostModelVersion(0))
	if err != nil {
		t.Fatalf("env.Extend(CostModelVersion(0)) failed: %v", err)
	}
	prg, err := envV0.Program(ast, cel.CostTracking(nil))
	if err != nil {
		t.Fatalf("env.Program(version 0) failed: %v", err)
	}
	if in == nil {
		in = cel.NoVars()
	}
	_, det, _ := prg.Eval(in)
	if det == nil || det.ActualCost() == nil {
		t.Fatalf("prg.Eval(version 0) reported no cost")
	}
	reportCostGoldenV0(t, "actual cost", *det.ActualCost(), g.Actuals[0])
}

// TestMain checks, after a full run of the package, that every test recorded in the golden file
// was replayed or is retired, so that renaming or deleting a release test cannot silently drop its
// version 0 baseline.
func TestMain(m *testing.M) {
	code := m.Run()
	if code == 0 && flag.Lookup("test.run").Value.String() == "" && !testing.Short() {
		if missing := missingCostGoldensV0(); len(missing) > 0 {
			fmt.Fprintf(os.Stderr, "--- FAIL: %d v0.32.0 cost tests were not replayed at version 0 "+
				"(rename them back, or list them in costGoldenV0Retired):\n", len(missing))
			for _, name := range missing {
				fmt.Fprintf(os.Stderr, "    %s\n", name)
			}
			code = 1
		}
	}
	os.Exit(code)
}

func missingCostGoldensV0() []string {
	goldens, err := loadCostGoldenV0()
	if err != nil {
		return []string{err.Error()}
	}
	var missing []string
	for name := range goldens {
		if !costGoldenV0Replayed[name] && costGoldenV0Retired[name] == "" {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}
