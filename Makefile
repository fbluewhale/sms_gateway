.PHONY: swagger test vet build

swagger:
	swag init -g cmd/api-gateway/main.go -o docs/swagger --parseInternal

test:
	go test ./...

vet:
	go vet ./...

build: swagger
	go build ./...
