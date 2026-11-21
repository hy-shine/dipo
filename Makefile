.PHONY: build release version run check check-native check-ui

GOARCH := $(shell go env GOARCH)
TARGET_ARCH := $(if $(filter amd64,$(GOARCH)),x64,$(GOARCH))
TARGET := macos_$(TARGET_ARCH)
VERSION := $(shell awk '$$1 == "version:" {print $$2}' nsgo.yaml)
APP := build/nsgo/Dipo-$(VERSION)-$(TARGET)/Dipo.app
# The code signing identity: ad hoc by default. Name a stable identity from your
# keychain to keep the Screen Recording grant across rebuilds; release requires one.
CODESIGN_IDENTITY ?= -

NATIVE_SOURCES := $(wildcard native/*.swift)

resources/dipo-sampler: $(NATIVE_SOURCES)
	mkdir -p resources
	xcrun swiftc -O -target $(shell uname -m)-apple-macosx14.0 -framework AppKit -framework ScreenCaptureKit $(NATIVE_SOURCES) -o resources/dipo-sampler
	codesign --force --sign - resources/dipo-sampler

build: resources/dipo-sampler
	codesign --dryrun --force --sign "$(CODESIGN_IDENTITY)" resources/dipo-sampler
	CGO_ENABLED=0 go tool nsgo build -target $(TARGET) -installers=false -sign "$(CODESIGN_IDENTITY)"
	/usr/libexec/PlistBuddy -c 'Set :LSMinimumSystemVersion 14.0' "$(APP)/Contents/Info.plist"
	codesign --force --sign "$(CODESIGN_IDENTITY)" "$(APP)/Contents/Resources/dipo-sampler"
	codesign --force --sign "$(CODESIGN_IDENTITY)" "$(APP)"
	codesign --verify --deep --strict "$(APP)"

# release writes the disk image and the signed appcast that updates read into
# build/release/releases. nsgo does not sign the picker helper it embeds, so it
# is signed here with the stable identity first; without that, every update
# would ask for Screen Recording again. nsgo also writes LSMinimumSystemVersion
# 11.0, which the Info.plist patch in build cannot reach, so the disk image
# does not hold back macOS 11 to 13, where the picker does not work.
release: resources/dipo-sampler
	@test -n "$$NSGO_UPDATES_KEY" || { echo "NSGO_UPDATES_KEY is not set; nsgo would write no appcast" >&2; exit 1; }
	@{ test -n "$(CODESIGN_IDENTITY)" && test "$(CODESIGN_IDENTITY)" != "-"; } || { echo "CODESIGN_IDENTITY must name a stable code signing identity, not ad hoc" >&2; exit 1; }
	codesign --force --sign "$(CODESIGN_IDENTITY)" resources/dipo-sampler
	CGO_ENABLED=0 go tool nsgo build -target $(TARGET) -installers=true -sign "$(CODESIGN_IDENTITY)" -output build/release

# version prints nsgo.yaml's version, for release.yml to match against the tag.
version:
	@echo $(VERSION)

run: build
	open "$(APP)"

check-native: resources/dipo-sampler
	resources/dipo-sampler --self-test build/native-checks

check-ui: resources/dipo-sampler
	go tool nsgo test -target macos -run '^TestNative' -timeout 90s

check: check-native
	go vet ./...
	go test ./...
