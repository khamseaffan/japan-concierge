package rules

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed data/*.yaml
var ruleData embed.FS

// LoadAll reads all YAML rule files from the embedded filesystem, parses them,
// and validates cross-references between rules.
//
// Validation performed:
//   - visa_code must be set
//   - every rule.id must be unique within its rule set
//   - every TaskTemplate.rule_id must be globally unique across all rule sets
//   - DeadlineSpec must set exactly one of relative_days, absolute_date, no_deadline
//   - every TaskTemplate.depends_on entry must reference a rule_id that exists
//     in some loaded rule set (not necessarily the same one)
//
// This catches the classic class of bug where a rule references a depends_on
// rule_id that was renamed or never existed — a silent runtime failure mode
// otherwise.
func LoadAll() ([]*VisaRuleSet, error) {
	entries, err := fs.ReadDir(ruleData, "data")
	if err != nil {
		return nil, fmt.Errorf("read rules dir: %w", err)
	}

	var sets []*VisaRuleSet
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".yaml") && !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		raw, err := ruleData.ReadFile("data/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		var rs VisaRuleSet
		if err := yaml.Unmarshal(raw, &rs); err != nil {
			return nil, fmt.Errorf("parse %s: %w", e.Name(), err)
		}
		if rs.VisaCode == "" {
			return nil, fmt.Errorf("%s: missing visa_code", e.Name())
		}
		sets = append(sets, &rs)
	}

	// Stable order so callers (and tests) get deterministic results.
	sort.Slice(sets, func(i, j int) bool { return sets[i].VisaCode < sets[j].VisaCode })

	if err := validateRuleSets(sets); err != nil {
		return nil, err
	}
	return sets, nil
}

// validateRuleSets cross-checks the loaded rule sets and returns the first
// problem found, or nil if everything is well-formed.
func validateRuleSets(sets []*VisaRuleSet) error {
	allTaskRuleIDs := make(map[string]string) // task rule_id -> "visa_code/rule_id" for error context

	for _, rs := range sets {
		ruleIDs := make(map[string]bool)
		for _, rule := range rs.Rules {
			if rule.ID == "" {
				return fmt.Errorf("%s: rule with empty id", rs.VisaCode)
			}
			if ruleIDs[rule.ID] {
				return fmt.Errorf("%s: duplicate rule id %q", rs.VisaCode, rule.ID)
			}
			ruleIDs[rule.ID] = true

			if rule.Trigger == "" {
				return fmt.Errorf("%s/%s: missing trigger", rs.VisaCode, rule.ID)
			}
			if !IsKnownEventType(string(rule.Trigger)) {
				return fmt.Errorf("%s/%s: trigger %q is not a registered EventType", rs.VisaCode, rule.ID, rule.Trigger)
			}
			if len(rule.AppliesToVisas) == 0 {
				return fmt.Errorf("%s/%s: applies_to_visas must not be empty", rs.VisaCode, rule.ID)
			}

			for i, tpl := range rule.GeneratesTasks {
				if tpl.RuleID == "" {
					return fmt.Errorf("%s/%s: task %d has empty rule_id", rs.VisaCode, rule.ID, i)
				}
				if existing, ok := allTaskRuleIDs[tpl.RuleID]; ok {
					return fmt.Errorf("duplicate task rule_id %q (defined in %s and %s/%s)",
						tpl.RuleID, existing, rs.VisaCode, rule.ID)
				}
				allTaskRuleIDs[tpl.RuleID] = rs.VisaCode + "/" + rule.ID

				if tpl.TitleEN == "" {
					return fmt.Errorf("%s/%s/%s: missing title_en", rs.VisaCode, rule.ID, tpl.RuleID)
				}
				if tpl.Category == "" {
					return fmt.Errorf("%s/%s/%s: missing category", rs.VisaCode, rule.ID, tpl.RuleID)
				}
				if tpl.Severity == "" {
					return fmt.Errorf("%s/%s/%s: missing severity", rs.VisaCode, rule.ID, tpl.RuleID)
				}

				// Deadline well-formed (exactly one of three set). The trigger
				// time we pass is irrelevant — computeDeadline only inspects
				// the resulting date when the spec is valid, and we discard
				// the result here.
				if _, err := computeDeadline(tpl.Deadline, time.Time{}); err != nil {
					return fmt.Errorf("%s/%s/%s: %w", rs.VisaCode, rule.ID, tpl.RuleID, err)
				}
			}
		}
	}

	// Second pass: depends_on references must resolve.
	for _, rs := range sets {
		for _, rule := range rs.Rules {
			for _, tpl := range rule.GeneratesTasks {
				for _, dep := range tpl.DependsOn {
					if _, ok := allTaskRuleIDs[dep]; !ok {
						return fmt.Errorf("%s/%s/%s: depends_on references unknown rule_id %q",
							rs.VisaCode, rule.ID, tpl.RuleID, dep)
					}
				}
			}
		}
	}

	return nil
}
