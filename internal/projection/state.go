package projection

import (
	"math"

	"github.com/lumberbarons/retirement/internal/config"
)

const midYearFraction = 0.5

func RoundCents(v float64) float64 {
	return math.Round(v*100) / 100
}

type Person struct {
	Name          string
	BirthYear     int
	DeathAge      int
	RetirementAge int
	Alive         bool
}

func (p Person) AgeAtJan1(year int) int {
	return year - p.BirthYear - 1
}

func (p Person) AgeAtDec31(year int) int {
	return year - p.BirthYear
}

func (p Person) RetirementYear() int {
	return p.BirthYear + p.RetirementAge
}

func (p Person) DeathYear() int {
	return p.BirthYear + p.DeathAge
}

type AccountState struct {
	Name                  string
	Type                  config.AccountType
	Owner                 string
	Balance               float64
	ACB                   float64
	Spousal               *config.SpousalRRSP
	YoungerSpouseElection bool
}

type State struct {
	Year     int
	People   []Person
	Accounts []*AccountState
}

func NewState(h *config.Household, startYear int) *State {
	s := &State{Year: startYear}
	for _, sp := range h.Spouses {
		s.People = append(s.People, Person{
			Name:          sp.Name,
			BirthYear:     sp.BirthYear,
			DeathAge:      sp.DeathAge,
			RetirementAge: sp.RetirementAge,
			Alive:         startYear <= sp.BirthYear+sp.DeathAge,
		})
	}
	for i := range h.Accounts {
		a := &h.Accounts[i]
		s.Accounts = append(s.Accounts, &AccountState{
			Name:                  a.Name,
			Type:                  a.Type,
			Owner:                 a.Owner,
			Balance:               a.Balance,
			ACB:                   a.ACB,
			Spousal:               a.Spousal,
			YoungerSpouseElection: a.YoungerSpouseElection,
		})
	}
	return s
}

func (s *State) person(name string) *Person {
	for i := range s.People {
		if s.People[i].Name == name {
			return &s.People[i]
		}
	}
	return nil
}

func (s *State) firstRetirementYear() int {
	first := 0
	for i, p := range s.People {
		y := p.RetirementYear()
		if i == 0 || y < first {
			first = y
		}
	}
	return first
}

func (s *State) secondDeathYear() int {
	last := 0
	for _, p := range s.People {
		if p.DeathYear() > last {
			last = p.DeathYear()
		}
	}
	return last
}
