package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/NetSys/di/dsl"
	"github.com/NetSys/di/util"
)

func TestReadme(t *testing.T) {
	f, err := util.Open("README.md")
	if err != nil {
		t.Errorf("Failed to open README: %s", err.Error())
		return
	}
	defer f.Close()

	r := bufio.NewReader(f)

	start := "<!-- BEGIN CODE -->\n"
	end := "<!-- END CODE -->\n"
	// XXX: Get rid of dummy label when publicWeb is implemented.
	code := `(label "publicWeb" (docker "google/pause"))` + "\n"
	recording := false

	for {
		line, err := r.ReadString('\n')

		if err == io.EOF {
			if recording {
				fmt.Printf("Unbalanced code blocks.")
				return
			}
			break
		}

		if err != nil {
			t.Errorf("Failed to read README: %s", err.Error())
			return
		}

		if line == start {
			if recording {
				t.Errorf("Unbalanced code blocks.")
				return
			}
			recording = true
			continue
		}

		if line == end {
			if !recording {
				t.Errorf("Unbalanced code blocks.")
				return
			}
			recording = false
		}

		if recording {
			code += fmt.Sprintf("%s", line)
		}
	}

	if err = checkConfig(code); err != nil {
		t.Errorf(err.Error())
	}
}

func checkConfig(content string) error {
	reader := strings.NewReader(content)

	_, err := dsl.New(reader)
	if err != nil {
		return err
	}
	return nil
}
