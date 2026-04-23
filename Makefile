VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
PREFIX  ?= /usr/local
LDFLAGS := -s -w -X main.Version=$(VERSION) -X github.com/NoneNameDeveloper/wireguard-ui/internal/tray.Version=$(VERSION)

.PHONY: build run vet tidy install setcap uninstall clean

build:
	CGO_ENABLED=1 go build -trimpath -ldflags '$(LDFLAGS)' -o build/wgtray ./cmd/wgtray

run: build
	./build/wgtray

vet:
	go vet ./...

tidy:
	go mod tidy

install: build
	install -Dm755 build/wgtray                   $(DESTDIR)$(PREFIX)/bin/wgtray
	install -Dm644 contrib/desktop/wgtray.desktop $(DESTDIR)$(PREFIX)/share/applications/wgtray.desktop
	install -Dm644 contrib/systemd/wgtray.service $(DESTDIR)$(PREFIX)/lib/systemd/user/wgtray.service
	install -Dm644 contrib/polkit/90-wgtray.rules $(DESTDIR)/etc/polkit-1/rules.d/90-wgtray.rules

# Grant the installed binary CAP_NET_ADMIN so it can read peer stats /
# transfer counters without running as root. Must be re-run after every
# reinstall because file capabilities are stripped on overwrite.
setcap:
	sudo setcap cap_net_admin+ep $(DESTDIR)$(PREFIX)/bin/wgtray

uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/wgtray
	rm -f $(DESTDIR)$(PREFIX)/share/applications/wgtray.desktop
	rm -f $(DESTDIR)$(PREFIX)/lib/systemd/user/wgtray.service
	rm -f $(DESTDIR)/etc/polkit-1/rules.d/90-wgtray.rules

clean:
	rm -rf build
