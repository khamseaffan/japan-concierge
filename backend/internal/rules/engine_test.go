package rules

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func mustParseDate(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("mustParseDate(%q): %v", s, err)
	}
	return parsed
}

// ---------- Loader / validation ----------

func TestLoadAll_LoadsBothVisaRuleSets(t *testing.T) {
	sets, err := LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(sets) < 2 {
		t.Fatalf("want at least 2 rule sets, got %d", len(sets))
	}
	want := map[string]bool{"jfind": false, "engineer": false}
	for _, s := range sets {
		if _, ok := want[s.VisaCode]; ok {
			want[s.VisaCode] = true
		}
	}
	for code, found := range want {
		if !found {
			t.Errorf("missing rule set for visa code %q", code)
		}
	}
}

func TestLoadAll_DependsOnValidation_RejectsUnknownReference(t *testing.T) {
	sets := []*VisaRuleSet{
		{
			VisaCode: "jfind",
			Rules: []Rule{{
				ID:             "r1",
				AppliesToVisas: []string{"jfind"},
				Trigger:        EventLandedJapan,
				GeneratesTasks: []TaskTemplate{{
					RuleID:        "task_a",
					TitleEN:       "Task A",
					Category:      CategoryGeneral,
					Severity:      SeverityMandatory,
					Deadline:      DeadlineSpec{NoDeadline: true},
					DependsOn:     []string{"task_does_not_exist"},
				}},
			}},
		},
	}
	err := validateRuleSets(sets)
	if err == nil {
		t.Fatal("expected error for unknown depends_on reference, got nil")
	}
}

func TestLoadAll_DependsOnValidation_AllowsCrossRulesetReference(t *testing.T) {
	sets := []*VisaRuleSet{
		{
			VisaCode: "a",
			Rules: []Rule{{
				ID:             "r1",
				AppliesToVisas: []string{"a"},
				Trigger:        EventLandedJapan,
				GeneratesTasks: []TaskTemplate{{
					RuleID: "task_a", TitleEN: "A", Category: CategoryGeneral,
					Severity: SeverityMandatory, Deadline: DeadlineSpec{NoDeadline: true},
				}},
			}},
		},
		{
			VisaCode: "b",
			Rules: []Rule{{
				ID:             "r2",
				AppliesToVisas: []string{"b"},
				Trigger:        EventLandedJapan,
				GeneratesTasks: []TaskTemplate{{
					RuleID: "task_b", TitleEN: "B", Category: CategoryGeneral,
					Severity: SeverityMandatory, Deadline: DeadlineSpec{NoDeadline: true},
					DependsOn: []string{"task_a"}, // cross-ruleset reference is OK
				}},
			}},
		},
	}
	if err := validateRuleSets(sets); err != nil {
		t.Errorf("expected cross-ruleset depends_on to validate, got error: %v", err)
	}
}

func TestLoadAll_RejectsDuplicateTaskRuleID(t *testing.T) {
	sets := []*VisaRuleSet{
		{
			VisaCode: "x",
			Rules: []Rule{
				{
					ID: "r1", AppliesToVisas: []string{"x"}, Trigger: EventLandedJapan,
					GeneratesTasks: []TaskTemplate{{
						RuleID: "dup", TitleEN: "A", Category: CategoryGeneral,
						Severity: SeverityMandatory, Deadline: DeadlineSpec{NoDeadline: true},
					}},
				},
				{
					ID: "r2", AppliesToVisas: []string{"x"}, Trigger: EventLandedJapan,
					GeneratesTasks: []TaskTemplate{{
						RuleID: "dup", TitleEN: "B", Category: CategoryGeneral,
						Severity: SeverityMandatory, Deadline: DeadlineSpec{NoDeadline: true},
					}},
				},
			},
		},
	}
	err := validateRuleSets(sets)
	if err == nil {
		t.Fatal("expected duplicate task rule_id error, got nil")
	}
}

func TestLoadAll_RejectsBadDeadlineSpec(t *testing.T) {
	relDays := 7
	d := mustParseDate(t, "2026-01-01")
	cases := []struct {
		name string
		spec DeadlineSpec
	}{
		{"all three set", DeadlineSpec{RelativeDays: &relDays, AbsoluteDate: &Date{Time: d}, NoDeadline: true}},
		{"none set", DeadlineSpec{}},
		{"two set", DeadlineSpec{RelativeDays: &relDays, NoDeadline: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sets := []*VisaRuleSet{{
				VisaCode: "x",
				Rules: []Rule{{
					ID: "r1", AppliesToVisas: []string{"x"}, Trigger: EventLandedJapan,
					GeneratesTasks: []TaskTemplate{{
						RuleID: "t", TitleEN: "T", Category: CategoryGeneral,
						Severity: SeverityMandatory, Deadline: tc.spec,
					}},
				}},
			}}
			if err := validateRuleSets(sets); err == nil {
				t.Errorf("expected error for malformed deadline (%s), got nil", tc.name)
			}
		})
	}
}

// ---------- Custom Date YAML parsing ----------

