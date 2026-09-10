package projection

import (
	"math"

	"github.com/lumberbarons/retirement/internal/config"
	"github.com/lumberbarons/retirement/internal/constants"
)

type AccountYear struct {
	Name       string
	Begin      float64
	Withdrawal float64
	End        float64
}

type YearResult struct {
	Year            int
	BeginTotal      float64
	EndTotal        float64
	MandatoryIncome float64
	Withdrawals     float64
	GrossIncome     float64
	Tax             float64
	NetSpending     float64
	TargetNominal   float64
	Accounts        []AccountYear
	Deaths          []string
}

func Run(h *config.Household, startYear int) ([]YearResult, error) {
	if err := config.Validate(h); err != nil {
		return nil, err
	}
	s := NewState(h, startYear)
	last := s.secondDeathYear()
	results := make([]YearResult, 0, last-startYear+1)
	for s.Year <= last {
		results = append(results, s.stepYear(h))
		s.Year++
	}
	return results, nil
}

func (s *State) stepYear(h *config.Household) YearResult {
	res := YearResult{Year: s.Year}
	s.stepSnapshot(&res)
	s.stepReturns(h, &res)
	s.stepMandatoryIncome(&res)
	s.stepDiscretionaryWithdrawals(h, &res)
	s.stepTaxes(&res)
	s.stepRollForward(&res)
	s.stepDeathEvents(&res)
	return res
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

func (s *State) stepMandatoryIncome(res *YearResult) {
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
	}
}

func (s *State) stepDiscretionaryWithdrawals(h *config.Household, res *YearResult) {
	res.TargetNominal = s.nominalSpending(h)
	need := res.TargetNominal - res.MandatoryIncome
	if need > 0 {
		for _, typ := range []config.AccountType{
			config.AccountNonRegistered, config.AccountRRSP, config.AccountRRIF, config.AccountTFSA,
		} {
			for i, a := range s.Accounts {
				if a.Type != typ || need <= 0 {
					continue
				}
				w := math.Min(need, a.Balance)
				if w <= 0 {
					continue
				}
				a.Balance -= w
				need -= w
				res.Accounts[i].Withdrawal += w
				res.Withdrawals += w
			}
		}
	}
	res.Withdrawals = RoundCents(res.Withdrawals)
	res.GrossIncome = RoundCents(res.MandatoryIncome + res.Withdrawals)
}

func (s *State) stepTaxes(res *YearResult) {
	res.Tax = 0
	res.NetSpending = RoundCents(res.GrossIncome - res.Tax)
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

func (s *State) nominalSpending(h *config.Household) float64 {
	first := s.firstRetirementYear()
	if s.Year < first {
		return 0
	}
	nominal := RoundCents(h.Spending.TargetTodayDollars *
		math.Pow(1+h.Spending.Inflation, float64(s.Year-h.BaseYear)))
	if s.Year == first {
		return RoundCents(nominal * midYearFraction)
	}
	return nominal
}
