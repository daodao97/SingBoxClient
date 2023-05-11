fmt:
	@gofumpt -l -w .
	@gofmt -s -w .
	@gci write --custom-order -s "standard,prefix(github.com/sagernet/),default" .

fmt_install:
	go install -v mvdan.cc/gofumpt@latest
	go install -v github.com/daixiang0/gci@latest

lint:
	GOOS=linux golangci-lint run ./...
	GOOS=android golangci-lint run ./...
	GOOS=windows golangci-lint run ./...
	GOOS=darwin golangci-lint run ./...
	GOOS=freebsd golangci-lint run ./...

lint_install:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

test:
	go test -v ./...