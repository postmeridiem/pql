package migrate

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// step builds a Step that appends its id to log when applied.
func step(to int, id string, log *[]string) Step {
	return Step{To: to, ID: id, Apply: func(context.Context) error {
		*log = append(*log, id)
		return nil
	}}
}

func TestPlan_UpToDateIsNoWork(t *testing.T) {
	var log []string
	plan, err := Plan(Axis{Name: "test", Current: 2, Found: 2, Steps: []Step{step(2, "a", &log)}})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan) != 0 {
		t.Errorf("plan = %v, want empty for an up-to-date axis", plan)
	}
}

func TestPlan_AppliesStepsInOrder(t *testing.T) {
	var log []string
	// Deliberately out of order: an axis should not have to keep its slice sorted.
	steps := []Step{step(3, "third", &log), step(1, "first", &log), step(2, "second", &log)}

	applied, err := Run(context.Background(), Axis{Name: "test", Current: 3, Found: 0, Steps: steps}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := strings.Join(log, ","); got != "first,second,third" {
		t.Errorf("applied %q, want first,second,third", got)
	}
	if len(applied) != 3 || applied[2].To != 3 {
		t.Errorf("applied = %+v, want three steps ending at version 3", applied)
	}
}

func TestPlan_SkipsStepsAlreadyPassed(t *testing.T) {
	var log []string
	steps := []Step{step(1, "first", &log), step(2, "second", &log), step(3, "third", &log)}

	if _, err := Run(context.Background(), Axis{Name: "test", Current: 3, Found: 2, Steps: steps}, nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := strings.Join(log, ","); got != "third" {
		t.Errorf("applied %q, want only third — the artefact was already at 2", got)
	}
}

// An artefact written by a newer pql cannot be reasoned about: there is no
// backward step and no way to know what the newer version encoded.
func TestPlan_RefusesArtefactAheadOfBinary(t *testing.T) {
	_, err := Plan(Axis{Name: "changelog format", Current: 2, Found: 5})
	if err == nil {
		t.Fatal("expected a refusal for an artefact ahead of the binary")
	}
	for _, want := range []string{"changelog format", "5", "2", "upgrade pql"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q, got: %v", want, err)
		}
	}
}

// Applying a partial chain would strand the artefact at an intermediate
// version, which is worse than refusing: the next run would see a version
// nothing describes.
func TestPlan_RefusesOnGap(t *testing.T) {
	var log []string
	steps := []Step{step(1, "first", &log), step(3, "third", &log)} // 2 missing

	_, err := Plan(Axis{Name: "test", Current: 3, Found: 0, Steps: steps})
	if err == nil {
		t.Fatal("expected a refusal when the steps do not chain to Current")
	}
	if !strings.Contains(err.Error(), "past version 1") {
		t.Errorf("error should name the version actually reachable, got: %v", err)
	}
	if len(log) != 0 {
		t.Errorf("no step should have run on a refused plan, ran %v", log)
	}
}

func TestPlan_GapErrorCarriesRecoveryHint(t *testing.T) {
	_, err := Plan(Axis{
		Name: "pql.db schema", Current: 2, Found: 1,
		Recovery: "delete pql.db and run pql plan rebuild",
	})
	if err == nil {
		t.Fatal("expected a refusal")
	}
	if !strings.Contains(err.Error(), "pql plan rebuild") {
		t.Errorf("error should carry the axis recovery hint, got: %v", err)
	}
}

func TestRun_ReportsPartialProgressOnFailure(t *testing.T) {
	var log []string
	boom := errors.New("boom")
	steps := []Step{
		step(1, "first", &log),
		{To: 2, ID: "second", Apply: func(context.Context) error { return boom }},
		step(3, "third", &log),
	}

	applied, err := Run(context.Background(), Axis{Name: "test", Current: 3, Found: 0, Steps: steps}, nil)
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want the step's own error wrapped", err)
	}
	if len(applied) != 1 || applied[0].ID != "first" {
		t.Errorf("applied = %+v, want just the step that succeeded", applied)
	}
	if !strings.Contains(err.Error(), "second") {
		t.Errorf("error should name the failing step, got: %v", err)
	}
}

// The ledger callback runs inside the pass, per step — that is what lets an
// interrupted run resume instead of guessing how far it got.
func TestRun_RecordsEachStepAsItLands(t *testing.T) {
	var log, recorded []string
	steps := []Step{step(1, "first", &log), step(2, "second", &log)}

	_, err := Run(context.Background(), Axis{Name: "test", Current: 2, Found: 0, Steps: steps},
		func(s Step) error {
			recorded = append(recorded, s.ID)
			return nil
		})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Join(recorded, ",") != "first,second" {
		t.Errorf("recorded %v, want each step as it landed", recorded)
	}
}

func TestRun_RecordingFailureStopsTheRun(t *testing.T) {
	var log []string
	steps := []Step{step(1, "first", &log), step(2, "second", &log)}

	_, err := Run(context.Background(), Axis{Name: "test", Current: 2, Found: 0, Steps: steps},
		func(Step) error { return errors.New("ledger write failed") })
	if err == nil {
		t.Fatal("expected the run to stop when progress cannot be recorded")
	}
	if len(log) != 1 {
		t.Errorf("ran %v, want the run to stop after the first unrecordable step", log)
	}
}
