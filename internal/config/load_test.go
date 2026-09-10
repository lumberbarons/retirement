package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const examplePath = "../../household.example.yaml"

const minimalValid = `base_year: 2026
province: ON
spouses:
  - name: A
    birth_year: 1980
    retirement_age: 60
    cpp:
      monthly_at_65: 1000
      start_age: 65
    oas:
      start_age: 65
  - name: B
    birth_year: 1982
    retirement_age: 62
    cpp:
      monthly_at_65: 800
      start_age: 65
    oas:
      start_age: 65
accounts:
  - name: A TFSA
    type: tfsa
    owner: A
    balance: 50000
  - name: B RRSP
    type: rrsp
    owner: B
    balance: 100000
spending:
  target_today_dollars: 60000
`

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "household.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustLoad(t *testing.T, path string) *Household {
	t.Helper()
	h, err := Load(path)
	if err != nil {
		t.Fatalf("Load(%s): %v", path, err)
	}
	return h
}

func TestLoad_ExampleHouseholdLoadsClean(t *testing.T) {
	h := mustLoad(t, examplePath)
	if len(h.Spouses) != 2 {
		t.Fatalf("example spouses: got %d, want 2", len(h.Spouses))
	}
}

func TestLoad_ExampleCoversSchema(t *testing.T) {
	h := mustLoad(t, examplePath)
	if h.BaseYear != 2026 || h.Province != "ON" {
		t.Fatalf("base year/province = %d/%s, want 2026/ON", h.BaseYear, h.Province)
	}
	types := map[AccountType]bool{}
	owners := map[string]bool{}
	for _, name := range h.SpouseNames() {
		owners[name] = true
	}
	for _, a := range h.Accounts {
		types[a.Type] = true
		if !owners[a.Owner] {
			t.Fatalf("account %s owner %q is not a spouse", a.Name, a.Owner)
		}
	}
	for _, want := range []AccountType{AccountTFSA, AccountRRSP, AccountRRIF, AccountNonRegistered} {
		if !types[want] {
			t.Fatalf("example household has no %s account", want)
		}
	}
	spousal := false
	election := false
	acb := false
	for _, a := range h.Accounts {
		if a.Spousal != nil {
			spousal = true
		}
		if a.YoungerSpouseElection {
			election = true
		}
		if a.Type == AccountNonRegistered && a.ACB > 0 {
			acb = true
		}
	}
	if !spousal {
		t.Fatal("example household has no spousal RRSP tag")
	}
	if !election {
		t.Fatal("example household has no younger-spouse election")
	}
	if !acb {
		t.Fatal("example household has no non-registered ACB")
	}
	pensionFound := false
	for _, s := range h.Spouses {
		if s.CPP.MonthlyAt65 <= 0 {
			t.Fatalf("%s: cpp.monthly_at_65 = %v, want > 0", s.Name, s.CPP.MonthlyAt65)
		}
		if s.CPP.StartAge < 60 || s.CPP.StartAge > 70 {
			t.Fatalf("%s: cpp.start_age = %d, want in [60,70]", s.Name, s.CPP.StartAge)
		}
		if s.OAS.StartAge < 65 || s.OAS.StartAge > 70 {
			t.Fatalf("%s: oas.start_age = %d, want in [65,70]", s.Name, s.OAS.StartAge)
		}
		if s.Pension != nil {
			pensionFound = true
		}
	}
	if !pensionFound {
		t.Fatal("example household has no DB pension")
	}
	if h.Spending.TargetTodayDollars <= 0 {
		t.Fatalf("spending target = %v, want > 0", h.Spending.TargetTodayDollars)
	}
	if h.Spending.Mode != "flat" && h.Spending.Mode != "smile" {
		t.Fatalf("spending mode = %q, want flat or smile", h.Spending.Mode)
	}
	if len(h.Spending.Lumpy) == 0 {
		t.Fatal("example household has no lumpy spending streams")
	}
	for _, item := range h.Spending.Lumpy {
		if item.AmountTodayDollars <= 0 || item.StartYear < h.BaseYear {
			t.Fatalf("lumpy item %q = %+v, want a positive amount in a future year", item.Name, item)
		}
	}
	if h.Assumptions.PortfolioReturn <= 0 {
		t.Fatalf("portfolio return = %v, want > 0", h.Assumptions.PortfolioReturn)
	}
}

