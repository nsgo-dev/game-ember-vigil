package strings

import (
	"reflect"
	"testing"
)

func TestTablesAreComplete(t *testing.T) {
	cardIDs := []string{
		"ember-bolt", "lantern-halo", "cinder-trail", "bell-toll",
		"wick", "oil", "reach", "step", "warmth", "mending", "gathering", "glow",
		"ember-heal",
	}

	for _, lang := range Languages {
		tbl := For(lang)
		v := reflect.ValueOf(tbl)
		typ := v.Type()
		for i := range v.NumField() {
			f := v.Field(i)
			if f.Kind() == reflect.String && f.String() == "" {
				t.Errorf("lang %q: field %s is empty", lang, typ.Field(i).Name)
			}
		}
		for _, id := range cardIDs {
			c, ok := tbl.Cards[id]
			if !ok {
				t.Errorf("lang %q: missing card %q", lang, id)
				continue
			}
			if c.Name == "" || c.Description == "" {
				t.Errorf("lang %q: card %q has empty name or description: %+v", lang, id, c)
			}
		}
	}
}

func TestNormalizeLang(t *testing.T) {
	if got := NormalizeLang("zh-CN"); got != ZhCN {
		t.Fatalf("NormalizeLang(zh-CN) = %q, want %q", got, ZhCN)
	}
	if got := NormalizeLang("zh"); got != ZhCN {
		t.Fatalf("NormalizeLang(zh) = %q, want %q", got, ZhCN)
	}
	if got := NormalizeLang("en"); got != EN {
		t.Fatalf("NormalizeLang(en) = %q, want %q", got, EN)
	}
	if got := NormalizeLang("unknown"); got != EN {
		t.Fatalf("NormalizeLang(unknown) = %q, want %q", got, EN)
	}
}
