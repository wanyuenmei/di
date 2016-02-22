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
