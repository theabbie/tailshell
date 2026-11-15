package phone

import "testing"

func TestAnnotationsAreConsistent(t *testing.T) {
	for _, tool := range Tools {
		if tool.ReadOnly && (tool.Destructive || tool.OpenWorld) {
			t.Errorf("%s: a read-only tool cannot be destructive or open-world", tool.Name)
		}
		if tool.Description == "" || tool.Title == "" {
			t.Errorf("%s: description and title are required", tool.Name)
		}
		if !tool.ReadOnly && len(tool.Description) < 80 {
			t.Errorf("%s: a state-changing tool must describe its side effects", tool.Name)
		}
	}
}

func TestShellToolIsPresent(t *testing.T) {
	if _, ok := Find("phone_shell"); !ok {
		t.Fatal("phone_shell must stay available")
	}
}
