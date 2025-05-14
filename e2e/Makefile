## e2e-test: Run end to end tests via docker.
e2e-test:
	@if [ -z "$(test)" ]; then \
		echo "ERROR: 'test' variable is required. Usage: make test-e2e test=TestE2ESimple"; \
		exit 1; \
	fi
	@echo "--> Running: TestCelestiaTestSuite/$(test)"
	go test -v -run ^TestCelestiaTestSuite/$(test)$$ ./...
