MAKEFILE_NAME:=$(notdir $(lastword $(MAKEFILE_LIST)))
TARGET=$(notdir $(abspath $(lastword $(MAKEFILE_LIST)/..)))
MAKETARGETS=$(filter-out $@,$(MAKECMDGOALS))

-include .makerc
-include ~/.makerc
include variables.mk

## Default Domain name
DOMAIN ?= localhost

## Default private key size
SIZE ?= 4096

## Default CA days duration
CANBDAYS ?= 3650

## Default certificate days duration
NBDAYS ?= 3650

## Country code
C ?= FR

## State
ST ?= France

## Location
L ?= Paris

## Organization
O ?= Orga

## Organization Unit
OU ?= Unit

## CA common name
CACN ?= EasyCA

## Alternative names (space separator)
ALTNAME ?= localhost

## IP addresses (space separator)
IP ?=

## Openssl path
OPENSSL?= $(shell which openssl 2> /dev/null)

MAKER=$(shell test -n "$${USER}" && echo $${USER} || { test -n "$${USERNAME}" && echo $${USERNAME} || { which logname > /dev/null 2>&1 && logname || echo Unknown ; } } )

# detect if running under unix by finding 'uname' in $PATH :
PLATFORM :=$(shell uname)
ifeq ($(PLATFORM),Linux)
# Linux stuff
WINMODE?=0
else
# Windows stuff
WINMODE?=1
endif

## convinient variable to define current date/time which can be used for build time
NOW=$(shell date --rfc-3339=seconds)
export NOW
DEBUG = printf -- "| $$(date --rfc-3339=seconds) | %-s | %-5.5s | %-s |\n" "$(subst $(ROOT),$(PROJECT_NAME),$(shell pwd)/$(MAKEFILE_NAME))" "DEBUG"
INFO = printf -- "\e[92m| $$(date --rfc-3339=seconds) | %-s | %-5.5s | %-s |\e[39m\n" "$(subst $(ROOT),$(PROJECT_NAME),$(shell pwd)/$(MAKEFILE_NAME))" "INFO"
WARN = printf -- "\e[93m| $$(date --rfc-3339=seconds) | %-s | %-5.5s | %-s |\e[39m\n" "$(subst $(ROOT),$(PROJECT_NAME),$(shell pwd)/$(MAKEFILE_NAME))" "WARN"
ERROR = printf -- "\e[91m| $$(date --rfc-3339=seconds) | %-s | %-5.5s | %-s |\e[39m\n" "$(subst $(ROOT),$(PROJECT_NAME),$(shell pwd)/$(MAKEFILE_NAME))" "ERROR"
FATAL = printf -- "\e[31m| $$(date --rfc-3339=seconds) | %-s | %-5.5s | %-s |\e[39m\n" "$(subst $(ROOT),$(PROJECT_NAME),$(shell pwd)/$(MAKEFILE_NAME))" "FATAL"

## Show this help prompt.
.PHONY: all
all : help
	@:

## Show this help prompt.
.PHONY: help
help:
	@ echo '  Easy server certificate generator'
	@ echo
	@ echo '  Usage:'
	@ echo ''
	@ echo '    [flags...] make <target>'
	@ echo ''
	@ echo '  Targets:'
	@ echo ''
	@ (echo '   Name:Description'; echo '   ----:-----------'; (awk -F: '/^## /{ comment = substr($$0,4) } comment && /^[a-zA-Z][a-zA-Z ]*[^ ]+:/{ print "   " substr($$1,0,80) ":" comment }' $(MAKEFILE_LIST) | sort -d)) | column -t -s ':'
	@ echo ''
	@ echo '  Flags:'
	@ echo ''
	@ (echo '   Name?=Default value?=Description'; echo '   ----?=-------------?=-----------'; (awk -F"\?=" '/^## /{ comment = substr($$0,4) } comment && /^[a-zA-Z][a-zA-Z0-9_-]*[ ]+\?= /{ print "   " $$1 "?=" substr($$2,0,80) "?=" comment }' $(MAKEFILE_LIST) 2>/dev/null | sort -d)) | sed -e 's/\?= /?=/g' | column -t -s '?='
	@ echo ''

## Will display value of variable.
.PHONY: debug/%3
debug/%:
	@test -z "$(wordlist 2,3,$(subst /, ,$*))" && echo '$*=$($*)' || $(MAKE) -C "$(*:$(firstword $(subst /, ,$*))/%=%)" "debug/$(firstword $(subst /, ,$*))"

