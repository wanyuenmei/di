package main

import (
    "fmt"
    "time"
)

func main() {
    for ;; {
        fmt.Println("Hello World!")
        time.Sleep(5 * time.Second)
    }
}
