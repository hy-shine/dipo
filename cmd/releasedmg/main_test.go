package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const oneEnclosure = `<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0" xmlns:sparkle="http://www.andymatuschak.org/xml-namespaces/sparkle">
  <channel>
    <item>
      <title>Dipo 0.0.1</title>
      <enclosure url="https://example.test/Dipo-0.0.1-macos_arm64.dmg" length="1" type="application/octet-stream" sparkle:os="macos" sparkle:version="1" sparkle:shortVersionString="0.0.1" sparkle:sha256="00" sparkle:edSignature="stale"/>
    </item>
  </channel>
</rss>
`

func testKey() ed25519.PrivateKey {
	return ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
}

// attribute returns the value of the first attribute name in text.
func attribute(text, name string) string {
	m := regexp.MustCompile(` ` + regexp.QuoteMeta(name) + `="([^"]*)"`).FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	return m[1]
}

func TestRewriteEnclosureDescribesTheStrippedImage(t *testing.T) {
	image := []byte("stripped disk image bytes")
	key := testKey()
	got, err := rewriteEnclosure([]byte(oneEnclosure), image, key)
	if err != nil {
		t.Fatalf("rewriteEnclosure: %v", err)
	}
	text := string(got)
	if want := fmt.Sprint(len(image)); attribute(text, "length") != want {
		t.Errorf("length = %q, want %q", attribute(text, "length"), want)
	}
	sum := sha256.Sum256(image)
	if want := hex.EncodeToString(sum[:]); attribute(text, "sparkle:sha256") != want {
		t.Errorf("sparkle:sha256 = %q, want %q", attribute(text, "sparkle:sha256"), want)
	}
	signature, err := base64.StdEncoding.DecodeString(attribute(text, "sparkle:edSignature"))
	if err != nil {
		t.Fatalf("sparkle:edSignature is not base64: %v", err)
	}
	if !ed25519.Verify(key.Public().(ed25519.PublicKey), image, signature) {
		t.Error("sparkle:edSignature does not verify the stripped image with the update key")
	}
	if strings.Count(text, "<item>") != 1 || !strings.Contains(text, "<title>Dipo 0.0.1</title>") || !strings.Contains(text, `url="https://example.test/Dipo-0.0.1-macos_arm64.dmg"`) {
		t.Errorf("the rest of the feed changed:\n%s", text)
	}
}

func TestImageNameIsTheFileTheEnclosureNames(t *testing.T) {
	withQuery := strings.Replace(oneEnclosure, "Dipo-0.0.1-macos_arm64.dmg", "dipo-0.2.0-macos-x64.dmg?raw=1", 1)
	for _, tc := range []struct{ name, feed, want string }{
		{"plain url", oneEnclosure, "Dipo-0.0.1-macos_arm64.dmg"},
		{"url with a query", withQuery, "dipo-0.2.0-macos-x64.dmg"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := imageName([]byte(tc.feed))
			if err != nil {
				t.Fatalf("imageName: %v", err)
			}
			if got != tc.want {
				t.Errorf("imageName = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestImageNameRefusesAFeedWithoutOneImage(t *testing.T) {
	twoEnclosures := strings.Replace(oneEnclosure, "</item>", `<enclosure url="https://example.test/x.dmg" length="1"/></item>`, 1)
	noURL := strings.Replace(oneEnclosure, ` url="https://example.test/Dipo-0.0.1-macos_arm64.dmg"`, "", 1)
	noFile := strings.Replace(oneEnclosure, "Dipo-0.0.1-macos_arm64.dmg", "", 1)
	for _, tc := range []struct{ name, feed string }{
		{"no enclosure", "<rss/>"},
		{"two enclosures", twoEnclosures},
		{"no url", noURL},
		{"url without a file", noFile},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := imageName([]byte(tc.feed)); err == nil {
				t.Fatal("imageName returned no error")
			}
		})
	}
}

// writeRelease fills dir with a tiny disk image and an appcast naming it for
// each of images, the way nsgo writes one release's files for each target.
func writeRelease(t *testing.T, dir string, images map[string]string) {
	t.Helper()
	for target, image := range images {
		file := filepath.Join(dir, image)
		if out, err := exec.Command("hdiutil", "create", "-size", "2m", "-fs", "HFS+", "-volname", "Dipo", file).CombinedOutput(); err != nil {
			t.Fatalf("hdiutil create %s: %v: %s", image, err, out)
		}
		feed := strings.Replace(oneEnclosure, "Dipo-0.0.1-macos_arm64.dmg", image, 1)
		if err := os.WriteFile(filepath.Join(dir, "appcast-"+target+".xml"), []byte(feed), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRunStripsEachImageAndRewritesItsOwnAppcast(t *testing.T) {
	if _, err := exec.LookPath("hdiutil"); err != nil {
		t.Skip("hdiutil is not available")
	}
	images := map[string]string{"macos_arm64": "dipo-0.2.0-macos-arm64.dmg", "macos_x64": "dipo-0.2.0-macos-x64.dmg"}
	dir := t.TempDir()
	writeRelease(t, dir, images)
	keyText := base64.StdEncoding.EncodeToString(make([]byte, ed25519.SeedSize))
	if err := run(dir, keyText); err != nil {
		t.Fatalf("run: %v", err)
	}
	for target, image := range images {
		data, err := os.ReadFile(filepath.Join(dir, image))
		if err != nil {
			t.Fatal(err)
		}
		feed, err := os.ReadFile(filepath.Join(dir, "appcast-"+target+".xml"))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if got, want := attribute(string(feed), "sparkle:sha256"), hex.EncodeToString(sum[:]); got != want {
			t.Errorf("%s: sparkle:sha256 = %q, want the stripped image's %q", target, got, want)
		}
		checksum, err := os.ReadFile(filepath.Join(dir, image+".sha256"))
		if err != nil {
			t.Fatal(err)
		}
		if want := checksumLine(image, data); string(checksum) != want {
			t.Errorf("%s.sha256 = %q, want %q", image, checksum, want)
		}
	}
}

func TestRunRefusesAnImageNoAppcastNames(t *testing.T) {
	if _, err := exec.LookPath("hdiutil"); err != nil {
		t.Skip("hdiutil is not available")
	}
	dir := t.TempDir()
	writeRelease(t, dir, map[string]string{"macos_arm64": "dipo-0.2.0-macos-arm64.dmg"})
	if out, err := exec.Command("hdiutil", "create", "-size", "2m", "-fs", "HFS+", "-volname", "Dipo", filepath.Join(dir, "dipo-0.2.0-macos-x64.dmg")).CombinedOutput(); err != nil {
		t.Fatalf("hdiutil create: %v: %s", err, out)
	}
	keyText := base64.StdEncoding.EncodeToString(make([]byte, ed25519.SeedSize))
	if err := run(dir, keyText); err == nil {
		t.Fatal("run returned no error for a disk image no appcast names")
	}
}

func TestRewriteEnclosureRefusesAFeedItCannotFix(t *testing.T) {
	twoEnclosures := strings.Replace(oneEnclosure, "</item>", `<enclosure url="x" length="1" sparkle:edSignature="a"/></item>`, 1)
	noSignature := strings.Replace(oneEnclosure, ` sparkle:edSignature="stale"`, "", 1)
	for _, tc := range []struct{ name, feed string }{
		{"no enclosure", "<rss/>"},
		{"two enclosures", twoEnclosures},
		{"no signature attribute", noSignature},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := rewriteEnclosure([]byte(tc.feed), []byte("image"), testKey()); err == nil {
				t.Fatal("rewriteEnclosure returned no error")
			}
		})
	}
}
