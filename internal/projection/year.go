package projection

import (
	"math"

	"github.com/lumberbarons/retirement/internal/config"
	"github.com/lumberbarons/retirement/internal/constants"
	"github.com/lumberbarons/retirement/internal/tax"
)

type AccountYear struct {
	Name       string
	Begin      float64
	Withdrawal float64
	End        float64
}

type YearResult struct {
	Year             int
	BeginTotal       float64
	EndTotal         float64
	MandatoryIncome  float64
	EmploymentIncome float64
	Surplus          float64
	Withdrawals      float64
	GrossIncome      float64
	TaxableIncome    float64
	Tax              float64
	NetSpending      float64
	TargetNominal    float64
	Accounts         []AccountYear
	Deaths           []string
}

func Run(h *config.Household, startYear int) ([]YearResult, error) {
	if err := config.Validate(h); err != nil {
		return nil, err
	}
	s := NewState(h, startYear)
	last := s.secondDeathYear()
	results := make([]YearResult, 0, last-startYear+1)
	for s.Year <= last {
		res, err := s.stepYear(h)
		if err != nil {
			return nil, err
		}
		results = append(results, res)
		s.Year++
	}
	return results, nil
}

func (s *State) stepYear(h *config.Household) (YearResult, error) {
	res := YearResult{Year: s.Year}
	incomes := make([]tax.Income, len(s.People))
	s.stepSnapshot(&res)
	s.stepReturns(h, &res)
	s.stepMandatoryIncome(incomes, &res)
	s.stepEmploymentIncome(h, incomes, &res)
	if err := s.stepDiscretionaryWithdrawals(h, incomes, &res); err != nil {
		return YearResult{}, err
	}
	if err := s.stepTaxes(h, incomes, &res); err != nil {
		return YearResult{}, err
	}
	s.stepRollForward(&res)
	s.stepDeathEvents(&res)
	return res, nil
}

func (s *State) stepSnapshot(res *YearResult) {
	res.Accounts = make([]AccountYear, len(s.Accounts))
	for i, a := range s.Accounts {
		res.Accounts[i] = AccountYear{Name: a.Name, Begin: a.Balance}
		res.BeginTotal += a.Balance
	}
}

func (s *State) stepReturns(h *config.Household, res *YearResult) {
	for _, a := range s.Accounts {
		a.Balance *= 1 + h.Assumptions.PortfolioReturn
	}
}

func (s *State) stepMandatoryIncome(incomes []tax.Income, res *YearResult) {
	for i, a := range s.Accounts {
		if a.Type != config.AccountRRIF {
			continue
		}
		owner := s.person(a.Owner)
		if owner == nil || !owner.Alive {
			continue
		}
		age := owner.AgeAtJan1(s.Year)
		if a.YoungerSpouseElection {
			// Irrevocable election at RRIF setup: minimums use the younger
			// spouse's Jan-1 age, and keep using it after that spouse dies.
			if spouse := s.spouse(a.Owner); spouse != nil {
				age = spouse.AgeAtJan1(s.Year)
			}
		}
		factor := constants.RRIFMinimumFactor(age)
		minimum := math.Min(RoundCents(res.Accounts[i].Begin*factor), a.Balance)
		minimum = RoundCents(minimum)
		a.Balance -= minimum
		res.Accounts[i].Withdrawal += minimum
		res.MandatoryIncome += minimum
		incomes[s.personIndex(a.Owner)].RRIFWithdrawals += minimum
	}
}

// stepEmploymentIncome posts each spouse's earned income for the year. The
// configured amount is in base-year dollars and grows with the wage-growth
// assumption: a full year through the year before retirement, half a year in
// the retirement year, and nothing after. Income also stops with the spouse.
func (s *State) stepEmploymentIncome(h *config.Household, incomes []tax.Income, res *YearResult) {
	forward := constants.Forward{CPI: h.Assumptions.Inflation, Wage: h.Assumptions.WageGrowth}
	for i := range h.Spouses {
		sp := &h.Spouses[i]
		if sp.EmploymentIncome <= 0 {
			continue
		}
		p := &s.People[i]
		if !p.Alive || s.Year > p.RetirementYear() {
			continue
		}
		amount := constants.ForwardIndex(sp.EmploymentIncome, constants.BasisAverageWage,
			h.BaseYear, s.Year, forward)
		if s.Year == p.RetirementYear() {
			amount *= midYearFraction
		}
		amount = RoundCents(amount)
		incomes[i].Employment += amount
		res.EmploymentIncome += amount
	}
	res.EmploymentIncome = RoundCents(res.EmploymentIncome)
}

func (s *State) stepDiscretionaryWithdrawals(h *config.Household, incomes []tax.Income, res *YearResult) error {
	res.TargetNominal = s.nominalSpending(h)
	var solveErr error
	net := func(withdrawal float64) float64 {
		trial := s.trialIncomes(withdrawal, incomes)
		result, err := s.householdTax(h, trial)
		if err != nil {
			solveErr = err
			return 0
		}
		return res.MandatoryIncome + res.EmploymentIncome + withdrawal - result.Total
	}
	withdrawal := RoundCents(SolveGross(res.TargetNominal, s.withdrawalCapacity(), net))
	if solveErr != nil {
		return solveErr
	}
	if withdrawal > 0 {
		s.applyWithdrawals(s.allocate(withdrawal), incomes, res)
	}
	res.Withdrawals = RoundCents(res.Withdrawals)
	res.GrossIncome = RoundCents(res.MandatoryIncome + res.EmploymentIncome + res.Withdrawals)
	return nil
}

