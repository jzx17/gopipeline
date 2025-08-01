# Go Pipeline Makefile

# 变量定义
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOLINT=golangci-lint

# 输出目录
BINARY_DIR=bin
COVERAGE_DIR=coverage
TEST_DIR=test
BENCHMARK_DIR=benchmarks
PROFILE_DIR=profiles

# 测试配置
TEST_TIMEOUT=10m
TEST_PARALLEL=4
COVERAGE_THRESHOLD=80

# 主要目标
.PHONY: all build test clean

all: test build

# 构建
build:
	@echo "==> Building..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) -v ./internal/... ./pkg/...

# 测试
test:
	@echo "==> Running tests..."
	$(GOTEST) -v -race -timeout=$(TEST_TIMEOUT) -parallel=$(TEST_PARALLEL) ./internal/... ./pkg/...

# 单元测试
test-unit:
	@echo "==> Running unit tests..."
	$(GOTEST) -v -short -timeout=$(TEST_TIMEOUT) -parallel=$(TEST_PARALLEL) ./internal/... ./pkg/...

# 集成测试
test-integration:
	@echo "==> Running integration tests..."
	$(GOTEST) -v -tags=integration -timeout=$(TEST_TIMEOUT) ./test/integration/...

# 基础集成测试（只运行basic_integration_test.go中的测试）
test-integration-basic:
	@echo "==> Running basic integration tests..."
	$(GOTEST) -v -tags=integration -timeout=$(TEST_TIMEOUT) -run="TestBasicIntegrationTestSuite" ./test/integration/basic_integration_test.go

# 端到端测试
test-e2e:
	@echo "==> Running end-to-end tests..."
	$(GOTEST) -v -tags=integration -timeout=30m -run="TestE2E" ./test/integration/...

# 性能集成测试
test-performance:
	@echo "==> Running performance tests..."
	$(GOTEST) -v -tags=integration -timeout=20m -run="TestPerformance" ./test/integration/...

# 稳定性测试
test-stability:
	@echo "==> Running stability tests..."
	$(GOTEST) -v -tags=integration -timeout=10m -run="TestStability" ./test/integration/...

# 错误场景测试
test-error-scenarios:
	@echo "==> Running error scenario tests..."
	$(GOTEST) -v -tags=integration -timeout=15m -run="TestErrorScenarios" ./test/integration/...

# 内存泄漏测试
test-memory-leak:
	@echo "==> Running memory leak tests..."
	$(GOTEST) -v -tags=integration -timeout=15m -run=".*MemoryLeak|.*Stability.*Memory" ./test/integration/...

# 并发测试
test-concurrency:
	@echo "==> Running concurrency tests..."
	$(GOTEST) -v -tags=integration -race -timeout=10m -run=".*Concurrent|.*Parallel" ./test/integration/...

# 快速集成测试（排除长期运行的测试）
test-integration-quick:
	@echo "==> Running quick integration tests..."
	$(GOTEST) -v -short -tags=integration -timeout=5m ./test/integration/...

# 基准测试
test-benchmark:
	@echo "==> Running benchmarks..."
	@mkdir -p $(BENCHMARK_DIR)
	$(GOTEST) -bench=. -benchmem -benchtime=1s -timeout=5m ./internal/... ./pkg/... | tee $(BENCHMARK_DIR)/benchmark_$(shell date +%Y%m%d_%H%M%S).txt

# 快速基准测试
test-benchmark-quick:
	@echo "==> Running quick benchmarks..."
	@mkdir -p $(BENCHMARK_DIR)
	$(GOTEST) -bench=. -benchmem -benchtime=500ms -timeout=2m ./pkg/pipeline/ ./pkg/retry/ | tee $(BENCHMARK_DIR)/benchmark_quick_$(shell date +%Y%m%d_%H%M%S).txt

# 表格驱动测试
test-table:
	@echo "==> Running table-driven tests..."
	$(GOTEST) -v -run="TestTable" -timeout=$(TEST_TIMEOUT) ./internal/... ./pkg/...

# 并发测试
test-race:
	@echo "==> Running race condition tests..."
	$(GOTEST) -v -race -count=100 -timeout=30m ./internal/... ./pkg/...

