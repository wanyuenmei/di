package util

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"testing"
)

func TestToTar(t *testing.T) {
	content := fmt.Sprintf("a b c\neasy as\n1 2 3")
	out, err := ToTar("test_tar", 0644, content)

	if err != nil {
		t.Errorf("Error %#v while writing tar archive, expected nil", err.Error())
	}

	var buffOut bytes.Buffer
	writer := io.Writer(&buffOut)

	for tr := tar.NewReader(out); err != io.EOF; _, err = tr.Next() {
		if err != nil {
			t.Errorf("Error %#v while reading tar archive, expected nil", err.Error())
		}

		_, err = io.Copy(writer, tr)
		if err != nil {
			t.Errorf("Error %#v while reading tar archive, expected nil", err.Error())
		}
	}

	actual := buffOut.String()
	if actual != content {
		t.Error("Generated incorrect tar archive.")
	}
}

func TestEditDistance(t *testing.T) {
	if err := ed(nil, nil, 0); err != "" {
		t.Error(err)
	}

	if err := ed([]string{"a"}, nil, 1); err != "" {
		t.Error(err)
	}

	if err := ed(nil, []string{"a"}, 1); err != "" {
		t.Error(err)
	}

	if err := ed([]string{"a"}, []string{"a"}, 0); err != "" {
		t.Error(err)
	}

	if err := ed([]string{"b"}, []string{"a"}, 2); err != "" {
		t.Error(err)
	}

	if err := ed([]string{"b", "a"}, []string{"a"}, 1); err != "" {
		t.Error(err)
	}

	if err := ed([]string{"b", "a"}, []string{}, 2); err != "" {
		t.Error(err)
	}

	if err := ed([]string{"a", "b", "c"}, []string{"a", "b", "c"}, 0); err != "" {
		t.Error(err)
	}

	if err := ed([]string{"b", "c"}, []string{"a", "b", "c"}, 1); err != "" {
		t.Error(err)
	}

	if err := ed([]string{"b", "c"}, []string{"a", "c"}, 2); err != "" {
		t.Error(err)
	}
}

func ed(a, b []string, exp int) string {
	if ed := EditDistance(a, b); ed != exp {
		return fmt.Sprintf("Distance(%s, %s) = %v, expected %v", a, b, ed, exp)
	}
	return ""
}