## prepare variables.mk
variables.mk:
	@echo "Generating <variables.mk> file"
	@printf 'SIZE ?=4096\nCANBDAYS ?=3650\nNBDAYS ?=3650\nC ?=FR\nST ?=France\nL ?=Paris\nO ?=Orga\nOU ?=Unit\nCACN ?=EasyCA\n' > variables.mk

## Make key, certificate and configuration for certificates authority (ca.key, ca.crt and openssl.cnf)
.PHONY: ca
ca: ca.crt openssl.cnf

.PHONY: ac
ac : ca

openssl.cnf :
	@echo "Generating configuration file <openssl.cnf> for certificates authority <$(CACN)>"
	@printf '[ ca ]\ndefault_ca = CA_default\n\n[ CA_default ]\ndatabase        = index.txt\ncrlnumber       = crlnumber\ndefault_crl_days = 30\ndefault_md      = sha256\ncrl_extensions  = crl_ext\n\n[ crl_ext ]\nauthorityKeyIdentifier = keyid:always\n' > openssl.cnf

ca.key :
	@echo "Generating private key <ca.key> for certificates authority <$(CACN)>"
	@$(OPENSSL) genrsa -out ca.key $(SIZE)

ifeq ($(WINMODE),0)
ca.crt : variables.mk ca.key
	@echo "Generating certificate <ca.crt> for certificates authority <$(CACN)>"
	@$(OPENSSL) req -x509 -sha512 -days $(CANBDAYS) \
	-new -noenc \
	-key ca.key \
	-outform PEM -out ca.crt \
	-addext "basicConstraints = critical, CA:TRUE" \
	-addext "subjectKeyIdentifier = hash" \
	-addext "authorityKeyIdentifier = keyid:always, issuer" \
	-addext "keyUsage = critical, keyCertSign, cRLSign, digitalSignature" \
	-subj "/C=$(C)/ST=$(ST)/L=$(L)/O=$(O)/OU=$(OU)/CN=$(CACN)"
else
ca.crt : variables.mk ca.key
	@echo "Generating certificate <ca.crt> for certificates authority <$(CACN)>"
	@$(OPENSSL) req -x509 -sha512 -days $(CANBDAYS) \
	-new -noenc \
	-key ca.key \
	-outform PEM -out ca.crt \
	-addext "basicConstraints = critical, CA:TRUE" \
	-addext "subjectKeyIdentifier = hash" \
	-addext "authorityKeyIdentifier = keyid:always, issuer" \
	-addext "keyUsage = critical, keyCertSign, cRLSign, digitalSignature" \
	-subj "//C=$(C)\ST=$(ST)\L=$(L)\O=$(O)\OU=$(OU)\CN=$(CACN)"
endif

## Read certificate (readcrt <filename>)
.PHONY: readcrt
readcrt:
	@$(MAKE) --no-print-directory readcrt/$(filter-out $@,$(MAKECMDGOALS))

.PHONY: readcrt/%
readcrt/% :
	@test -z "$(wordlist 2,3,$(subst /, ,$*))" && { echo "Reading certificate <$*>" ; $(OPENSSL) x509 -text -noout -in $* ; }

## Generate a private key and a certificate signing request for a domain (req <domain>)
.PHONY: req
req:
	@DOMAIN=$(filter-out $@,$(MAKECMDGOALS)) $(MAKE) --no-print-directory csr

.PHONY: domainkey
domainkey :
	@echo "Generating private key <$(DOMAIN).key> for domain <$(DOMAIN)>"
	@$(OPENSSL) genrsa -out $(DOMAIN).key $(SIZE)

.PHONY: csr
ifeq ($(WINMODE),0)
csr :
	@test -f $(DOMAIN).key || $(MAKE) --no-print-directory domainkey
	@echo "Generating certificate signing request <$(DOMAIN).csr> for domain <$(DOMAIN)>"
	@for ip in $(IP);do IPS="$${IPS},IP:$${ip}";done;for name in $(ALTNAME);do NAMES="$${NAMES},DNS:$${name}";done;$(OPENSSL) req -new \
	-key $(DOMAIN).key \
	-outform PEM -out $(DOMAIN).csr \
	-subj "/C=$(C)/ST=$(ST)/L=$(L)/O=$(O)/OU=$(OU)/CN=$(DOMAIN)" \
	-addext "subjectAltName = IP:127.0.0.1$${IPS},DNS:$(DOMAIN)$${NAMES}"