func TestLoad_InvalidConfigNamesFirstOffendingField(t *testing.T) {
	cases := []struct {
		name    string
		find    string
		replace string
		want    string
	}{
		{"spouse missing birth year", "birth_year: 1980", "birth_year: 0", "spouses[0].birth_year"},
		{"cpp start age below 60", "monthly_at_65: 1000\n      start_age: 65", "monthly_at_65: 1000\n      start_age: 55", "spouses[0].cpp.start_age"},
		{"oas start age above 70", "oas:\n      start_age: 65\n  - name: B", "oas:\n      start_age: 75\n  - name: B", "spouses[0].oas.start_age"},
		{"unknown account type", "type: tfsa", "type: gic", "accounts[0].type"},
		{"unknown account owner", "owner: A\n    balance: 50000", "owner: C\n    balance: 50000", "accounts[0].owner"},
		{"missing spending target", "target_today_dollars: 60000", "target_today_dollars: 0", "spending.target_today_dollars"},
		{
			"single spouse",
			"  - name: B\n    birth_year: 1982\n    retirement_age: 62\n    cpp:\n      monthly_at_65: 800\n      start_age: 65\n    oas:\n      start_age: 65\n",
			"",
			"spouses",
		},
		{
			"assumption inflation out of range",
			"spending:\n  target_today_dollars: 60000\n",
			"spending:\n  target_today_dollars: 60000\nassumptions:\n  inflation: 0.9\n",
			"assumptions.inflation",
		},
		{"invalid spending mode", "target_today_dollars: 60000", "target_today_dollars: 60000\n  mode: wavy", "spending.mode"},
		{
			"spousal contributor is the owner",
			"type: rrsp\n    owner: B",
			"type: rrsp\n    owner: B\n    spousal:\n      contributor: B\n      contribution_years: [2024]",
			"accounts[1].spousal.contributor",
		},
		{
			"spousal tag on a tfsa",
			"type: tfsa",
			"type: tfsa\n    spousal:\n      contributor: B\n      contribution_years: [2024]",
			"accounts[0].spousal",
		},
		{"younger-spouse election on a tfsa", "type: tfsa", "type: tfsa\n    younger_spouse_election: true", "accounts[0].younger_spouse_election"},
		{"duplicate spouse name", "- name: B\n    birth_year: 1982", "- name: A\n    birth_year: 1982", "spouses[1].name"},
		{"unknown yaml field", "birth_year: 1980", "birth_yer: 1980", "birth_yer"},
		{
			"pension accrual rate out of range",
			"  - name: A\n    birth_year: 1980\n",
			"  - name: A\n    birth_year: 1980\n    pension:\n      accrual_rate: 0.9\n      years_of_service: 30\n      final_average_earnings: 80000\n      start_age: 60\n",
			"spouses[0].pension.accrual_rate",
		},
		{"retirement after death", "birth_year: 1980\n    retirement_age: 60", "birth_year: 1980\n    retirement_age: 60\n    death_age: 55", "spouses[0].retirement_age"},
		{"base year below 2026", "base_year: 2026", "base_year: 2025", "base_year"},
		{"base year above 2100", "base_year: 2026", "base_year: 2150", "base_year"},
		{"province other than ON", "province: ON\n", "province: AB\n", "province"},
		{"birth year before 1900", "birth_year: 1980", "birth_year: 1850", "spouses[0].birth_year"},
		{"birth year after base year", "birth_year: 1980", "birth_year: 2030", "spouses[0].birth_year"},
		{"death age below 50", "birth_year: 1980\n    retirement_age: 60", "birth_year: 1980\n    retirement_age: 60\n    death_age: 40", "spouses[0].death_age"},
		{"death age above 120", "birth_year: 1980\n    retirement_age: 60", "birth_year: 1980\n    retirement_age: 60\n    death_age: 121", "spouses[0].death_age"},
		{"retirement age below 40", "retirement_age: 60", "retirement_age: 35", "spouses[0].retirement_age"},
		{"retirement age above 85", "retirement_age: 60", "retirement_age: 90", "spouses[0].retirement_age"},
		{"negative cpp monthly", "monthly_at_65: 1000", "monthly_at_65: -5", "spouses[0].cpp.monthly_at_65"},
		{"cpp start age above 70", "monthly_at_65: 1000\n      start_age: 65", "monthly_at_65: 1000\n      start_age: 75", "spouses[0].cpp.start_age"},
		{"oas start age below 65", "oas:\n      start_age: 65", "oas:\n      start_age: 60", "spouses[0].oas.start_age"},
		{"empty account name", "  - name: A TFSA", "  - name: ''", "accounts[0].name"},
		{"duplicate account name", "  - name: B RRSP", "  - name: A TFSA", "accounts[1].name"},
		{"negative account balance", "balance: 50000", "balance: -50000", "accounts[0].balance"},
		{"acb on a tfsa", "balance: 50000", "balance: 50000\n    acb: 100", "accounts[0].acb"},
		{
			"negative acb on non-registered",
			"  - name: B RRSP\n    type: rrsp\n    owner: B\n    balance: 100000\n",
			"  - name: B NR\n    type: non_registered\n    owner: B\n    balance: 100000\n    acb: -5\n",
			"accounts[1].acb",
		},
		{
			"spousal contributor is not a spouse",
			"type: rrsp\n    owner: B",
			"type: rrsp\n    owner: B\n    spousal:\n      contributor: C\n      contribution_years: [2024]",
			"accounts[1].spousal.contributor",
		},
		{
			"spousal contribution year out of range",
			"type: rrsp\n    owner: B",
			"type: rrsp\n    owner: B\n    spousal:\n      contributor: A\n      contribution_years: [1985]",
			"accounts[1].spousal.contribution_years",
		},
		{
			"pension years of service below 1",
			"  - name: A\n    birth_year: 1980\n",
			"  - name: A\n    birth_year: 1980\n    pension:\n      accrual_rate: 0.02\n      years_of_service: 0\n      final_average_earnings: 80000\n      start_age: 60\n",
			"spouses[0].pension.years_of_service",
		},
		{
			"pension final average earnings non-positive",
			"  - name: A\n    birth_year: 1980\n",
			"  - name: A\n    birth_year: 1980\n    pension:\n      accrual_rate: 0.02\n      years_of_service: 30\n      final_average_earnings: 0\n      start_age: 60\n",
			"spouses[0].pension.final_average_earnings",
		},
		{
			"pension start age below 45",
			"  - name: A\n    birth_year: 1980\n",
			"  - name: A\n    birth_year: 1980\n    pension:\n      accrual_rate: 0.02\n      years_of_service: 30\n      final_average_earnings: 80000\n      start_age: 44\n",
			"spouses[0].pension.start_age",
		},
		{
			"negative pension bridge",
			"  - name: A\n    birth_year: 1980\n",
			"  - name: A\n    birth_year: 1980\n    pension:\n      accrual_rate: 0.02\n      years_of_service: 30\n      final_average_earnings: 80000\n      start_age: 60\n      bridge_monthly: -5\n",
			"spouses[0].pension.bridge_monthly",
		},
		{
			"pension indexation not none/full_cpi/partial",
			"  - name: A\n    birth_year: 1980\n",
			"  - name: A\n    birth_year: 1980\n    pension:\n      accrual_rate: 0.02\n      years_of_service: 30\n      final_average_earnings: 80000\n      start_age: 60\n      indexation: bananas\n",
			"spouses[0].pension.indexation",
		},
		{
			"partial pension indexation without a rate",
			"  - name: A\n    birth_year: 1980\n",
			"  - name: A\n    birth_year: 1980\n    pension:\n      accrual_rate: 0.02\n      years_of_service: 30\n      final_average_earnings: 80000\n      start_age: 60\n      indexation: partial\n      indexation_rate: 0\n",
			"spouses[0].pension.indexation_rate",
		},
		{
			"pension survivor percent above 1",
			"  - name: A\n    birth_year: 1980\n",
			"  - name: A\n    birth_year: 1980\n    pension:\n      accrual_rate: 0.02\n      years_of_service: 30\n      final_average_earnings: 80000\n      start_age: 60\n      survivor_percent: 1.5\n",
			"spouses[0].pension.survivor_percent",
		},
		{
			"pension JS reduction above 1",
			"  - name: A\n    birth_year: 1980\n",
			"  - name: A\n    birth_year: 1980\n    pension:\n      accrual_rate: 0.02\n      years_of_service: 30\n      final_average_earnings: 80000\n      start_age: 60\n      js_reduction_factor: 1.5\n",
			"spouses[0].pension.js_reduction_factor",
		},
		{
			"spending inflation out of range",
			"spending:\n  target_today_dollars: 60000\n",
			"spending:\n  target_today_dollars: 60000\n  inflation: 0.5\n",
			"spending.inflation",
		},
		{
			"spending survivor factor above 1",
			"spending:\n  target_today_dollars: 60000\n",
			"spending:\n  target_today_dollars: 60000\n  survivor_factor: 1.5\n",
			"spending.survivor_factor",
		},
		{
			"lumpy amount non-positive",
			"spending:\n  target_today_dollars: 60000\n",
			"spending:\n  target_today_dollars: 60000\n  lumpy:\n    - amount_today_dollars: 0\n      start_year: 2030\n",
			"spending.lumpy[0].amount_today_dollars",
		},
		{
			"lumpy start year before base year",
			"spending:\n  target_today_dollars: 60000\n",
			"spending:\n  target_today_dollars: 60000\n  lumpy:\n    - amount_today_dollars: 1000\n      start_year: 2020\n",
			"spending.lumpy[0].start_year",
		},
		{
			"lumpy end year before start year",
			"spending:\n  target_today_dollars: 60000\n",
			"spending:\n  target_today_dollars: 60000\n  lumpy:\n    - amount_today_dollars: 1000\n      start_year: 2030\n      end_year: 2029\n",
			"spending.lumpy[0].end_year",
		},
		{
			"lumpy negative every-years",
			"spending:\n  target_today_dollars: 60000\n",
			"spending:\n  target_today_dollars: 60000\n  lumpy:\n    - amount_today_dollars: 1000\n      start_year: 2030\n      every_years: -2\n",
			"spending.lumpy[0].every_years",
		},
		{
			"wage growth out of range",
			"spending:\n  target_today_dollars: 60000\n",
			"spending:\n  target_today_dollars: 60000\nassumptions:\n  wage_growth: 0.5\n",
			"assumptions.wage_growth",
		},
		{
			"portfolio return below -0.5",
			"spending:\n  target_today_dollars: 60000\n",
			"spending:\n  target_today_dollars: 60000\nassumptions:\n  portfolio_return: -0.9\n",
			"assumptions.portfolio_return",
		},
		{
			"portfolio return at 0.5",
			"spending:\n  target_today_dollars: 60000\n",
			"spending:\n  target_today_dollars: 60000\nassumptions:\n  portfolio_return: 0.9\n",
			"assumptions.portfolio_return",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text := strings.Replace(minimalValid, tc.find, tc.replace, 1)
			_, err := Load(writeConfig(t, text))
			if err == nil {
				t.Fatalf("expected an error naming %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not name field %q", err.Error(), tc.want)
			}
		})
	}
}

