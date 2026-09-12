package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/lumberbarons/retirement/internal/constants"
	"gopkg.in/yaml.v3"
)

const (
	defaultDeathAge       = 95
	defaultProvince       = "ON"
	defaultSurvivorFactor = 0.70
	defaultStartAge       = 65
	defaultSurvivorPct    = 0.60
)

type FieldError struct {
	Path    string
	Message string
}

func (e *FieldError) Error() string {
	return e.Path + ": " + e.Message
}

func fieldError(path, format string, args ...any) *FieldError {
	return &FieldError{Path: path, Message: fmt.Sprintf(format, args...)}
}

func Load(path string) (*Household, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

func Parse(data []byte) (*Household, error) {
	h := &Household{}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(h); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("config is empty")
		}
		return nil, err
	}
	if h.BaseYear != 0 && (h.BaseYear < 2026 || h.BaseYear > 2100) {
		return nil, fieldError("base_year", "must be between 2026 and 2100, got %d", h.BaseYear)
	}
	if err := applyDefaults(h); err != nil {
		return nil, err
	}
	if err := Validate(h); err != nil {
		return nil, err
	}
	return h, nil
}

func applyDefaults(h *Household) error {
	if h.BaseYear == 0 {
		h.BaseYear = time.Now().Year()
	}
	if h.Province == "" {
		h.Province = defaultProvince
	}
	for i := range h.Spouses {
		s := &h.Spouses[i]
		if s.DeathAge == 0 {
			s.DeathAge = defaultDeathAge
		}
		if s.CPP.StartAge == 0 {
			s.CPP.StartAge = defaultStartAge
		}
		if s.OAS.StartAge == 0 {
			s.OAS.StartAge = defaultStartAge
		}
		if s.Pension != nil {
			if s.Pension.Indexation == "" {
				s.Pension.Indexation = "none"
			}
			if s.Pension.SurvivorPercent == 0 {
				s.Pension.SurvivorPercent = defaultSurvivorPct
			}
			if s.Pension.JSReductionFactor == 0 {
				s.Pension.JSReductionFactor = 1
			}
		}
	}
	for i := range h.Accounts {
		a := &h.Accounts[i]
		if a.Type == AccountNonRegistered && a.ACB == 0 {
			a.ACB = a.Balance
		}
	}
	if h.Spending.Mode == "" {
		h.Spending.Mode = "smile"
	}
	if h.Spending.SurvivorFactor == 0 {
		h.Spending.SurvivorFactor = defaultSurvivorFactor
	}
	fp, _, err := constants.FPCanada.For(h.BaseYear)
	if err != nil {
		return err
	}
	if h.Assumptions.Inflation == 0 {
		h.Assumptions.Inflation = fp.Inflation
	}
	if h.Assumptions.WageGrowth == 0 {
		h.Assumptions.WageGrowth = fp.SalaryGrowth
	}
	if h.Spending.Inflation == 0 {
		h.Spending.Inflation = fp.Inflation
	}
	if h.Assumptions.PortfolioReturn == 0 {
		h.Assumptions.PortfolioReturn = (fp.FixedIncome + fp.CanadianEquity + fp.USEquity + fp.IntlDevelopedEquity) / 4
	}
	return nil
}

func Validate(h *Household) error {
	if h.BaseYear < 2026 || h.BaseYear > 2100 {
		return fieldError("base_year", "must be between 2026 and 2100, got %d", h.BaseYear)
	}
	if h.Province != defaultProvince {
		return fieldError("province", "only ON is supported, got %q", h.Province)
	}
	if h.CPPSharingFraction < 0 || h.CPPSharingFraction > 1 {
		return fieldError("cpp_sharing_fraction", "must be in [0, 1], got %v", h.CPPSharingFraction)
	}
	if err := validateSpouses(h); err != nil {
		return err
	}
	if err := validateAccounts(h); err != nil {
		return err
	}
	if err := validatePlan(h); err != nil {
		return err
	}
	return validateEmploymentSavings(h)
}

// validateEmploymentSavings requires a non_registered account whenever a
// spouse declares employment income: pre-retirement surplus is retained
// there, and without one the engine would have nowhere to deposit it.
func validateEmploymentSavings(h *Household) error {
	for i := range h.Accounts {
		if h.Accounts[i].Type == AccountNonRegistered {
			return nil
		}
	}
	for i := range h.Spouses {
		s := &h.Spouses[i]
		if s.EmploymentIncome > 0 && s.BirthYear+s.RetirementAge >= h.BaseYear {
			return fieldError(fmt.Sprintf("spouses[%d].employment_income", i),
				"requires a non_registered account to hold surplus savings")
		}
	}
	return nil
}

