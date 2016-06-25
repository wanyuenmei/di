package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const maxLength = 89

func checkFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	var lnum int
	var messages []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		lnum++
		line := strings.Replace(scanner.Text(), "\t", "        ", -1)
		if len(line) > (maxLength + 1) {
			msg := fmt.Sprintf("  %d (len %d > %d): %s", lnum, len(line),
				maxLength, line)
			messages = append(messages, msg)
		}
	}

	if len(messages) > 0 {
		fmt.Printf("%s contains %d offending lines:\n", path, len(messages))
		for _, msg := range messages {
			fmt.Println(msg)
		}
		fmt.Println("")
	}

	return len(messages) == 0
}

func main() {
	ok := true
	for _, file := range os.Args[1:] {
		ok = checkFile(file) && ok
	}

	if !ok {
		os.Exit(1)
	}
}
