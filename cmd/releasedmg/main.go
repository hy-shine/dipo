// Command releasedmg makes a release's disk image unsigned and rewrites the
// appcast entry that updates read, so the entry matches the changed image.
//
// nsgo signs the disk image with the app's identity. The identity is
// self-signed, so macOS warns when the image is opened, although the app
// inside is signed the same way and gives no warning of its own. An unsigned
// image opens without that warning, and the app inside keeps its signature,
// which Sparkle checks on each update. Stripping changes the image's bytes, so
// the length, SHA-256 and EdDSA signature nsgo wrote for it are made again.
//
// The argument is the releases directory nsgo writes. The update secret key is
// read from NSGO_UPDATES_KEY, as nsgo reads it.
package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: releasedmg RELEASES_DIR")
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Getenv("NSGO_UPDATES_KEY")); err != nil {
		fmt.Fprintf(os.Stderr, "releasedmg: %v\n", err)
		os.Exit(1)
	}
}

// run strips the disk image in dir and rewrites the macOS appcast beside it,
// signing with the secret key keyText holds.
func run(dir, keyText string) error {
	key, err := secretKey(keyText)
	if err != nil {
		return err
	}
	image, err := onlyMatch(dir, "*.dmg")
	if err != nil {
		return err
	}
	feedPath, err := onlyMatch(dir, "appcast-macos_*.xml")
	if err != nil {
		return err
	}
	stripped := strings.TrimSuffix(image, ".dmg") + "-stripped.dmg"
	if out, err := exec.Command("hdiutil", "convert", image, "-format", "ULMO", "-ov", "-o", stripped).CombinedOutput(); err != nil {
		return fmt.Errorf("hdiutil convert %s: %w: %s", filepath.Base(image), err, out)
	}
	if err := os.Rename(stripped, image); err != nil {
		return err
	}
	data, err := os.ReadFile(image)
	if err != nil {
		return err
	}
	feed, err := os.ReadFile(feedPath)
	if err != nil {
		return err
	}
	updated, err := rewriteEnclosure(feed, data, key)
	if err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(feedPath), err)
	}
	if err := os.WriteFile(feedPath, updated, 0o644); err != nil {
		return err
	}
	return os.WriteFile(image+".sha256", []byte(checksumLine(filepath.Base(image), data)), 0o644)
}

// secretKey decodes the update secret key nsgo keygen prints, the base64 of its
// 32-byte seed. The error never includes the key.
func secretKey(text string) (ed25519.PrivateKey, error) {
	if text == "" {
		return nil, errors.New("NSGO_UPDATES_KEY is not set")
	}
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(text))
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, errors.New("NSGO_UPDATES_KEY is not a secret key from nsgo keygen")
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

// onlyMatch returns the one file in dir whose name matches pattern.
func onlyMatch(dir, pattern string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return "", err
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("want one %s in %s, found %d", pattern, dir, len(matches))
	}
	return matches[0], nil
}

// checksumLine is the line nsgo writes beside a release file: its SHA-256 and
// its name.
func checksumLine(name string, data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]) + "  " + name + "\n"
}

// rewriteEnclosure returns feed with its one enclosure's length, SHA-256 and
// EdDSA signature set for image, which key signs the way Sparkle checks it. It
// fails when feed has no single enclosure or one lacks one of those attributes,
// since then the entry would not be checked as nsgo meant it.
func rewriteEnclosure(feed, image []byte, key ed25519.PrivateKey) ([]byte, error) {
	tag := regexp.MustCompile(`<enclosure [^>]*>`)
	text := string(feed)
	found := tag.FindAllStringIndex(text, -1)
	if len(found) != 1 {
		return nil, fmt.Errorf("want one enclosure, found %d", len(found))
	}
	sum := sha256.Sum256(image)
	element := text[found[0][0]:found[0][1]]
	for _, attr := range []struct{ name, value string }{
		{"length", strconv.Itoa(len(image))},
		{"sparkle:sha256", hex.EncodeToString(sum[:])},
		{"sparkle:edSignature", base64.StdEncoding.EncodeToString(ed25519.Sign(key, image))},
	} {
		var err error
		if element, err = setAttribute(element, attr.name, attr.value); err != nil {
			return nil, err
		}
	}
	return []byte(text[:found[0][0]] + element + text[found[0][1]:]), nil
}

// setAttribute replaces the value of attribute name in element, which must
// already have it.
func setAttribute(element, name, value string) (string, error) {
	attr := regexp.MustCompile(` ` + regexp.QuoteMeta(name) + `="[^"]*"`)
	if !attr.MatchString(element) {
		return "", fmt.Errorf("enclosure has no %s", name)
	}
	return attr.ReplaceAllLiteralString(element, ` `+name+`="`+value+`"`), nil
}
