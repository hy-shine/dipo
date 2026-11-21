package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
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