func TestLoad_AppliesDefaults(t *testing.T) {
	h := mustLoad(t, writeConfig(t, `base_year: 2026
spouses:
  - {name: A, birth_year: 1980, retirement_age: 60}
  - {name: B, birth_year: 1982, retirement_age: 62}
spending: {target_today_dollars: 50000}
`))
	if h.Province != "ON" {
		t.Fatalf("province = %q, want ON", h.Province)
	}
	for i, s := range h.Spouses {
		if s.DeathAge != 95 {
			t.Fatalf("spouses[%d].death_age = %d, want default 95", i, s.DeathAge)
		}
		if s.CPP.StartAge != 65 {
			t.Fatalf("spouses[%d].cpp.start_age = %d, want default 65", i, s.CPP.StartAge)
		}
		if s.OAS.StartAge != 65 {
			t.Fatalf("spouses[%d].oas.start_age = %d, want default 65", i, s.OAS.StartAge)
		}
	}
	if h.Spending.Mode != "smile" {
		t.Fatalf("spending mode = %q, want default smile", h.Spending.Mode)
	}
	if h.Spending.SurvivorFactor != 0.70 {
		t.Fatalf("survivor factor = %v, want default 0.70", h.Spending.SurvivorFactor)
	}
	if h.Spending.Inflation != 0.021 {
		t.Fatalf("spending inflation = %v, want FP Canada default 0.021", h.Spending.Inflation)
	}
	if h.Assumptions.Inflation != 0.021 {
		t.Fatalf("assumption inflation = %v, want FP Canada default 0.021", h.Assumptions.Inflation)
	}
	if h.Assumptions.WageGrowth != 0.031 {
		t.Fatalf("wage growth = %v, want FP Canada default 0.031", h.Assumptions.WageGrowth)
	}
	wantReturn := (0.032 + 0.063 + 0.064 + 0.066) / 4
	if h.Assumptions.PortfolioReturn != wantReturn {
		t.Fatalf("portfolio return = %v, want equal-weight default %v", h.Assumptions.PortfolioReturn, wantReturn)
	}
}

func TestLoad_AppliesPensionAndAccountDefaults(t *testing.T) {
	h := mustLoad(t, writeConfig(t, `base_year: 2026
spouses:
  - name: A
    birth_year: 1980
    retirement_age: 60
    pension:
      accrual_rate: 0.02
      years_of_service: 30
      final_average_earnings: 80000
      start_age: 60
  - name: B
    birth_year: 1982
    retirement_age: 62
spending:
  target_today_dollars: 50000
accounts:
  - name: Joint NR
    type: non_registered
    owner: A
    balance: 100000
`))
	p := h.Spouses[0].Pension
	if p.Indexation != "none" {
		t.Fatalf("pension indexation = %q, want default none", p.Indexation)
	}
	if p.SurvivorPercent != 0.60 {
		t.Fatalf("pension survivor percent = %v, want default 0.60", p.SurvivorPercent)
	}
	if p.JSReductionFactor != 1 {
		t.Fatalf("pension JS reduction = %v, want default 1", p.JSReductionFactor)
	}
	b, err := Load(writeConfig(t, `base_year: 2026
province: ON
spouses:
  - name: A
    birth_year: 1980
    retirement_age: 60
  - name: B
    birth_year: 1982
    retirement_age: 62
accounts:
  - name: Joint NR
    type: non_registered
    owner: A
    balance: 100000
spending:
  target_today_dollars: 50000
`))
	if err != nil {
		t.Fatalf("load non-registered default acb: %v", err)
	}
	a := b.Accounts[0]
	if a.ACB != 100000 {
		t.Fatalf("non-registered ACB = %v, want default to balance 100000", a.ACB)
	}
	if h.Spending.Inflation != 0.021 {
		t.Fatalf("spending inflation = %v, want FP Canada default 0.021", h.Spending.Inflation)
	}
}

func TestLoad_MissingFileFails(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("expected an error for a missing file, got nil")
	}
}

func TestLoad_EmptyFileFails(t *testing.T) {
	_, err := Load(writeConfig(t, ""))
	if err == nil {
		t.Fatal("expected an error for an empty file, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Fatalf("error %q should say the config is empty", err.Error())
	}
}
