package stitch

import (
	"bufio"
	"os"
)

func contains(ls []string, it string) bool {
	for _, a := range ls {
		if a == it {
			return true
		}
	}

	return false
}

type parser func(string) error

func forLineInFile(path string, f parser) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if err := f(line); err != nil {
			return err
		}
	}
	return nil
}
