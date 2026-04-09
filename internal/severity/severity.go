// Package severity classifies change events into info/warn/alert levels.
//
// A Rule decides whether a given event should be elevated beyond the default
// "info" level. Rules are composed into a Classifier; the highest severity
// that applies wins. Adding a new rule is a single-file change — rules live
// in subpackages or files next to this one, and are registered via DefaultRules.
//
// The classifier is deliberately stateless and deterministic: given the same
// input, it always produces the same output. Any history-aware logic (e.g.
// "this record was re-added after being removed") needs to be computed in a
// RuleContext passed explicitly.
package severity

import (
	"context"

	"github.com/gmowses/fepublica/internal/store"
)

// Level values. Keep in sync with the CHECK constraint on change_events.severity.
const (
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelAlert = "alert"
)

// RuleContext carries extra information a rule may need beyond the event itself.
// Rules that don't need history just ignore it.
type RuleContext struct {
	// Lookup lets a rule query the store for related events (e.g. "did this
	// external_id disappear and come back?"). Keep these queries cheap.
	Store *store.Store
	Ctx   context.Context
}

// Rule is implemented by anything that can inspect a change_event and decide
// whether to escalate its severity.
type Rule interface {
	// Name is a short, stable identifier used in logs and metrics.
	Name() string
	// Evaluate returns the severity level this rule wants to assign, plus a
	// boolean indicating whether the rule even applies. If Applies is false,
	// the returned level is ignored.
	Evaluate(event *store.ChangeEvent, rc RuleContext) (level string, applies bool, reason string)
}

// Classifier runs a set of Rules and returns the highest severity any rule
// assigns. Rules are evaluated in declaration order; ties prefer later rules
// so newer/more-specific rules can override older/generic ones.
type Classifier struct {
	rules []Rule
}

// New creates a Classifier from the given rules.
func New(rules ...Rule) *Classifier {
	return &Classifier{rules: rules}
}

// Classify returns the highest severity level applicable to the event and the
// name of the rule that produced it (empty if nothing escalated beyond info).
func (c *Classifier) Classify(event *store.ChangeEvent, rc RuleContext) (level string, ruleName string, reason string) {
	level = LevelInfo
	for _, r := range c.rules {
		l, ok, why := r.Evaluate(event, rc)
		if !ok {
			continue
		}
		if levelRank(l) > levelRank(level) {
			level = l
			ruleName = r.Name()
			reason = why
		}
	}
	return level, ruleName, reason
}

func levelRank(l string) int {
	switch l {
	case LevelAlert:
		return 2
	case LevelWarn:
		return 1
	default:
		return 0
	}
}

// DefaultRules returns the built-in rule set shipped with the Observatório.
// Callers can extend or replace it.
func DefaultRules() []Rule {
	return []Rule{
		ceisSilentRemovalRule{},
		pncpBigValueRule{},
	}
}

// ceisSilentRemovalRule: any 'removed' event on CEIS/CNEP is suspicious enough
// to be a warn — a sanctioned company being removed from the inidoneous list
// deserves attention.
type ceisSilentRemovalRule struct{}

func (ceisSilentRemovalRule) Name() string { return "ceis-silent-removal" }
func (ceisSilentRemovalRule) Evaluate(event *store.ChangeEvent, _ RuleContext) (string, bool, string) {
	if event.ChangeType != "removed" {
		return "", false, ""
	}
	if event.SourceID != "ceis" && event.SourceID != "cnep" {
		return "", false, ""
	}
	return LevelWarn, true, "remoção em cadastro de sanções é sempre digna de atenção"
}

// pncpBigValueRule: modification on a PNCP contract (for now we classify all
// modifications on pncp-contratos as warn until we have a value-delta parser).
type pncpBigValueRule struct{}

func (pncpBigValueRule) Name() string { return "pncp-modification" }
func (pncpBigValueRule) Evaluate(event *store.ChangeEvent, _ RuleContext) (string, bool, string) {
	if event.ChangeType != "modified" {
		return "", false, ""
	}
	if event.SourceID != "pncp-contratos" {
		return "", false, ""
	}
	return LevelWarn, true, "alteração em contrato público merece revisão"
}