else
csr :
	@test -f $(DOMAIN).key || $(MAKE) --no-print-directory domainkey
	@echo "Generating certificate signing request <$(DOMAIN).csr> for domain <$(DOMAIN)>"
	@for ip in $(IP);do IPS="$${IPS},IP:$${ip}";done;for name in $(ALTNAME);do NAMES="$${NAMES},DNS:$${name}";done;$(OPENSSL) req -new \
	-key $(DOMAIN).key \
	-outform PEM -out $(DOMAIN).csr \
	-subj "//C=$(C)\ST=$(ST)\L=$(L)\O=$(O)\OU=$(OU)\CN=$(DOMAIN)" \
	-addext "subjectAltName = IP:127.0.0.1$${IPS},DNS:$(DOMAIN)$${NAMES}"
endif

## Read certificate signing request (readcsr <filename>)
.PHONY: readcsr
readcsr:
	@$(MAKE) --no-print-directory readcsr/$(filter-out $@,$(MAKECMDGOALS))

.PHONY: readcsr/%
readcsr/% :
	@test -z "$(wordlist 2,3,$(subst /, ,$*))" && { echo "Reading certificate signing request <$*>" ; $(OPENSSL) req -text -noout -verify -in $* ; }

## Sign a certificate signing request with local pki (sign <domain>)
.PHONY: sign
sign:
	@DOMAIN=$(filter-out $@,$(MAKECMDGOALS)) $(MAKE) --no-print-directory dosign

.PHONY: dosign
ifeq ($(WINMODE),0)
dosign : ca
	@test -f $(DOMAIN).csr || $(MAKE) --no-print-directory csr
	@echo "Signing certificate signing request for domain <$(DOMAIN)>"
	@$(OPENSSL) x509 -req -days $(NBDAYS) -sha512 \
	-CA ca.crt -CAkey ca.key -CAcreateserial \
	-in $(DOMAIN).csr \
	-copy_extensions copy \
	-outform PEM -out $(DOMAIN).crt
else
dosign: ca
	@test -f $(DOMAIN).csr || $(MAKE) --no-print-directory csr
	@echo "Signing certificate signing request for domain <$(DOMAIN)>"
	@$(OPENSSL) x509 -req -days $(NBDAYS) -sha512 \
	-CA ca.crt -CAkey ca.key -CAcreateserial \
	-in $(DOMAIN).csr \
	-outform PEM -out $(DOMAIN).crt
endif

## Sign a certificate signing request with remote pki (pki <domain>), need 'sign.sh' external script
.PHONY: pki
pki:
	@DOMAIN=$(filter-out $@,$(MAKECMDGOALS)) $(MAKE) --no-print-directory dopki

.PHONY: dopki
dopki :
	@test -x sign.sh || { echo "can not find external script 'sign.sh'" ; exit 1 ; }
	@test -f $(DOMAIN).csr || $(MAKE) --no-print-directory csr
	@sign.sh $(DOMAIN).csr > $(DOMAIN).crt

## Make Certificate chain
.PHONY: chain
chain: ca.crt
	@test -f $(DOMAIN).crt || $(MAKE) --no-print-directory sign
	@test -f $(DOMAIN)-chain.crt || { \
	echo "Generating <$(DOMAIN)-chain.crt> file" ; \
	cat ca.crt $(DOMAIN).crt > $(DOMAIN)-chain.crt ; \
	}

## Clean all or clean domain (clean <domain>)
.PHONY: clean
clean:
	@echo "Cleaning..."
	-@$(MAKE) --no-print-directory clean/$(filter-out $@,$(MAKECMDGOALS))
	-@rm -f ca.key ca.crt domain.key domain.crt

.PHONY: clean/%
clean/% :
	-@test -z "$(wordlist 2,3,$(subst /, ,$*))" && { rm -f $*.key $*.csr $*.crt ; }

## Revoke a certificate (revoke <domain>)
.PHONY: revoke
revoke:
	@DOMAIN=$(filter-out $@,$(MAKECMDGOALS)) $(MAKE) --no-print-directory dorevoke

.PHONY: dorevoke
dorevoke :
	@test -f $(DOMAIN).crt || { echo "can not find certificate file '$(DOMAIN).crt'" ; exit 1 ; }
	@touch index.txt && echo "1000" > crlnumber
	@$(OPENSSL) ca -config openssl.cnf -keyfile ca.key -cert ca.crt -revoke $(DOMAIN).crt
	@$(OPENSSL) ca -config openssl.cnf -keyfile ca.key -cert ca.crt -gencrl -out ca.crl
	@rm -f crlnumber crlnumber.old index.txt index.txt.old index.txt.attr index.txt.attr.old

## Read CRL file
.PHONY: readcrl
readcrl:
	@$(OPENSSL) crl -in ca.crl -text -noout

# Default target
%:
	@:

