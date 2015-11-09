package util

import (
    "net/http"
    "io/ioutil"
)

func httpRequest(url string) (string, error){
    resp, err := http.Get(url)
    if err != nil {
        return "<error>", err
    }

    defer resp.Body.Close()

    body_byte, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return "<error>", err
    }

    body := string(body_byte)
    return body[:len(body) - 1], nil
}

func MyIp() (string, error) {
    return httpRequest("http://checkip.amazonaws.com/")
}
