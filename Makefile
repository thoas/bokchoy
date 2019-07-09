test:
	scripts/test.sh

run-cover:
	go tool cover -html=coverage.out
