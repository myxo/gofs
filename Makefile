run_rapid_test:
	go test -v -rapid.checks=50000 ./...

run_concurrent_test:
	go test -v -race -run=TestMultithreading ./memory_test/

run_bench:
	go test -run=^# -bench=. -count=10 ./...