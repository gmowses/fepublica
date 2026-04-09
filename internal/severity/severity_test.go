package severity

import (
	"testing"

	"github.com/gmowses/fepublica/internal/store"
)

func TestClassify_DefaultInfo(t *testing.T) {
	c := New(DefaultRules()...)
	ev := &store.ChangeEvent{
		SourceID:   "ceis",
		ChangeType: "added",
	}
	level, rule, _ := c.Classify(ev, RuleContext{})
	if level != LevelInfo {
		t.Fatalf("expected info, got %s (rule %s)", level, rule)
	}
}

func TestClassify_CEISRemovedIsWarn(t *testing.T) {
	c := New(DefaultRules()...)
	ev := &store.ChangeEvent{
		SourceID:   "ceis",
		ChangeType: "removed",
	}
	level, rule, _ := c.Classify(ev, RuleContext{})
	if level != LevelWarn {
		t.Fatalf("expected warn, got %s", level)
	}
	if rule != "ceis-silent-removal" {
		t.Fatalf("expected ceis-silent-removal rule, got %s", rule)
	}
}

func TestClassify_CNEPRemovedIsWarn(t *testing.T) {
	c := New(DefaultRules()...)
	ev := &store.ChangeEvent{
		SourceID:   "cnep",
		ChangeType: "removed",
	}
	level, _, _ := c.Classify(ev, RuleContext{})
	if level != LevelWarn {
		t.Fatalf("expected warn for cnep removed, got %s", level)
	}
}

func TestClassify_PNCPModifiedIsWarn(t *testing.T) {
	c := New(DefaultRules()...)
	ev := &store.ChangeEvent{
		SourceID:   "pncp-contratos",
		ChangeType: "modified",
	}
	level, _, _ := c.Classify(ev, RuleContext{})
	if level != LevelWarn {
		t.Fatalf("expected warn, got %s", level)
	}
}

func TestClassify_CustomAlertRuleWinsOverWarn(t *testing.T) {
	custom := fakeRule{name: "custom-alert", level: LevelAlert, applies: true}
	c := New(append(DefaultRules(), custom)...)
	ev := &store.ChangeEvent{
		SourceID:   "ceis",
		ChangeType: "removed",
	}
	level, rule, _ := c.Classify(ev, RuleContext{})
	if level != LevelAlert {
		t.Fatalf("expected alert, got %s", level)
	}
	if rule != "custom-alert" {
		t.Fatalf("expected custom-alert, got %s", rule)
	}
}

func TestLevelRank(t *testing.T) {
	if levelRank(LevelAlert) <= levelRank(LevelWarn) {
		t.Fatal("alert must rank higher than warn")
	}
	if levelRank(LevelWarn) <= levelRank(LevelInfo) {
		t.Fatal("warn must rank higher than info")
	}
}

type fakeRule struct {
	name    string
	level   string
	applies bool
}

func (f fakeRule) Name() string { return f.name }
func (f fakeRule) Evaluate(*store.ChangeEvent, RuleContext) (string, bool, string) {
	return f.level, f.applies, "fake"
}
