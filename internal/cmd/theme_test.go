package cmd

import (
	"strings"
	"testing"

	"github.com/lucasassuncao/bezel/theme"

	"github.com/lucasassuncao/vivi/internal/app"
)

func TestResolveThemeDefaultsToTheBuiltInPalette(t *testing.T) {
	th, err := resolveTheme("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The zero value means "use the adaptive palette", which is the only one
	// that reads on both light and dark terminals.
	if th != theme.ThemeDefault {
		t.Fatalf("expected the adaptive default, got %+v", th)
	}
}

func TestResolveThemeLoadsANamedTheme(t *testing.T) {
	th, err := resolveTheme("grape")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c := th.Colors; c.Accent == "" || c.Selection == "" || c.Danger == "" {
		t.Fatalf("theme did not fill the palette: %+v", c)
	}
}

func TestResolveThemeIsCaseInsensitive(t *testing.T) {
	lower, err := resolveTheme("mango")
	if err != nil {
		t.Fatal(err)
	}
	upper, err := resolveTheme("MANGO")
	if err != nil {
		t.Fatalf("an upper case name should resolve: %v", err)
	}
	if lower != upper {
		t.Fatal("case should not change which theme is chosen")
	}
}

// A theme name that does not exist has to say so and point at the list, rather
// than silently falling back and leaving the user wondering why nothing changed.
func TestResolveThemeRejectsAnUnknownName(t *testing.T) {
	_, err := resolveTheme("chartreuse")
	if err == nil {
		t.Fatal("expected an error for an unknown theme")
	}
	if !strings.Contains(err.Error(), "--list-themes") {
		t.Fatalf("the error should point at the list: %v", err)
	}
}

func TestResolveThemeReadsTheEnvironment(t *testing.T) {
	t.Setenv(ThemeEnvVar, "kiwi")

	fromEnv, err := resolveTheme("")
	if err != nil {
		t.Fatal(err)
	}
	if fromEnv.Colors.Accent == "" {
		t.Fatal("the environment variable should select a theme")
	}

	// An explicit flag wins over the environment.
	fromFlag, err := resolveTheme("cherry")
	if err != nil {
		t.Fatal(err)
	}
	if fromFlag == fromEnv {
		t.Fatal("--theme should override the environment variable")
	}
}

// A soft-deleted version is recoverable and a destroyed one is not, so no
// theme may paint the two alike.
func TestThemesKeepWarningApartFromDanger(t *testing.T) {
	for name, th := range theme.All() {
		if c := theme.ResolveColors(th, true); c.Warning == c.Danger {
			t.Fatalf("theme %q paints warning and danger alike (%q)", name, c.Warning)
		}
	}
}

// The flag wins, the environment fills in behind it, and a misspelling in
// either is refused: "prdo" parsing as "off" would hand a writable session to
// someone who asked for a safe one.
func TestResolveReadOnlyPrefersTheFlagOverTheEnvironment(t *testing.T) {
	t.Setenv(ReadOnlyEnvVar, "prod")

	fromEnv, err := resolveReadOnly("")
	if err != nil {
		t.Fatal(err)
	}
	if fromEnv != app.ReadOnlyOnProd {
		t.Fatalf("the environment should select prod, got %v", fromEnv)
	}

	fromFlag, err := resolveReadOnly("on")
	if err != nil {
		t.Fatal(err)
	}
	if fromFlag != app.ReadOnlyAlways {
		t.Fatalf("--read-only should override the environment, got %v", fromFlag)
	}

	if _, err := resolveReadOnly("prdo"); err == nil {
		t.Fatal("a misspelt flag must be refused, not read as off")
	}
	t.Setenv(ReadOnlyEnvVar, "yes please")
	if _, err := resolveReadOnly(""); err == nil {
		t.Fatal("a misspelt environment value must be refused, not read as off")
	}
}