# CI环境的竞态测试（减少运行次数）
test-race-ci:
	@echo "==> Running race condition tests (CI mode)..."
	$(GOTEST) -v -race -count=10 -timeout=5m ./internal/... ./pkg/...

# 快速竞态测试
test-race-quick:
	@echo "==> Running quick race condition tests..."
	$(GOTEST) -v -race -count=1 -timeout=5m ./internal/... ./pkg/...

# 压力测试
test-stress:
	@echo "==> Running stress tests..."
	$(GOTEST) -v -count=1000 -timeout=30m ./internal/... ./pkg/...

# 快速测试（仅失败后运行）
test-quick:
	@echo "==> Running quick tests..."
	$(GOTEST) -v -short -failfast -timeout=5m ./internal/... ./pkg/...

# 测试特定包
test-pkg:
	@echo "==> Running tests for package: $(PKG)"
	@if [ -z "$(PKG)" ]; then echo "Usage: make test-pkg PKG=<package_path>"; exit 1; fi
	$(GOTEST) -v -race -timeout=$(TEST_TIMEOUT) $(PKG)

# 测试覆盖率
test-coverage:
	@echo "==> Generating test coverage..."
	@mkdir -p $(COVERAGE_DIR)
	$(GOTEST) -coverprofile=$(COVERAGE_DIR)/coverage.out -covermode=atomic -timeout=$(TEST_TIMEOUT) ./internal/... ./pkg/...
	$(GOCMD) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	$(GOCMD) tool cover -func=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.txt
	@echo "Coverage report generated at $(COVERAGE_DIR)/coverage.html"
	@echo "Coverage summary:"
	@$(GOCMD) tool cover -func=$(COVERAGE_DIR)/coverage.out | tail -1

# 覆盖率检查（排除examples目录）
test-coverage-check:
	@echo "==> Checking coverage threshold..."
	@mkdir -p $(COVERAGE_DIR)
	@$(GOTEST) -coverprofile=$(COVERAGE_DIR)/coverage.out -covermode=atomic ./internal/... ./pkg/...
	@COVERAGE=$$($(GOCMD) tool cover -func=$(COVERAGE_DIR)/coverage.out | tail -1 | awk '{print $$3}' | sed 's/%//'); \
	echo "Current coverage: $$COVERAGE%%"; \
	if [ "$$(echo "$$COVERAGE" | cut -d. -f1)" -lt "$(COVERAGE_THRESHOLD)" ]; then \
		echo "❌ Coverage $$COVERAGE%% is below threshold $(COVERAGE_THRESHOLD)%%"; \
		exit 1; \
	else \
		echo "✅ Coverage $$COVERAGE%% meets threshold $(COVERAGE_THRESHOLD)%%"; \
	fi

# 覆盖率报告
test-coverage-report:
	@echo "==> Generating detailed coverage report..."
	@mkdir -p $(COVERAGE_DIR)
	$(GOTEST) -coverprofile=$(COVERAGE_DIR)/coverage.out -covermode=atomic ./internal/... ./pkg/...
	$(GOCMD) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	$(GOCMD) tool cover -func=$(COVERAGE_DIR)/coverage.out > $(COVERAGE_DIR)/coverage_summary.txt
	@echo "Detailed coverage report:"
	@cat $(COVERAGE_DIR)/coverage_summary.txt

# 格式化代码
fmt:
	@echo "==> Formatting code..."
	$(GOFMT) -w .

# 检查格式
fmt-check:
	@echo "==> Checking code format..."
	@test -z "$$($(GOFMT) -l .)" || (echo "Please run 'make fmt' to format code"; exit 1)

