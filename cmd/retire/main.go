package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/lumberbarons/retirement/internal/config"
)

const usage = `usage: retire <command> [flags]

commands:
  project    load and validate a household config
    --config <file>    household config file (YAML, default household.yaml)
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "retire:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command given\n%s", usage)
	}
	switch args[0] {
	case "project":
		fs := flag.NewFlagSet("project", flag.ContinueOnError)
		configPath := fs.String("config", "household.yaml", "household config file (YAML)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return project(*configPath)
	default:
		return fmt.Errorf("unknown command %q\n%s", args[0], usage)
	}
}

func project(configPath string) error {
	h, err := config.Load(configPath)
	if err != nil {
		return err
	}
	fmt.Printf("household loaded from %s\n", configPath)
	fmt.Printf("base year:   %d (%s)\n", h.BaseYear, h.Province)
	for _, s := range h.Spouses {
		fmt.Printf("spouse:      %s, born %d, retires at %d, dies at %d\n", s.Name, s.BirthYear, s.RetirementAge, s.DeathAge)
	}
	fmt.Printf("accounts:    %d\n", len(h.Accounts))
	for _, a := range h.Accounts {
		fmt.Printf("  %-20s %-14s %-6s $%.2f\n", a.Name, a.Type, a.Owner, a.Balance)
	}
	fmt.Printf("spending:    $%.2f today's dollars (%s)\n", h.Spending.TargetTodayDollars, h.Spending.Mode)
	fmt.Printf("assumptions: return %.2f%%, inflation %.2f%%, wage growth %.2f%%\n",
		h.Assumptions.PortfolioReturn*100, h.Assumptions.Inflation*100, h.Assumptions.WageGrowth*100)
	return nil
}
