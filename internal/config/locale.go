package config

import (
	"fmt"
	"sort"
)

// Locale holds the interface texts of one language, keyed by dotted path ("home.tagline").
type Locale struct {
	Lang     string
	messages map[string]string
}

// T returns the text for key, or an error if the key is missing so pages fail loudly instead of showing a hole.
func (l *Locale) T(key string) (string, error) {
	msg, ok := l.messages[key]
	if !ok {
		return "", fmt.Errorf("locale %s: missing key %q", l.Lang, key)
	}
	return msg, nil
}

// Keys returns all keys, sorted.
func (l *Locale) Keys() []string {
	keys := make([]string, 0, len(l.messages))
	for k := range l.messages {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func newLocale(lang string, tree map[string]any) (*Locale, error) {
	l := &Locale{Lang: lang, messages: map[string]string{}}
	if err := flatten(l.messages, "", tree); err != nil {
		return nil, err
	}
	return l, nil
}

func flatten(dst map[string]string, prefix string, tree map[string]any) error {
	for k, v := range tree {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch v := v.(type) {
		case string:
			dst[key] = v
		case map[string]any:
			if err := flatten(dst, key, v); err != nil {
				return err
			}
		default:
			return fmt.Errorf("key %q: want a text or a nested group, got %T", key, v)
		}
	}
	return nil
}