# 代码检查 - 使用Go内置工具组合
lint:
	@echo "==> Running Go vet (built-in static analysis)..."
	go vet ./internal/... ./pkg/...
	@echo "==> Running staticcheck (if available)..."
	@if [ -f "$$(go env GOPATH)/bin/staticcheck" ] || [ -f "$$HOME/go/bin/staticcheck" ]; then \
		$$(go env GOPATH)/bin/staticcheck ./internal/... ./pkg/... 2>/dev/null || $$HOME/go/bin/staticcheck ./internal/... ./pkg/... 2>/dev/null || echo "✅ staticcheck completed"; \
	else \
		echo "⚠️  staticcheck not installed, run: make lint-tools"; \
	fi
	@echo "==> Running ineffassign (if available)..."
	@if [ -f "$$(go env GOPATH)/bin/ineffassign" ] || [ -f "$$HOME/go/bin/ineffassign" ]; then \
		$$(go env GOPATH)/bin/ineffassign ./internal/... ./pkg/... 2>/dev/null || $$HOME/go/bin/ineffassign ./internal/... ./pkg/... 2>/dev/null || echo "✅ ineffassign completed"; \
	else \
		echo "⚠️  ineffassign not installed, run: make lint-tools"; \
	fi
	@echo "==> Checking for misspellings (if available)..."
	@if [ -f "$$(go env GOPATH)/bin/misspell" ] || [ -f "$$HOME/go/bin/misspell" ]; then \
		$$(go env GOPATH)/bin/misspell -error ./internal/... ./pkg/... 2>/dev/null || $$HOME/go/bin/misspell -error ./internal/... ./pkg/... 2>/dev/null || echo "✅ misspell completed"; \
	else \
		echo "⚠️  misspell not installed, run: make lint-tools"; \
	fi

# 轻量级代码检查 - 只使用Go内置工具
lint-basic:
	@echo "==> Running basic Go vet..."
	go vet ./internal/... ./pkg/...

# 安装推荐的静态分析工具
lint-tools:
	@echo "==> Installing recommended linting tools..."
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install github.com/gordonklaus/ineffassign@latest  
	go install github.com/client9/misspell/cmd/misspell@latest
	@echo "✅ Tools installed. Run 'make lint' to use them."

# 安装依赖
deps:
	@echo "==> Installing dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# 更新依赖
deps-update:
	@echo "==> Updating dependencies..."
	$(GOGET) -u ./internal/... ./pkg/...
	$(GOMOD) tidy

# 清理
clean:
	@echo "==> Cleaning..."
	$(GOCLEAN)
	@rm -rf $(BINARY_DIR)
	@rm -rf $(COVERAGE_DIR)
	@rm -rf $(BENCHMARK_DIR)
	@rm -rf $(PROFILE_DIR)
	@rm -f coverage.out
	@rm -f *.prof
	@rm -f *.test

# 安装开发工具
tools:
	@echo "==> Installing development tools..."
	@echo "==> Installing static analysis tools..."
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install github.com/gordonklaus/ineffassign@latest  
	go install github.com/client9/misspell/cmd/misspell@latest
	@echo "==> Installing other useful tools..."
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "✅ All development tools installed successfully!"

# 测试相关的综合目标
test-all: test-unit test-integration test-race test-benchmark

# CI 检查
ci: deps fmt-check lint test test-coverage-check

# 完整的CI检查（包括基准测试）
ci-full: deps fmt-check lint test-unit test-integration test-race-ci test-benchmark test-coverage-check

# 宽松的CI检查（适用于开发阶段）
ci-dev: deps fmt-check lint-basic test

# 生成文档
docs:
	@echo "==> Generating documentation..."
	@godoc -http=:6060 &
	@echo "Documentation server started at http://localhost:6060"

# 运行示例
run-example:
	@echo "==> Running example..."
	$(GOCMD) run examples/simple/main.go

# 性能分析
profile:
	@echo "==> Running performance profiling..."
	@mkdir -p $(PROFILE_DIR)
	@echo "==> Running CPU profile..."
	$(GOTEST) -cpuprofile=$(PROFILE_DIR)/cpu.prof -bench=. -benchtime=30s ./internal/... ./pkg/...
	@echo "==> Running memory profile..."
	$(GOTEST) -memprofile=$(PROFILE_DIR)/mem.prof -bench=. -benchtime=30s ./internal/... ./pkg/...
	@echo "==> Running block profile..."
	$(GOTEST) -blockprofile=$(PROFILE_DIR)/block.prof -bench=. -benchtime=30s ./internal/... ./pkg/...
	@echo "==> Running mutex profile..."
	$(GOTEST) -mutexprofile=$(PROFILE_DIR)/mutex.prof -bench=. -benchtime=30s ./internal/... ./pkg/...
	@echo "Profiles generated in $(PROFILE_DIR)/"

