package projection

import (
	"math"

	"github.com/lumberbarons/retirement/internal/benefits"
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

// SpouseYear is one spouse's share of a projected year: the cash income the
// year attributes to them and the tax they bear. The measure matches the
// household's GrossIncome — CPP, OAS, RRIF minimums, and withdrawals from
// accounts they own — so the spouse rows reconcile with the household total.
type SpouseYear struct {
	Name               string
	GrossIncome        float64
	TaxableIncome      float64
	Tax                float64
	CPPContributions   float64
	Surplus            float64
	UnallocatedSurplus float64
}

type YearResult struct {
	Year               int
	BeginTotal         float64
	EndTotal           float64
	MandatoryIncome    float64
	EmploymentIncome   float64
	CPPContributions   float64
	Surplus            float64
	UnallocatedSurplus float64
	CPP                float64
	OAS                float64
	OASRecovery        float64
	GIS                float64
	Withdrawals        float64
	GrossIncome        float64
	TaxableIncome      float64
	Tax                float64
	NetSpending        float64
	TargetNominal      float64
	Accounts           []AccountYear
	Spouses            []SpouseYear
	Deaths             []string
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
	if err := s.stepMandatoryIncome(h, incomes, &res); err != nil {
		return YearResult{}, err
	}
	if err := s.stepEmploymentIncome(h, incomes, &res); err != nil {
		return YearResult{}, err
	}
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
	res.Spouses = make([]SpouseYear, len(s.People))
	for i, p := range s.People {
		res.Spouses[i] = SpouseYear{Name: p.Name}
	}
}

func (s *State) stepReturns(h *config.Household, res *YearResult) {
	for _, a := range s.Accounts {
		a.Balance *= 1 + h.Assumptions.PortfolioReturn
	}
}

func (s *State) stepMandatoryIncome(h *config.Household, incomes []tax.Income, res *YearResult) error {
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
	return s.addGovernmentBenefits(h, incomes, res)
}

// addGovernmentBenefits records each alive spouse's CPP and OAS for the year
// — zero before their start ages — adds them to the year's mandatory income,
// and applies the user-set CPP sharing election. The survivor top-up is
// applied by the death-event wiring, not here.
func (s *State) addGovernmentBenefits(h *config.Household, incomes []tax.Income, res *YearResult) error {
	f := s.forward(h)
	cpp := make([]float64, len(s.People))
	oas := make([]float64, len(s.People))
	for i := range s.People {
		p := &s.People[i]
		if !p.Alive {
			continue
		}
		v, err := benefits.CPPAnnual(p.CPPMonthlyAt65, p.CPPStartAge, p.BirthYear, s.Year, h.BaseYear, f)
		if err != nil {
			return err
		}
		cpp[i] = v
		if oas[i], err = benefits.OASAnnual(p.OASStartAge, p.BirthYear, s.Year, f); err != nil {
			return err
		}
	}
	if err := s.applyCPPSharing(h, cpp); err != nil {
		return err
	}
	for i := range s.People {
		if !s.People[i].Alive {
			continue
		}
		incomes[i].CPP += cpp[i]
		incomes[i].OAS += oas[i]
		res.CPP += cpp[i]
		res.OAS += oas[i]
	}
	res.CPP = RoundCents(res.CPP)
	res.OAS = RoundCents(res.OAS)
	res.MandatoryIncome = RoundCents(res.MandatoryIncome + res.CPP + res.OAS)
	return nil
}

// applyCPPSharing pools the elected fraction of the couple's CPP retirement
// pensions and re-splits it equally. The election only applies while both
// spouses are alive and past their own CPP start age.
func (s *State) applyCPPSharing(h *config.Household, cpp []float64) error {
	if h.CPPSharingFraction == 0 || len(cpp) != 2 {
		return nil
	}
	for i := range s.People {
		p := &s.People[i]
		if !p.Alive || s.Year < p.BirthYear+p.CPPStartAge {
			return nil
		}
	}
	first, second, err := benefits.ShareCPP(cpp[0], cpp[1], h.CPPSharingFraction)
	if err != nil {
		return err
	}
	cpp[0], cpp[1] = first, second
	return nil
}

// stepEmploymentIncome posts each spouse's earned income for the year and the
// employee CPP/CPP2 contributions withheld from it. The configured amount is
// in base-year dollars and grows with the wage-growth assumption: a full year
// through the year before retirement, half a year in the retirement year, and
// nothing after. Income and contributions also stop with the spouse. The
// contribution's base portion becomes a tax credit and its enhanced portion a
// deduction when the year's tax is computed.
func (s *State) stepEmploymentIncome(h *config.Household, incomes []tax.Income, res *YearResult) error {
	forward := s.forward(h)
	for i := range h.Spouses {
		sp := &h.Spouses[i]
		if sp.EmploymentIncome <= 0 {
			continue
		}
		p := &s.People[i]
		if !p.Alive || s.Year > p.RetirementYear() {
			continue
		}
		fraction := 1.0
		if s.Year == p.RetirementYear() {
			fraction = midYearFraction
		}
		amount := RoundCents(constants.ForwardIndex(sp.EmploymentIncome, constants.BasisAverageWage,
			h.BaseYear, s.Year, forward) * fraction)
		base, enhanced, err := benefits.CPPEmployeeContributions(amount, fraction, s.Year, forward)
		if err != nil {
			return err
		}
		incomes[i].Employment += amount
		incomes[i].CPPContributions += base + enhanced
		incomes[i].CPPBaseContributions += base
		res.EmploymentIncome += amount
		res.CPPContributions += base + enhanced
	}
	res.EmploymentIncome = RoundCents(res.EmploymentIncome)
	res.CPPContributions = RoundCents(res.CPPContributions)
	return nil
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
		recovery, gis, err := s.benefitAdjustments(h, trial, result)
		if err != nil {
			solveErr = err
			return 0
		}
		return res.MandatoryIncome + res.EmploymentIncome + withdrawal -
			result.Total - sum(recovery) + sum(gis) - res.CPPContributions
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
	recovery, gis, err := s.benefitAdjustments(h, incomes, result)
	if err != nil {
		return err
	}
	taxable := 0.0
	spouseNet := make([]float64, len(s.People))
	for i, spouse := range result.Spouses {
		taxable += spouse.TaxableIncome
		// Gross income uses the same measure as the household total:
		// employment income, CPP, OAS, and every withdrawal from an account
		// the spouse owns.
		gross := incomes[i].Employment + incomes[i].CPP + incomes[i].OAS
		for j, a := range s.Accounts {
			if a.Owner == s.People[i].Name {
				gross += res.Accounts[j].Withdrawal
			}
		}
		res.Spouses[i].GrossIncome = RoundCents(gross)
		res.Spouses[i].TaxableIncome = RoundCents(spouse.TaxableIncome)
		res.Spouses[i].Tax = RoundCents(spouse.TotalTax)
		res.Spouses[i].CPPContributions = RoundCents(incomes[i].CPPContributions)
		// Each spouse's after-tax cash positions their share of any surplus:
		// the household's cash is pooled to fund the target, and what remains
		// belongs to the spouses in proportion to what they contributed.
		net := gross - spouse.TotalTax - recovery[i] + gis[i] - incomes[i].CPPContributions
		if net < 0 {
			net = 0
		}
		spouseNet[i] = net
	}
	res.TaxableIncome = RoundCents(taxable)
	res.Tax = RoundCents(result.Total)
	res.OASRecovery = RoundCents(sum(recovery))
	res.GIS = RoundCents(sum(gis))

	// The year's after-tax cash funds the spending target. Anything above the
	// target — earnings not spent — is surplus, retained in the household
	// rather than dropped; only a cash shortfall reports spending below it.
	netCash := RoundCents(res.GrossIncome - res.Tax - res.OASRecovery + res.GIS - res.CPPContributions)
	if netCash < 0 {
		netCash = 0
	}
	res.NetSpending = math.Min(netCash, res.TargetNominal)
	// A year topped up by a discretionary withdrawal is at the target by
	// construction; any cent-level excess is rounding noise, not retained
	// surplus. Only a year funded without withdrawals can run a surplus.
	if res.Withdrawals == 0 {
		if surplus := RoundCents(netCash - res.NetSpending); surplus > 0 {
			res.Surplus = surplus
			s.retainSurplus(h, spouseNet, res, surplus)
		}
	}
	return nil
}

// retainSurplus places each spouse's share of the year's surplus in a
// non_registered destination — the savings_account declared in config, else
// the first non_registered account they own — so later growth is attributed
// to the taxpayer whose earnings generated it. A share with no destination is
// reported as unallocated rather than dropped silently.
func (s *State) retainSurplus(h *config.Household, spouseNet []float64, res *YearResult, surplus float64) {
	householdNet := 0.0
	last := -1
	for i, net := range spouseNet {
		householdNet += net
		if net > 0 {
			last = i
		}
	}
	// No spouse's cash positions the surplus (a rounding-boundary year): it
	// cannot be attributed, so report it whole rather than lose it.
	if householdNet <= 0 {
		res.UnallocatedSurplus = RoundCents(res.UnallocatedSurplus + surplus)
		return
	}
	allocated := 0.0
	for i, net := range spouseNet {
		share := 0.0
		if householdNet > 0 && net > 0 {
			if i == last {
				share = RoundCents(surplus - allocated)
			} else {
				share = RoundCents(surplus * net / householdNet)
				allocated += share
			}
		}
		if share <= 0 {
			continue
		}
		res.Spouses[i].Surplus = share
		index := s.surplusDestination(h, s.People[i].Name)
		if index < 0 {
			res.Spouses[i].UnallocatedSurplus = share
			res.UnallocatedSurplus += share
			continue
		}
		account := s.Accounts[index]
		account.Balance += share
		account.ACB += share
	}
	res.UnallocatedSurplus = RoundCents(res.UnallocatedSurplus)
}

// surplusDestination returns the account that receives a spouse's retained
// surplus: the savings_account declared in config, else the first
// non_registered account they own, else -1 when there is nowhere to put it.
func (s *State) surplusDestination(h *config.Household, owner string) int {
	for i := range h.Spouses {
		if h.Spouses[i].Name != owner || h.Spouses[i].SavingsAccount == "" {
			continue
		}
		for j, a := range s.Accounts {
			if a.Name == h.Spouses[i].SavingsAccount {
				return j
			}
		}
	}
	for j, a := range s.Accounts {
		if a.Owner == owner && a.Type == config.AccountNonRegistered {
			return j
		}
	}
	return -1
}

// benefitAdjustments returns the year's OAS recovery tax and non-taxable GIS
// for each spouse, given the income ledger and tax result, so cash-flow
// attribution can hold each spouse to their own claws and credits. The
// recovery is 15% of each spouse's net income over the threshold, capped at
// the pension received; net income is approximated by taxable income and the
// recovery is taken in the same year rather than on the real July-to-June
// cycle. GIS is evaluated for each spouse receiving OAS, using the single
// test once one spouse has died.
func (s *State) benefitAdjustments(h *config.Household, incomes []tax.Income, result tax.Result) (recovery, gis []float64, err error) {
	f := s.forward(h)
	single := s.survivors() == 1
	recovery = make([]float64, len(s.People))
	gis = make([]float64, len(s.People))
	for i := range s.People {
		p := &s.People[i]
		if !p.Alive {
			continue
		}
		netIncome := result.Spouses[i].TaxableIncome
		recovery[i], err = benefits.OASRecovery(incomes[i].OAS, netIncome, s.Year, f)
		if err != nil {
			return nil, nil, err
		}
		if s.Year < p.BirthYear+p.OASStartAge {
			continue
		}
		gis[i], err = benefits.GISAnnual(benefits.GISInput{
			Year:        s.Year,
			Forward:     f,
			Single:      single,
			OtherIncome: math.Max(0, netIncome-incomes[i].OAS),
		})
		if err != nil {
			return nil, nil, err
		}
	}
	return recovery, gis, nil
}

func sum(values []float64) float64 {
	total := 0.0
	for _, v := range values {
		total += v
	}
	return total
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

// forward is the year's indexation input for the dated constants: the
// configured CPI and wage-growth assumptions.
func (s *State) forward(h *config.Household) constants.Forward {
	return constants.Forward{CPI: h.Assumptions.Inflation, Wage: h.Assumptions.WageGrowth}
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
		Forward: s.forward(h),
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
