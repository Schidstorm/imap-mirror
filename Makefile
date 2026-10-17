version = $(shell tomlq -r '.tool.commitizen.version' .cz.toml)

all: test build push deploy

build:
	docker build -t registry.schidlow.ski/imap_mirror:$(version) .

push:
	docker push registry.schidlow.ski/imap_mirror:$(version)
	
deploy:
	cat application.yaml | VERSION=$(version) envsubst | kubectl apply -f -

test:
	go run ./cmd/test/