package main

import (
	"log"
	"os"
	"strings"
)

const (
	seed = 42
)

type TestFunc func(*log.Logger) error

type Test struct {
	Name string
	Func TestFunc
}

func main() {
	logger := log.New(os.Stdout, "test-e2e", log.LstdFlags)

	tests := []Test{
		{"E2ESimple", E2ESimple},
	}

	// check if a specific test is passed and run it
	specificTestFound := false
	for _, arg := range os.Args[1:] {
		for _, test := range tests {
			if test.Name == arg {
				runTest(logger, test)
				specificTestFound = true
				break
			}
		}
	}

	if !specificTestFound {
		logger.Println("No particular test specified. Running all tests.")
		logger.Println("make test-e2e <test_name> to run a specific test")
		logger.Printf("Valid tests are: %s\n\n", getTestNames(tests))
		// if no specific test is passed, run all tests
		for _, test := range tests {
			runTest(logger, test)
		}
	}
}

func runTest(logger *log.Logger, test Test) {
	logger.Printf("=== RUN %s", test.Name)
	err := test.Func(logger)
	if err != nil {
		logger.Fatalf("--- ERROR %s: %v", test.Name, err)
	}
	logger.Printf("--- ✅ PASS: %s \n\n", test.Name)
}

func getTestNames(tests []Test) string {
	testNames := make([]string, 0, len(tests))
	for _, test := range tests {
		testNames = append(testNames, test.Name)
	}
	return strings.Join(testNames, ", ")
}
