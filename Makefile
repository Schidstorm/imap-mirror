all: build

build:
	cd cmd/mirror && GOOS=linux GOARCH=arm GOARM=5 go build -o mirror .