func TestDate_UnmarshalYAML_AcceptsBareDate(t *testing.T) {
	doc := []byte(`absolute_date: 2026-03-15`)
	var spec DeadlineSpec
	if err := yaml.Unmarshal(doc, &spec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if spec.AbsoluteDate == nil {
		t.Fatal("AbsoluteDate is nil")
	}
	want := mustParseDate(t, "2026-03-15")
	if !spec.AbsoluteDate.Time.Equal(want) {
		t.Errorf("got %v, want %v", spec.AbsoluteDate.Time, want)
	}
}

func TestDate_UnmarshalYAML_RejectsBadFormat(t *testing.T) {
	doc := []byte(`absolute_date: "March 15, 2026"`)
	var spec DeadlineSpec
	if err := yaml.Unmarshal(doc, &spec); err == nil {
		t.Fatal("expected error for non-ISO date, got nil")
	}
}

// ---------- Engine evaluation ----------

func TestJFIND_PreArrivalDocumentCheck(t *testing.T) {
	sets, err := LoadAll()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	engine := NewEngine(sets)

	tasks, err := engine.Evaluate(EvaluationInput{
		UserID:     1,
		VisaCode:   "jfind",
		EventType:  EventVisaApplicationStarted,
		OccurredAt: mustParseDate(t, "2026-05-09"),
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	expectedRules := map[string]bool{
		"jfind_passport_validity_check": false,
		"jfind_university_transcript":   false,
		"jfind_proof_of_funds":          false,
		"jfind_consulate_application":   false,
	}
	for _, task := range tasks {
		if _, ok := expectedRules[task.RuleID]; ok {
			expectedRules[task.RuleID] = true
		}
	}
	for ruleID, fired := range expectedRules {
		if !fired {
			t.Errorf("expected rule %s to fire, did not", ruleID)
		}
	}
}

func TestJFIND_PostLanding14DayWindow(t *testing.T) {
	sets, err := LoadAll()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	engine := NewEngine(sets)

	landingDate := mustParseDate(t, "2026-08-01")
	tasks, err := engine.Evaluate(EvaluationInput{
		UserID:     1,
		VisaCode:   "jfind",
		EventType:  EventLandedJapan,
		OccurredAt: landingDate,
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	var addrTask *GeneratedTask
	for i := range tasks {
		if tasks[i].RuleID == "jfind_post_landing_address_registration" {
			addrTask = &tasks[i]
			break
		}
	}
	if addrTask == nil {
		t.Fatal("address registration task not generated")
	}
	if addrTask.DeadlineAt == nil {
		t.Fatal("address registration deadline is nil")
	}
	want := mustParseDate(t, "2026-08-15")
	if !addrTask.DeadlineAt.Equal(want) {
		t.Errorf("deadline = %v, want %v", addrTask.DeadlineAt, want)
	}
	if addrTask.Severity != SeverityMandatory {
		t.Errorf("severity = %v, want mandatory", addrTask.Severity)
	}
}

func TestEngineer_PostLandingGeneratesEmployerInsurance(t *testing.T) {
	sets, err := LoadAll()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	engine := NewEngine(sets)

	tasks, err := engine.Evaluate(EvaluationInput{
		UserID:     1,
		VisaCode:   "engineer",
		EventType:  EventLandedJapan,
		OccurredAt: mustParseDate(t, "2026-09-01"),
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	wantRules := []string{
		"engineer_post_landing_address_registration",
		"engineer_employer_health_insurance_confirmation",
		"engineer_employer_pension_confirmation",
	}
	got := map[string]bool{}
	for _, task := range tasks {
		got[task.RuleID] = true
	}
	for _, r := range wantRules {
		if !got[r] {
			t.Errorf("expected rule %s to fire for engineer post-landing", r)
		}
	}
}

func TestEngine_RejectsUnknownVisa(t *testing.T) {
	sets, err := LoadAll()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	engine := NewEngine(sets)

	_, err = engine.Evaluate(EvaluationInput{
		UserID:     1,
		VisaCode:   "highly_skilled_professional",
		EventType:  EventLandedJapan,
		OccurredAt: time.Now(),
	})
	if err == nil {
		t.Fatal("expected error for unknown visa code, got nil")
	}
}

func TestEngine_WildcardRulesApplyToAllKnownVisas(t *testing.T) {
	sets, err := LoadAll()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	engine := NewEngine(sets)

	for _, visa := range []string{"jfind", "engineer"} {
		t.Run(visa, func(t *testing.T) {
			tasks, err := engine.Evaluate(EvaluationInput{
				UserID:     1,
				VisaCode:   visa,
				EventType:  EventTaxResidencyTriggered,
				OccurredAt: mustParseDate(t, "2027-01-30"),
			})
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			found := false
			for _, task := range tasks {
				if task.RuleID == "tax_residency_filing_obligation" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("wildcard tax residency rule did not fire for %s", visa)
			}
		})
	}
}

func TestEngine_NoTasksForUnhandledEvent(t *testing.T) {
	sets, err := LoadAll()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	engine := NewEngine(sets)

	tasks, err := engine.Evaluate(EvaluationInput{
		UserID:     1,
		VisaCode:   "jfind",
		EventType:  EventVisaApproved,
		OccurredAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks for unhandled event, got %d", len(tasks))
	}
}

func TestEngine_VisaCodes_Sorted(t *testing.T) {
	sets, err := LoadAll()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	engine := NewEngine(sets)
	codes := engine.VisaCodes()
	if len(codes) < 2 {
		t.Fatalf("want at least 2 codes, got %v", codes)
	}
	for i := 1; i < len(codes); i++ {
		if codes[i-1] > codes[i] {
			t.Errorf("VisaCodes not sorted: %v", codes)
			break
		}
	}
}

func TestEngineer_RenewalWindowDeadlineMath(t *testing.T) {
	sets, err := LoadAll()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	engine := NewEngine(sets)

	windowOpenedAt := mustParseDate(t, "2027-06-01")
	tasks, err := engine.Evaluate(EvaluationInput{
		UserID:     1,
		VisaCode:   "engineer",
		EventType:  EventVisaRenewalWindowOpens,
		OccurredAt: windowOpenedAt,
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(tasks) == 0 {
		t.Fatal("renewal window event produced no tasks")
	}
	want := windowOpenedAt.AddDate(0, 0, 90)
	if !tasks[0].DeadlineAt.Equal(want) {
		t.Errorf("renewal deadline = %v, want %v", tasks[0].DeadlineAt, want)
	}
}
