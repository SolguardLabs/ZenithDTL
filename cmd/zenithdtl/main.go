package main

import (
	"encoding/json"
	"fmt"
	"os"

	zenithdtl "github.com/solguardlabs/zenithdtl/src"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usage()
	}

	switch args[0] {
	case "list":
		return list(args[1:])
	case "run":
		return runScenario(args[1:])
	case "validate":
		return validateScenario(args[1:])
	default:
		return usage()
	}
}

func usage() error {
	return fmt.Errorf("usage: zenithdtl list [--json] | run <scenario> [--json] [--events] | validate <scenario>")
}

func list(args []string) error {
	scenarios := zenithdtl.ListScenarios()
	if zenithdtl.ParseBoolFlag(args, "--json") {
		return writeJSON(scenarios)
	}
	for _, scenario := range scenarios {
		fmt.Println(scenario.Name)
	}
	return nil
}

func runScenario(args []string) error {
	values := zenithdtl.WithoutFlags(args)
	if len(values) != 1 {
		return fmt.Errorf("usage: zenithdtl run <scenario> [--json] [--events]")
	}
	report, err := zenithdtl.RunScenarioReport(values[0], zenithdtl.ParseBoolFlag(args, "--events"))
	if err != nil {
		return err
	}
	return writeJSON(report)
}

func validateScenario(args []string) error {
	values := zenithdtl.WithoutFlags(args)
	if len(values) != 1 {
		return fmt.Errorf("usage: zenithdtl validate <scenario>")
	}
	result := zenithdtl.ValidateScenario(values[0])
	if err := result.Error(); err != nil {
		return err
	}
	fmt.Printf("ok %s\n", values[0])
	return nil
}

func writeJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