func validateSpouses(h *Household) error {
	if len(h.Spouses) != 2 {
		return fieldError("spouses", "exactly 2 spouses are required, got %d", len(h.Spouses))
	}
	names := map[string]bool{}
	for i := range h.Spouses {
		s := &h.Spouses[i]
		p := fmt.Sprintf("spouses[%d]", i)
		if s.Name == "" {
			return fieldError(p+".name", "is required")
		}
		if names[s.Name] {
			return fieldError(p+".name", "duplicate spouse name %q", s.Name)
		}
		names[s.Name] = true
		if s.BirthYear == 0 {
			return fieldError(p+".birth_year", "is required")
		}
		if s.BirthYear < 1900 || s.BirthYear > h.BaseYear {
			return fieldError(p+".birth_year", "must be between 1900 and %d, got %d", h.BaseYear, s.BirthYear)
		}
		if s.DeathAge < 50 || s.DeathAge > 120 {
			return fieldError(p+".death_age", "must be between 50 and 120, got %d", s.DeathAge)
		}
		if s.RetirementAge == 0 {
			return fieldError(p+".retirement_age", "is required")
		}
		if s.RetirementAge < 40 || s.RetirementAge > 85 {
			return fieldError(p+".retirement_age", "must be between 40 and 85, got %d", s.RetirementAge)
		}
		if s.RetirementAge > s.DeathAge {
			return fieldError(p+".retirement_age", "%d is after death age %d", s.RetirementAge, s.DeathAge)
		}
		if s.EmploymentIncome < 0 {
			return fieldError(p+".employment_income", "must not be negative, got %v", s.EmploymentIncome)
		}
		if s.CPP.MonthlyAt65 < 0 {
			return fieldError(p+".cpp.monthly_at_65", "must not be negative, got %v", s.CPP.MonthlyAt65)
		}
		if s.CPP.StartAge < 60 || s.CPP.StartAge > 70 {
			return fieldError(p+".cpp.start_age", "must be between 60 and 70, got %d", s.CPP.StartAge)
		}
		if s.OAS.StartAge < 65 || s.OAS.StartAge > 70 {
			return fieldError(p+".oas.start_age", "must be between 65 and 70, got %d", s.OAS.StartAge)
		}
		if err := validatePension(p, s.Pension, h.BaseYear); err != nil {
			return err
		}
		if err := validatePensionSplit(p, s.PensionSplit, h.BaseYear); err != nil {
			return err
		}
	}
	return nil
}

func validatePensionSplit(spousePath string, splits []PensionSplitYear, baseYear int) error {
	seen := map[int]bool{}
	for i, ps := range splits {
		p := fmt.Sprintf("%s.pension_split[%d]", spousePath, i)
		if ps.Year < baseYear || ps.Year > 2100 {
			return fieldError(p+".year", "must be between %d and 2100, got %d", baseYear, ps.Year)
		}
		if ps.Fraction < 0 || ps.Fraction > 0.5 {
			return fieldError(p+".fraction", "must be in [0, 0.5], got %v", ps.Fraction)
		}
		if seen[ps.Year] {
			return fieldError(p+".year", "duplicate pension split year %d", ps.Year)
		}
		seen[ps.Year] = true
	}
	return nil
}

func validatePension(spousePath string, pn *DBPension, baseYear int) error {
	if pn == nil {
		return nil
	}
	p := spousePath + ".pension"
	if pn.AccrualRate <= 0 || pn.AccrualRate > 0.05 {
		return fieldError(p+".accrual_rate", "must be in (0, 0.05], got %v", pn.AccrualRate)
	}
	if pn.YearsOfService <= 0 || pn.YearsOfService > 60 {
		return fieldError(p+".years_of_service", "must be in (0, 60], got %v", pn.YearsOfService)
	}
	if pn.FinalAverageEarnings <= 0 {
		return fieldError(p+".final_average_earnings", "must be positive, got %v", pn.FinalAverageEarnings)
	}
	if pn.StartAge < 45 || pn.StartAge > 85 {
		return fieldError(p+".start_age", "must be between 45 and 85, got %d", pn.StartAge)
	}
	if pn.BridgeMonthly < 0 {
		return fieldError(p+".bridge_monthly", "must not be negative, got %v", pn.BridgeMonthly)
	}
	switch pn.Indexation {
	case "none", "full_cpi":
	case "partial":
		if pn.IndexationRate <= 0 || pn.IndexationRate > 1 {
			return fieldError(p+".indexation_rate", "must be in (0, 1] for partial indexation, got %v", pn.IndexationRate)
		}
	default:
		return fieldError(p+".indexation", "must be none, full_cpi, or partial, got %q", pn.Indexation)
	}
	if pn.SurvivorPercent <= 0 || pn.SurvivorPercent > 1 {
		return fieldError(p+".survivor_percent", "must be in (0, 1], got %v", pn.SurvivorPercent)
	}
	if pn.JSReductionFactor <= 0 || pn.JSReductionFactor > 1 {
		return fieldError(p+".js_reduction_factor", "must be in (0, 1], got %v", pn.JSReductionFactor)
	}
	return nil
}

