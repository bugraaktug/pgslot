# Builds the pgslot CLI in isolation -- not part of the top-level Makefile,
# same reasoning as src/Makefile: the extension's own `make install` stays
# pure SQL, no Go toolchain required to install pgslot itself.

BINARY := pgslot
PREFIX ?= /usr/local

.PHONY: build install clean vet

build:
	go build -o $(BINARY) .

install: build
	install -d $(DESTDIR)$(PREFIX)/bin
	install -m 755 $(BINARY) $(DESTDIR)$(PREFIX)/bin/$(BINARY)

vet:
	go vet ./...

clean:
	rm -f $(BINARY)
