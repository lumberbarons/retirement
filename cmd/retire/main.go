package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/lumberbarons/retirement/internal/config"
	"github.com/lumberbarons/retirement/internal/projection"
	"github.com/lumberbarons/retirement/internal/report"
)

const usage = `usage: retire <command> [flags]

commands:
  project    run a household projection from base year to second death
    --config <file>    household config file (YAML, default household.yaml)
    --csv <file>       write the year-by-year projection as CSV (default projection.csv)
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
		csvPath := fs.String("csv", "projection.csv", "write the year-by-year projection as CSV")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return project(*configPath, *csvPath)
	default:
		return fmt.Errorf("unknown command %q\n%s", args[0], usage)
	}
}

func project(configPath, csvPath string) error {
	h, err := config.Load(configPath)
	if err != nil {
		return err
	}
	years, err := projection.Run(h, h.BaseYear)
	if err != nil {
		return err
	}
	r := report.New(h.BaseYear, h.Assumptions.Inflation, years)
	if err := r.WriteTable(os.Stdout); err != nil {
		return err
	}
	fmt.Println()
	if err := r.WriteSummary(os.Stdout); err != nil {
		return err
	}
	f, err := os.Create(csvPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := r.WriteCSV(f); err != nil {
		return err
	}
	fmt.Printf("CSV written to %s\n", csvPath)
	return nil
}