func validateAccounts(h *Household) error {
	names := map[string]bool{}
	for _, s := range h.Spouses {
		names[s.Name] = true
	}
	accountNames := map[string]bool{}
	for i := range h.Accounts {
		a := &h.Accounts[i]
		p := fmt.Sprintf("accounts[%d]", i)
		if a.Name == "" {
			return fieldError(p+".name", "is required")
		}
		if accountNames[a.Name] {
			return fieldError(p+".name", "duplicate account name %q", a.Name)
		}
		accountNames[a.Name] = true
		switch a.Type {
		case AccountTFSA, AccountRRSP, AccountRRIF, AccountNonRegistered:
		default:
			return fieldError(p+".type", "must be tfsa, rrsp, rrif, or non_registered, got %q", a.Type)
		}
		if !names[a.Owner] {
			return fieldError(p+".owner", "%q is not a spouse", a.Owner)
		}
		if a.Balance < 0 {
			return fieldError(p+".balance", "must not be negative, got %v", a.Balance)
		}
		if a.ACB < 0 {
			return fieldError(p+".acb", "must not be negative, got %v", a.ACB)
		}
		if a.ACB > 0 && a.Type != AccountNonRegistered {
			return fieldError(p+".acb", "is only valid on a non_registered account")
		}
		if a.Spousal != nil {
			if a.Type != AccountRRSP {
				return fieldError(p+".spousal", "is only valid on an rrsp account")
			}
			if !names[a.Spousal.Contributor] {
				return fieldError(p+".spousal.contributor", "%q is not a spouse", a.Spousal.Contributor)
			}
			if a.Spousal.Contributor == a.Owner {
				return fieldError(p+".spousal.contributor", "must be the other spouse, got the owner %q", a.Owner)
			}
			for _, y := range a.Spousal.ContributionYears {
				if y < 1990 || y > h.BaseYear {
					return fieldError(p+".spousal.contribution_years", "year %d must be between 1990 and %d", y, h.BaseYear)
				}
			}
		}
		if a.YoungerSpouseElection && a.Type != AccountRRIF {
			return fieldError(p+".younger_spouse_election", "is only valid on a rrif account")
		}
	}
	return nil
}

func validatePlan(h *Household) error {
	if h.Spending.TargetTodayDollars <= 0 {
		return fieldError("spending.target_today_dollars", "must be positive, got %v", h.Spending.TargetTodayDollars)
	}
	if h.Spending.Mode != "flat" && h.Spending.Mode != "smile" {
		return fieldError("spending.mode", "must be flat or smile, got %q", h.Spending.Mode)
	}
	if h.Spending.Inflation <= 0 || h.Spending.Inflation > 0.2 {
		return fieldError("spending.inflation", "must be in (0, 0.2], got %v", h.Spending.Inflation)
	}
	if h.Spending.SurvivorFactor <= 0 || h.Spending.SurvivorFactor > 1 {
		return fieldError("spending.survivor_factor", "must be in (0, 1], got %v", h.Spending.SurvivorFactor)
	}
	for i, item := range h.Spending.Lumpy {
		p := fmt.Sprintf("spending.lumpy[%d]", i)
		if item.AmountTodayDollars <= 0 {
			return fieldError(p+".amount_today_dollars", "must be positive, got %v", item.AmountTodayDollars)
		}
		if item.StartYear < h.BaseYear {
			return fieldError(p+".start_year", "must not be before base year %d, got %d", h.BaseYear, item.StartYear)
		}
		if item.EndYear != 0 && item.EndYear < item.StartYear {
			return fieldError(p+".end_year", "must not be before start year %d, got %d", item.StartYear, item.EndYear)
		}
		if item.EveryYears < 0 {
			return fieldError(p+".every_years", "must not be negative, got %d", item.EveryYears)
		}
	}
	if h.Assumptions.Inflation <= 0 || h.Assumptions.Inflation > 0.2 {
		return fieldError("assumptions.inflation", "must be in (0, 0.2], got %v", h.Assumptions.Inflation)
	}
	if h.Assumptions.WageGrowth <= 0 || h.Assumptions.WageGrowth > 0.2 {
		return fieldError("assumptions.wage_growth", "must be in (0, 0.2], got %v", h.Assumptions.WageGrowth)
	}
	if h.Assumptions.PortfolioReturn <= -0.5 || h.Assumptions.PortfolioReturn >= 0.5 {
		return fieldError("assumptions.portfolio_return", "must be in [-0.5, 0.5), got %v", h.Assumptions.PortfolioReturn)
	}
	return nil
}
