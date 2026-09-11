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
	// The tax engine (US4) has not landed, so a discretionary withdrawal
	// currently passes through untaxed and the gross-vs-net solve reduces to
	// net = mandatory income + withdrawal. Plugging in the real tax function
	// here is the seam that turns this back into a genuine gross-vs-net solve.
	net := func(withdrawal float64) float64 {
		return res.MandatoryIncome + withdrawal
	}
	withdrawal := RoundCents(SolveGross(res.TargetNominal, s.withdrawalCapacity(), net))
	if withdrawal > 0 {
		s.applyWithdrawals(s.allocate(withdrawal), res)
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
