package rules

import (
	"fmt"
	"slices"
	"time"
)

// Engine holds loaded rule sets keyed by visa code.
type Engine struct {
	ruleSets map[string]*VisaRuleSet
}

// NewEngine constructs an engine from already-loaded rule sets.
func NewEngine(ruleSets []*VisaRuleSet) *Engine {
	m := make(map[string]*VisaRuleSet, len(ruleSets))
	for _, rs := range ruleSets {
		m[rs.VisaCode] = rs
	}
	return &Engine{ruleSets: m}
}

// VisaCodes returns the set of visa codes the engine knows about.
func (e *Engine) VisaCodes() []string {
	codes := make([]string, 0, len(e.ruleSets))
	for code := range e.ruleSets {
		codes = append(codes, code)
	}
	slices.Sort(codes)
	return codes
}

// Evaluate returns the tasks a given event generates for a user on a given visa.
// Pure function: no I/O, no time.Now(), all inputs explicit.
//
// The engine handles visa-scoped rules (matched by visa code) and wildcard rules
// (applies_to_visas: ['*']) in a single pass. Wildcard rules are evaluated against
// any visa code the engine knows about, so e.g. tax-residency obligations fire
// regardless of which visa the user holds.
func (e *Engine) Evaluate(in EvaluationInput) ([]GeneratedTask, error) {
	rs, ok := e.ruleSets[in.VisaCode]
	if !ok {
		return nil, fmt.Errorf("no rule set loaded for visa code %q (known: %v)", in.VisaCode, e.VisaCodes())
	}

	var generated []GeneratedTask

	// Visa-scoped rules from this visa's rule set.
	for _, rule := range rs.Rules {
		if !ruleApplies(rule, in.VisaCode, in.EventType) {
			continue
		}
		for _, tpl := range rule.GeneratesTasks {
			task, err := materializeTask(tpl, in.OccurredAt)
			if err != nil {
				return nil, fmt.Errorf("rule %s: %w", rule.ID, err)
			}
			generated = append(generated, task)
		}
	}

	// Wildcard rules from OTHER visa rule sets that target this event.
	// (Wildcard rules in the user's own rule set were already covered above.)
	for code, otherRS := range e.ruleSets {
		if code == in.VisaCode {
			continue
		}
		for _, rule := range otherRS.Rules {
			if rule.Trigger != in.EventType {
				continue
			}
			if !slices.Contains(rule.AppliesToVisas, "*") {
				continue
			}
			for _, tpl := range rule.GeneratesTasks {
				task, err := materializeTask(tpl, in.OccurredAt)
				if err != nil {
					return nil, fmt.Errorf("rule %s: %w", rule.ID, err)
				}
				generated = append(generated, task)
			}
		}
	}

	return generated, nil
}

func ruleApplies(rule Rule, visaCode string, eventType EventType) bool {
	if rule.Trigger != eventType {
		return false
	}
	if slices.Contains(rule.AppliesToVisas, "*") {
		return true
	}
	return slices.Contains(rule.AppliesToVisas, visaCode)
}

func materializeTask(tpl TaskTemplate, triggerTime time.Time) (GeneratedTask, error) {
	deadline, err := computeDeadline(tpl.Deadline, triggerTime)
	if err != nil {
		return GeneratedTask{}, err
	}
	return GeneratedTask{
		RuleID:          tpl.RuleID,
		TitleEN:         tpl.TitleEN,
		TitleJA:         tpl.TitleJA,
		DescriptionEN:   tpl.DescriptionEN,
		DescriptionJA:   tpl.DescriptionJA,
		Category:        tpl.Category,
		Severity:        tpl.Severity,
		DeadlineAt:      deadline,
		LegalSourceURL:  tpl.LegalSourceURL,
		LegalSourceText: tpl.LegalSourceText,
		LocationHint:    tpl.LocationHint,
		DependsOnRules:  tpl.DependsOn,
	}, nil
}

func computeDeadline(spec DeadlineSpec, triggerTime time.Time) (*time.Time, error) {
	set := 0
	if spec.RelativeDays != nil {
		set++
	}
	if spec.AbsoluteDate != nil {
		set++
	}
	if spec.NoDeadline {
		set++
	}
	if set != 1 {
		return nil, fmt.Errorf("invalid deadline spec: must set exactly one of relative_days, absolute_date, no_deadline (got %d set)", set)
	}

	switch {
	case spec.NoDeadline:
		return nil, nil
	case spec.RelativeDays != nil:
		t := triggerTime.AddDate(0, 0, *spec.RelativeDays)
		return &t, nil
	case spec.AbsoluteDate != nil:
		t := spec.AbsoluteDate.Time
		return &t, nil
	}
	// unreachable
	return nil, fmt.Errorf("unreachable")
}
