version = 1.1.6

define smb
	smbclient //$(shell yq -r '.cifsAddr' config.yml | cut -d: -f1)/$(1) --password "${SMB_PASSWORD}" -U "user" -c "$(2)"
endef

all: build push

build:
	docker build -t necromant/imap_mirror:$(version) .

push:
	docker push necromant/imap_mirror:$(version)

copy: test config_yml 

test:
	go run ./cmd/test/

config_yml: check_env
	$(call smb,docker,put config.yml projects\\imap_mirror\\email_backup_config.yml)

check_env:
ifndef SMB_PASSWORD
	$(error please set SMB_PASSWORD env variable)
endif