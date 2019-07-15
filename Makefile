test:
	scripts/test.sh

lint:
	scripts/lint.sh

run-cover:
	go tool cover -html=coverage.out
