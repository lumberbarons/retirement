package config

type Household struct {
	BaseYear    int         `yaml:"base_year"`
	Province    string      `yaml:"province"`
	Spouses     []Spouse    `yaml:"spouses"`
	Accounts    []Account   `yaml:"accounts"`
	Spending    Spending    `yaml:"spending"`
	Assumptions Assumptions `yaml:"assumptions"`
}

type Spouse struct {
	Name          string     `yaml:"name"`
	BirthYear     int        `yaml:"birth_year"`
	DeathAge      int        `yaml:"death_age"`
	RetirementAge int        `yaml:"retirement_age"`
	CPP           CPP        `yaml:"cpp"`
	OAS           OAS        `yaml:"oas"`
	Pension       *DBPension `yaml:"pension"`
}

type CPP struct {
	MonthlyAt65 float64 `yaml:"monthly_at_65"`
	StartAge    int     `yaml:"start_age"`
}

type OAS struct {
	StartAge int `yaml:"start_age"`
}

type DBPension struct {
	AccrualRate          float64 `yaml:"accrual_rate"`
	YearsOfService       float64 `yaml:"years_of_service"`
	FinalAverageEarnings float64 `yaml:"final_average_earnings"`
	StartAge             int     `yaml:"start_age"`
	BridgeMonthly        float64 `yaml:"bridge_monthly"`
	Indexation          string  `yaml:"indexation"`
	IndexationRate      float64 `yaml:"indexation_rate"`
	SurvivorPercent     float64 `yaml:"survivor_percent"`
	JSReductionFactor   float64 `yaml:"js_reduction_factor"`
}

type AccountType string

const (
	AccountTFSA          AccountType = "tfsa"
	AccountRRSP          AccountType = "rrsp"
	AccountRRIF          AccountType = "rrif"
	AccountNonRegistered AccountType = "non_registered"
)

type SpousalRRSP struct {
	Contributor       string `yaml:"contributor"`
	ContributionYears []int  `yaml:"contribution_years"`
}

type Account struct {
	Name                  string        `yaml:"name"`
	Type                  AccountType   `yaml:"type"`
	Owner                 string        `yaml:"owner"`
	Balance               float64       `yaml:"balance"`
	ACB                   float64       `yaml:"acb"`
	Spousal               *SpousalRRSP  `yaml:"spousal"`
	YoungerSpouseElection bool          `yaml:"younger_spouse_election"`
}

type Spending struct {
	TargetTodayDollars float64 `yaml:"target_today_dollars"`
	Mode               string  `yaml:"mode"`
	Inflation          float64 `yaml:"inflation"`
	SurvivorFactor     float64 `yaml:"survivor_factor"`
}

type Assumptions struct {
	Inflation       float64 `yaml:"inflation"`
	WageGrowth      float64 `yaml:"wage_growth"`
	PortfolioReturn float64 `yaml:"portfolio_return"`
}

func (h *Household) SpouseNames() []string {
	names := make([]string, 0, len(h.Spouses))
	for _, s := range h.Spouses {
		names = append(names, s.Name)
	}
	return names
}
