package localengine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

// loadFiles loads the given name→content YAML files into s.
func loadFiles(t *testing.T, s *Store, files map[string]string) {
	t.Helper()
	fsys := fstest.MapFS{}
	for name, content := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(content)}
	}
	if err := s.LoadFromDir(fsys); err != nil {
		t.Fatalf("load locales: %v", err)
	}
}

func newHTTPTestStore(t *testing.T) *Store {
	t.Helper()
	s := loadTestStore(t, map[string]string{
		"en.yaml": "locale:\n  name: English\nk: en-value\n",
		"ru.yaml": "locale:\n  name: Русский\nk: ru-value\n",
	})
	return s
}

// negotiatedLocale runs the middleware and reports the locale the handler
// saw via FromContext.
func negotiatedLocale(s *Store, target, acceptLanguage string) string {
	seen := ""
	handler := s.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if l := FromContext(r.Context()); l != nil {
			seen = l.Locale()
		}
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if acceptLanguage != "" {
		req.Header.Set("Accept-Language", acceptLanguage)
	}
	handler.ServeHTTP(httptest.NewRecorder(), req)
	return seen
}

func TestMiddlewareNegotiation(t *testing.T) {
	s := newHTTPTestStore(t)

	cases := []struct {
		name   string
		target string
		header string
		want   string
	}{
		{"query param wins", "/?locale=ru", "en-US,en", "ru"},
		{"unknown query falls through to header", "/?locale=xx", "ru-RU,ru", "ru"},
		{"header exact", "/", "en-US,en;q=0.9", "en"},
		{"header primary subtag", "/", "ru-RU", "ru"},
		{"header q-values", "/", "en;q=0.5,ru;q=0.9", "ru"},
		{"default when nothing matches", "/", "xx-YY", "en"},
		{"default when empty", "/", "", "en"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := negotiatedLocale(s, tc.target, tc.header); got != tc.want {
				t.Errorf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestMiddlewareUsesDefaultLocaleFunc(t *testing.T) {
	s := New(WithDefaultLocaleFunc(func() string { return "ru" }))
	loadFiles(t, s, map[string]string{
		"en.yaml": "k: en\n",
		"ru.yaml": "k: ru\n",
	})
	if got := negotiatedLocale(s, "/", ""); got != "ru" {
		t.Errorf("expected default locale 'ru', got %q", got)
	}
}

func TestFromContextWithoutMiddleware(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if l := FromContext(req.Context()); l != nil {
		t.Errorf("expected nil localizer, got %v", l)
	}
}

func TestHandleLocales(t *testing.T) {
	s := newHTTPTestStore(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/locales", nil)
	s.HandleLocales(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body struct {
		DefaultLocale string `json:"defaultLocale"`
		Locales       []struct {
			Code string `json:"code"`
			Name string `json:"name"`
		} `json:"locales"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.DefaultLocale != "en" {
		t.Errorf("expected default 'en', got %q", body.DefaultLocale)
	}
	if len(body.Locales) != 2 || body.Locales[0].Code != "en" || body.Locales[1].Code != "ru" {
		t.Fatalf("unexpected locales: %+v", body.Locales)
	}
	if body.Locales[1].Name != "Русский" {
		t.Errorf("expected native name, got %q", body.Locales[1].Name)
	}
}

func TestHandleDictionary(t *testing.T) {
	s := newHTTPTestStore(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dictionary?locale=ru", nil)
	s.HandleDictionary(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["k"] != "ru-value" {
		t.Errorf("expected 'ru-value', got %v", body["k"])
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/dictionary?locale=xx", nil)
	s.HandleDictionary(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown locale, got %d", rec.Code)
	}
}

func TestLocalizer(t *testing.T) {
	s := newHTTPTestStore(t)
	l := s.NewLocalizer("ru")

	if got := l.T("k"); got != "ru-value" {
		t.Errorf("expected 'ru-value', got %q", got)
	}
	if got := l.T("missing.key"); got != "missing.key" {
		t.Errorf("expected dotted fallback, got %q", got)
	}
	if got, ok := l.Get("k"); !ok || got != "ru-value" {
		t.Errorf("Get: got %q, %v", got, ok)
	}
}
