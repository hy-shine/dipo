.PHONY: build run check check-native check-ui

GOARCH := $(shell go env GOARCH)
TARGET_ARCH := $(if $(filter amd64,$(GOARCH)),x64,$(GOARCH))
TARGET := macos_$(TARGET_ARCH)
APP := build/nsgo/Dipo-0.0.1-$(TARGET)/Dipo.app
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

run: build
	open "$(APP)"

check-native: resources/dipo-sampler
	resources/dipo-sampler --self-test build/native-checks

check-ui: resources/dipo-sampler
	go tool nsgo test -target macos -run '^TestNative' -timeout 90s

check: check-native
	go vet ./...
	go test ./...
