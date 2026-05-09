// Package rules is the pure compliance rules engine. It has no dependencies
// on the database, HTTP, or wall-clock time. Given a set of loaded rules and
// an input event, it returns the tasks that event should generate.
package rules

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// EventType is the kind of life event that can trigger rules.
type EventType string

const (
	EventVisaApplicationStarted EventType = "visa_application_started"
	EventVisaApplied            EventType = "visa_applied"
	EventVisaApproved           EventType = "visa_approved"
	EventCoEReceived            EventType = "coe_received"
	EventLandedJapan            EventType = "landed_japan"
	EventAddressRegistered      EventType = "address_registered"
	EventAddressChanged         EventType = "address_changed"
	EventEmployerChanged        EventType = "employer_changed"
	EventVisaRenewalWindowOpens EventType = "visa_renewal_window_opens"
	EventTaxResidencyTriggered  EventType = "tax_residency_triggered"
)

// Severity classifies how strictly a task must be done.
type Severity string

const (
	SeverityMandatory     Severity = "mandatory"
	SeverityRecommended   Severity = "recommended"
	SeverityInformational Severity = "informational"
)

// Category groups tasks by domain.
type Category string

const (
	CategoryPreArrival      Category = "pre_arrival"
	CategoryImmigration     Category = "immigration"
	CategoryMunicipal       Category = "municipal"
	CategoryTax             Category = "tax"
	CategoryHealthInsurance Category = "health_insurance"
	CategoryPension         Category = "pension"
	CategoryBanking         Category = "banking"
	CategoryTelecom         Category = "telecom"
	CategoryEmployer        Category = "employer"
	CategoryHousing         Category = "housing"
	CategoryGeneral         Category = "general"
)

// Date is a calendar date with no time-of-day component, parsed from YYYY-MM-DD.
//
// yaml.v3's default time.Time unmarshaller accepts RFC3339 timestamps but not
// bare date strings, which silently fails on values like "2026-03-15". This
// type accepts the bare date form rules authors actually write.
type Date struct {
	time.Time
}

// UnmarshalYAML parses a YAML scalar in YYYY-MM-DD form into a Date.
func (d *Date) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode {
		return fmt.Errorf("date must be a scalar (got kind %d at line %d)", value.Kind, value.Line)
	}
	t, err := time.Parse("2006-01-02", value.Value)
	if err != nil {
		return fmt.Errorf("invalid date %q at line %d: %w", value.Value, value.Line, err)
	}
	d.Time = t
	return nil
}

// MarshalYAML emits the date back as a YYYY-MM-DD scalar.
func (d Date) MarshalYAML() (any, error) {
	return d.Format("2006-01-02"), nil
}

// DeadlineSpec describes how to compute a deadline relative to a trigger event.
// Exactly one of RelativeDays, AbsoluteDate, or NoDeadline must be set.
type DeadlineSpec struct {
	RelativeDays *int  `yaml:"relative_days,omitempty"`
	AbsoluteDate *Date `yaml:"absolute_date,omitempty"`
	NoDeadline   bool  `yaml:"no_deadline,omitempty"`
}

// TaskTemplate describes a task that a rule will create when fired.
type TaskTemplate struct {
	RuleID          string       `yaml:"rule_id"`
	TitleEN         string       `yaml:"title_en"`
	TitleJA         string       `yaml:"title_ja"`
	DescriptionEN   string       `yaml:"description_en"`
	DescriptionJA   string       `yaml:"description_ja"`
	Category        Category     `yaml:"category"`
	Severity        Severity     `yaml:"severity"`
	Deadline        DeadlineSpec `yaml:"deadline"`
	LegalSourceURL  string       `yaml:"legal_source_url,omitempty"`
	LegalSourceText string       `yaml:"legal_source_text,omitempty"`
	LocationHint    string       `yaml:"location_hint,omitempty"`
	DependsOn       []string     `yaml:"depends_on,omitempty"`
}

// Rule is one trigger -> set-of-tasks mapping, scoped to applicable visas.
type Rule struct {
	ID             string         `yaml:"id"`
	AppliesToVisas []string       `yaml:"applies_to_visas"`
	Trigger        EventType      `yaml:"trigger"`
	GeneratesTasks []TaskTemplate `yaml:"generates_tasks"`
	Notes          string         `yaml:"notes,omitempty"`
}

// VisaRuleSet is the parsed contents of one YAML file.
type VisaRuleSet struct {
	VisaCode string `yaml:"visa_code"`
	Rules    []Rule `yaml:"rules"`
}

// EvaluationInput is what the engine needs to evaluate rules for an event.
type EvaluationInput struct {
	UserID      int64
	VisaCode    string
	EventType   EventType
	OccurredAt  time.Time
	UserContext UserContext
}

// UserContext carries any state rules might consult (current address, employer, etc).
// Unused by v1 rules but defined now so the engine signature is stable.
type UserContext struct {
	HasJapaneseEmployer bool
	HasOverseasEmployer bool
	DaysInJapan         int
	MunicipalityCode    string
}

// GeneratedTask is the engine's output: a task ready to persist.
type GeneratedTask struct {
	RuleID          string
	TitleEN         string
	TitleJA         string
	DescriptionEN   string
	DescriptionJA   string
	Category        Category
	Severity        Severity
	DeadlineAt      *time.Time
	LegalSourceURL  string
	LegalSourceText string
	LocationHint    string
	DependsOnRules  []string
}