func (s *State) stepTaxes(h *config.Household, incomes []tax.Income, res *YearResult) error {
	result, err := s.householdTax(h, incomes)
	if err != nil {
		return err
	}
	taxable := 0.0
	for _, spouse := range result.Spouses {
		taxable += spouse.TaxableIncome
	}
	res.TaxableIncome = RoundCents(taxable)
	res.Tax = RoundCents(result.Total)

	// The year's after-tax cash funds the spending target. Anything above the
	// target — earnings not spent — is surplus, retained in the household
	// rather than dropped; only a cash shortfall reports spending below it.
	netCash := RoundCents(res.GrossIncome - res.Tax)
	if netCash < 0 {
		netCash = 0
	}
	res.NetSpending = math.Min(netCash, res.TargetNominal)
	if surplus := RoundCents(netCash - res.NetSpending); surplus > 0 {
		res.Surplus = surplus
		s.retainSurplus(surplus)
	}
	return nil
}

// retainSurplus deposits the year's after-tax surplus into the first
// non_registered account, raising ACB by the deposit so the retained cash is
// not taxed again as a capital gain when it is later withdrawn.
func (s *State) retainSurplus(surplus float64) {
	for _, a := range s.Accounts {
		if a.Type == config.AccountNonRegistered {
			a.Balance += surplus
			a.ACB += surplus
			return
		}
	}
}

func (s *State) stepRollForward(res *YearResult) {
	res.EndTotal = 0
	for i, a := range s.Accounts {
		a.Balance = RoundCents(a.Balance)
		res.Accounts[i].End = a.Balance
		res.EndTotal += a.Balance
	}
}

func (s *State) stepDeathEvents(res *YearResult) {
	for i := range s.People {
		p := &s.People[i]
		if p.Alive && p.DeathYear() == s.Year {
			p.Alive = false
			res.Deaths = append(res.Deaths, p.Name)
		}
	}
}

// householdTax maps the state's people and the year's income ledger onto the
// tax package's joint-household input.
func (s *State) householdTax(h *config.Household, incomes []tax.Income) (tax.Result, error) {
	spouses := make([]tax.Spouse, len(s.People))
	for i, p := range s.People {
		spouses[i] = tax.Spouse{
			Name:                 p.Name,
			Age:                  p.AgeAtDec31(s.Year),
			Income:               incomes[i],
			PensionSplitFraction: pensionSplitFraction(h, p.Name, s.Year),
		}
	}
	return tax.Compute(tax.Household{
		Year:    s.Year,
		Forward: constants.Forward{CPI: h.Assumptions.Inflation, Wage: h.Assumptions.WageGrowth},
		Spouses: spouses,
	})
}

// trialIncomes returns the year's income with a trial discretionary
// withdrawal allocated across accounts, without mutating state. The
// allocation follows the same tier order and pro-rata split the applied
// withdrawal will use, so the solve sees the same tax the year will pay.
func (s *State) trialIncomes(withdrawal float64, incomes []tax.Income) []tax.Income {
	out := append([]tax.Income(nil), incomes...)
	for i, w := range s.allocate(withdrawal) {
		s.addWithdrawalIncome(out, i, w)
	}
	return out
}

// addWithdrawalIncome adds one account's withdrawal to its owner's income.
// Registered withdrawals are fully taxable; a non-registered withdrawal
// realizes the gain fraction (1 - ACB/balance) against ACB; TFSA withdrawals
// are tax-free. It reads balances before the withdrawal is applied.
func (s *State) addWithdrawalIncome(incomes []tax.Income, accountIndex int, amount float64) {
	if amount <= 0 {
		return
	}
	a := s.Accounts[accountIndex]
	p := s.personIndex(a.Owner)
	switch a.Type {
	case config.AccountRRSP:
		incomes[p].RRSPWithdrawals += amount
	case config.AccountRRIF:
		incomes[p].RRIFWithdrawals += amount
	case config.AccountNonRegistered:
		if a.Balance > 0 {
			incomes[p].CapitalGains += amount * (1 - a.ACB/a.Balance)
		}
	}
}

func (s *State) personIndex(name string) int {
	for i := range s.People {
		if s.People[i].Name == name {
			return i
		}
	}
	return 0
}

// pensionSplitFraction returns the spouse's T1032 elected fraction for the
// year, defaulting to zero when the year is not listed.
func pensionSplitFraction(h *config.Household, name string, year int) float64 {
	for _, sp := range h.Spouses {
		if sp.Name != name {
			continue
		}
		for _, ps := range sp.PensionSplit {
			if ps.Year == year {
				return ps.Fraction
			}
		}
	}
	return 0
}
