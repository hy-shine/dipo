package locales_test

import (
	"testing"

	"github.com/hy-shine/dipo/locales"

	baselocales "github.com/nsgo-dev/nsgo/base/locales"
	textlocales "github.com/nsgo-dev/nsgo/base/textview/locales"
	desktoplocales "github.com/nsgo-dev/nsgo/desktop/locales"
	updaterlocales "github.com/nsgo-dev/nsgo/updater/locales"
)

func TestSystemLanguageMatching(test *testing.T) {
	for _, example := range []struct {
		locale string
		want   string
	}{
		{"en", "Language"},
		{"en-US", "Language"},
		{"en-GB", "Language"},
		{"zh", "语言"},
		{"zh-CN", "语言"},
		{"zh-SG", "语言"},
		{"zh-Hans", "语言"},
		{"zh-TW", "語言"},
		{"zh-HK", "語言"},
		{"zh-MO", "語言"},
		{"zh-Hant", "語言"},
		{"zh-Hant-CN", "語言"},
		{"zh-Hans-TW", "语言"},
		{"ja-JP", "Language"},
		{"fr-FR", "Language"},
		{"", "Language"},
	} {
		test.Run(example.locale, func(test *testing.T) {
			if got := locales.In(example.locale).Language(); got != example.want {
				test.Fatalf("language for %q = %q; want %q", example.locale, got, example.want)
			}
		})
	}
}

func TestTraditionalLanguageIncludesNativeControls(test *testing.T) {
	for _, example := range []struct {
		name string
		got  string
		want string
	}{
		{"copy menu", desktoplocales.In("zh-Hant").Copy(), "拷貝"},
		{"opacity control", baselocales.In("zh-Hant").ColorAlpha(), "不透明度"},
		{"text alert", textlocales.In("zh-Hant").AlertNote(), "備註"},
		{"update dialog", updaterlocales.In("zh-Hant").Later(), "稍後"},
	} {
		if example.got != example.want {
			test.Errorf("%s = %q; want %q", example.name, example.got, example.want)
		}
	}
}
