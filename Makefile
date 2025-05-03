version = 1.0.6

define smb
	smbclient //$(shell yq -r '.cifsAddr' config.yml | cut -d: -f1)/$(1) --password "${SMB_PASSWORD}" -U "user" -c "$(2)"
endef

all: build

build:
	docker build -t  necromant/imap_mirror:$(version) . && docker save necromant/imap_mirror:$(version) > necromant_imap_mirror.tar

copy: test docker_compose_yml config_yml filter_lua

test:
	go run ./cmd/test/

docker_compose_yml: check_env
	$(call smb,docker,put docker-compose.yml projects\\imap_mirror\\docker-compose.yml)

config_yml: check_env
	$(call smb,docker,put config.yml projects\\imap_mirror\\email_backup_config.yml)

filter_lua: check_env
	$(call smb,backup,put filter.lua filter\\scripts\\filter.lua)

check_env:
ifndef SMB_PASSWORD
	$(error please set SMB_PASSWORD env variable)
endif