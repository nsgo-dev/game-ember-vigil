package strings

import (
	"reflect"
	"testing"
)

func TestTablesAreComplete(t *testing.T) {
	cardIDs := []string{
		"ember-bolt", "lantern-halo", "cinder-trail", "bell-toll", "spark-chain", "ash-censer",
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
		for _, id := range []string{"starfall-volley", "sun-ring", "wildfire-path", "dawn-bell", "ember-storm", "pyre-censer"} {
			if c := tbl.Evolutions[id]; c.Name == "" || c.Description == "" || c.Hint == "" {
				t.Errorf("lang %q: evolution %q is incomplete: %+v", lang, id, c)
			}
		}
		for _, id := range []string{"banked-fire", "lamp-oil", "hearth", "light-foot", "lure", "redraw"} {
			if c := tbl.ShopItems[id]; c.Name == "" || c.Description == "" {
				t.Errorf("lang %q: shop item %q is incomplete: %+v", lang, id, c)
			}
		}
		for _, id := range []string{"evolution", "spark-chain", "ash-censer"} {
			if tbl.Unlocks[id] == "" {
				t.Errorf("lang %q: the %q unlock has no name", lang, id)
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