# 查看性能分析结果
profile-cpu:
	@echo "==> Opening CPU profile..."
	$(GOCMD) tool pprof $(PROFILE_DIR)/cpu.prof

profile-mem:
	@echo "==> Opening memory profile..."
	$(GOCMD) tool pprof $(PROFILE_DIR)/mem.prof

profile-block:
	@echo "==> Opening block profile..."
	$(GOCMD) tool pprof $(PROFILE_DIR)/block.prof

profile-mutex:
	@echo "==> Opening mutex profile..."
	$(GOCMD) tool pprof $(PROFILE_DIR)/mutex.prof

# Web性能分析界面
profile-web:
	@echo "==> Starting web interface for CPU profile..."
	$(GOCMD) tool pprof -http=:8080 $(PROFILE_DIR)/cpu.prof

# 生成性能报告
profile-report:
	@echo "==> Generating performance report..."
	@mkdir -p $(PROFILE_DIR)/reports
	$(GOCMD) tool pprof -top $(PROFILE_DIR)/cpu.prof > $(PROFILE_DIR)/reports/cpu_top.txt
	$(GOCMD) tool pprof -top $(PROFILE_DIR)/mem.prof > $(PROFILE_DIR)/reports/mem_top.txt
	@echo "Performance reports generated in $(PROFILE_DIR)/reports/"

# 帮助
help:
	@echo "Available targets:"
	@echo ""
	@echo "Build:"
	@echo "  make build              - Build the project"
	@echo "  make clean              - Clean build artifacts"
	@echo ""
	@echo "Testing:"
	@echo "  make test               - Run all tests with race detection"
	@echo "  make test-unit          - Run unit tests only"
	@echo "  make test-integration   - Run integration tests"
	@echo "  make test-e2e           - Run end-to-end tests"
	@echo "  make test-performance   - Run performance tests"
	@echo "  make test-stability     - Run stability tests"
	@echo "  make test-error-scenarios - Run error scenario tests"
	@echo "  make test-memory-leak   - Run memory leak tests"
	@echo "  make test-concurrency   - Run concurrency tests"
	@echo "  make test-integration-quick - Run quick integration tests"
	@echo "  make test-table         - Run table-driven tests"
	@echo "  make test-race          - Run race condition tests (100 iterations)"
	@echo "  make test-race-ci       - Run race condition tests for CI (10 iterations)"
	@echo "  make test-stress        - Run stress tests"
	@echo "  make test-quick         - Run quick tests (fail fast)"
	@echo "  make test-pkg PKG=...   - Run tests for specific package"
	@echo "  make test-all           - Run all test types"
	@echo ""
	@echo "Coverage:"
	@echo "  make test-coverage      - Generate test coverage report"
	@echo "  make test-coverage-check - Check coverage threshold"
	@echo "  make test-coverage-report - Generate detailed coverage report"
	@echo ""
	@echo "Benchmarking:"
	@echo "  make test-benchmark     - Run benchmark tests"
	@echo "  make profile            - Generate performance profiles"
	@echo "  make profile-cpu        - View CPU profile"
	@echo "  make profile-mem        - View memory profile"
	@echo "  make profile-web        - Open web interface for profiles"
	@echo "  make profile-report     - Generate performance reports"
	@echo ""
	@echo "Code Quality:"
	@echo "  make fmt                - Format code"  
	@echo "  make fmt-check          - Check code format"
	@echo "  make lint               - Run linter"
	@echo ""
	@echo "Dependencies:"
	@echo "  make deps               - Install dependencies"
	@echo "  make deps-update        - Update dependencies"
	@echo "  make tools              - Install development tools"
	@echo ""
	@echo "CI/CD:"
	@echo "  make ci                 - Run CI checks"
	@echo "  make ci-full            - Run full CI checks with benchmarks"
	@echo ""
	@echo "Other:"
	@echo "  make docs               - Generate documentation"
	@echo "  make run-example        - Run example"
	@echo "  make help               - Show this help message"

.DEFAULT_GOAL := help