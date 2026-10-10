package cases

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLoadValidCases(t *testing.T) {
	got, invalid, err := LoadAll("testdata/valid", "")
	if err != nil || len(invalid) != 0 {
		t.Fatalf("LoadAll: err=%v invalid=%v", err, invalid)
	}
	var ids []string
	for _, c := range got {
		ids = append(ids, c.Role+"/"+c.ID)
	}
	if want := []string{"impl/i1", "review/r1", "visual/v1"}; !slices.Equal(ids, want) {
		t.Fatalf("loaded %v, want %v", ids, want)
	}
	impl := got[0]
	if !impl.NeedsDocker || impl.Source.Change != "kan-1-example" || len(impl.Verify) != 1 || len(impl.Files) != 1 {
		t.Errorf("impl case decoded wrong: %+v", impl)
	}
	if impl.Dir != filepath.Join("testdata/valid", "impl", "i1") {
		t.Errorf("impl.Dir = %q", impl.Dir)
	}
	only, invalid, err := LoadAll("testdata/valid", "visual")
	if err != nil || len(invalid) != 0 || len(only) != 1 || only[0].ID != "v1" {
		t.Errorf("LoadAll(visual) = %v, %v, %v", only, invalid, err)
	}
}

func TestRejectMissingAsset(t *testing.T) {
	_, invalid, err := LoadAll("testdata/missing-asset", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(invalid) != 1 || !strings.Contains(invalid[0].Error(), "r2") || !strings.Contains(invalid[0].Error(), "diff.patch") {
		t.Fatalf("invalid = %v, want one error naming r2 and diff.patch", invalid)
	}
}

func TestRejectUnknownRole(t *testing.T) {
	_, invalid, err := LoadAll("testdata/unknown-role", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(invalid) != 1 || !strings.Contains(invalid[0].Error(), `"foo"`) {
		t.Fatalf("invalid = %v, want one error naming role \"foo\"", invalid)
	}
	if _, _, err := LoadAll("testdata/valid", "foo"); err == nil {
		t.Error(`LoadAll(role "foo") succeeded, want an error`)
	}
}

func TestRejectInvalidFields(t *testing.T) {
	cases := map[string]struct {
		file, body, want string
	}{
		"id mismatch":                {"case.json", `{"id":"other","role":"visual","source":{"repo":"g"}}`, "does not match"},
		"role mismatch":              {"case.json", `{"id":"v1","role":"review","source":{"repo":"g"}}`, "directory"},
		"no repo":                    {"case.json", `{"id":"v1","role":"visual","source":{}}`, "source.repo"},
		"unknown field":              {"case.json", `{"id":"v1","role":"visual","source":{"repo":"g"},"needs_docker":true}`, "needs_docker"},
		"key in another case":        {"case.json", `{"ID":"v1","role":"visual","source":{"repo":"g"}}`, `"ID"`},
		"bad verdict":                {"labels.json", `{"verdict":"maybe","defects":[]}`, "verdict"},
		"mismatch empty":             {"labels.json", `{"verdict":"mismatch","defects":[]}`, "defect"},
		"match defects":              {"labels.json", `{"verdict":"match","defects":[{"summary":"x"}]}`, "defect"},
		"labels key in another case": {"labels.json", `{"Verdict":"match","defects":[]}`, "verdict"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			root := copyTree(t, "testdata/valid")
			if err := os.WriteFile(filepath.Join(root, "visual", "v1", tc.file), []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			_, invalid, err := LoadAll(root, "visual")
			if err != nil || len(invalid) != 1 || !strings.Contains(invalid[0].Error(), tc.want) {
				t.Fatalf("err=%v invalid=%v, want one error containing %q", err, invalid, tc.want)
			}
		})
	}
	t.Run("impl without snapshot", func(t *testing.T) {
		root := copyTree(t, "testdata/valid")
		body := `{"id":"i1","role":"impl","source":{"repo":"g","change":"c"},"verify":["true"],"files":["a"]}`
		if err := os.WriteFile(filepath.Join(root, "impl", "i1", "case.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		_, invalid, _ := LoadAll(root, "impl")
		if len(invalid) != 1 || !strings.Contains(invalid[0].Error(), "snapshot") {
			t.Fatalf("invalid = %v, want one error naming snapshot", invalid)
		}
	})
	t.Run("review label without slot", func(t *testing.T) {
		root := copyTree(t, "testdata/valid")
		body := `{"mustFind":[{"id":"F1","file":"x","line":3,"summary":"bug"}],"knownFalse":[]}`
		if err := os.WriteFile(filepath.Join(root, "review", "r1", "labels.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		_, invalid, _ := LoadAll(root, "review")
		if len(invalid) != 1 || !strings.Contains(invalid[0].Error(), "labels F1 carry no slot") {
			t.Fatalf("invalid = %v, want one error naming F1's missing slot", invalid)
		}
	})
	t.Run("review known-false and ignored labels without slot", func(t *testing.T) {
		root := copyTree(t, "testdata/valid")
		body := `{"mustFind":[{"id":"F1","file":"x","line":3,"summary":"bug","slot":"primary"}],` +
			`"knownFalse":[{"id":"F2","file":"x","line":4,"summary":"no bug"}],` +
			`"ignored":[{"id":"F3","file":"y","line":1,"summary":"elsewhere"}]}`
		if err := os.WriteFile(filepath.Join(root, "review", "r1", "labels.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		_, invalid, _ := LoadAll(root, "review")
		if len(invalid) != 1 || !strings.Contains(invalid[0].Error(), "labels F2, F3 carry no slot") {
			t.Fatalf("invalid = %v, want one error naming F2's and F3's missing slot", invalid)
		}
	})
	t.Run("impl without verify", func(t *testing.T) {
		root := copyTree(t, "testdata/valid")
		body := `{"id":"i1","role":"impl","source":{"repo":"g","change":"c"},"snapshot":"s","files":["a"]}`
		if err := os.WriteFile(filepath.Join(root, "impl", "i1", "case.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		_, invalid, _ := LoadAll(root, "impl")
		if len(invalid) != 1 || !strings.Contains(invalid[0].Error(), "verify") {
			t.Fatalf("invalid = %v, want one error naming verify", invalid)
		}
	})
}

func TestCaseSetHashStable(t *testing.T) {
	cs, _, err := LoadAll("testdata/valid", "")
	if err != nil {
		t.Fatal(err)
	}
	h := mustHash(t, cs)
	if got := mustHash(t, slices.Clone(cs)); got != h {
		t.Errorf("hash not deterministic: %s vs %s", got, h)
	}
	reversed := slices.Clone(cs)
	slices.Reverse(reversed)
	if got := mustHash(t, reversed); got != h {
		t.Errorf("hash depends on input order: %s vs %s", got, h)
	}
	root := copyTree(t, "testdata/valid")
	if err := os.WriteFile(filepath.Join(root, "review", "r1", "diff.patch"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, _, err := LoadAll(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if mustHash(t, changed) == h {
		t.Error("hash unchanged after an asset changed")
	}
}

func TestCaseSetHashIgnoresHiddenFiles(t *testing.T) {
	root := copyTree(t, "testdata/valid")
	cs, _, err := LoadAll(root, "")
	if err != nil {
		t.Fatal(err)
	}
	h := mustHash(t, cs)
	v1 := filepath.Join(root, "visual", "v1")
	if err := os.WriteFile(filepath.Join(v1, ".DS_Store"), []byte("finder"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(v1, ".cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v1, ".cache", "x"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := mustHash(t, cs); got != h {
		t.Errorf("hash changed after hidden files appeared: %s vs %s", got, h)
	}
}

func TestRejectSymlinkedCase(t *testing.T) {
	root := copyTree(t, "testdata/valid")
	if err := os.Symlink("i1", filepath.Join(root, "impl", "i2")); err != nil {
		t.Fatal(err)
	}
	_, invalid, err := LoadAll(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(invalid) != 1 || !strings.Contains(invalid[0].Error(), "symlink") {
		t.Errorf("invalid = %v, want one symlink refusal", invalid)
	}
}

func mustHash(t *testing.T, cs []Case) string {
	t.Helper()
	h, err := SetHash(cs)
	if err != nil || h == "" {
		t.Fatalf("SetHash = %q, %v", h, err)
	}
	return h
}

func copyTree(t *testing.T, src string) string {
	t.Helper()
	dst := t.TempDir()
	if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
		t.Fatal(err)
	}
	return dst
}
