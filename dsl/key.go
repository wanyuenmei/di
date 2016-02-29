package dsl

import "github.com/google/go-github/github"

type Key interface {
	keys() ([]string, error)
}

type GithubKey struct {
	username string

	AtomImpl
}

type PlaintextKey struct {
	key string

	AtomImpl
}

var githubCache = make(map[string][]string)

func (githubKey GithubKey) keys() ([]string, error) {
	if keys, ok := githubCache[githubKey.username]; ok {
		return keys, nil
	}
	keys, err := getGithubKeys(githubKey.username)
	if err != nil {
		return nil, err
	}
	githubCache[githubKey.username] = keys
	return keys, nil
}

func (plaintextKey PlaintextKey) keys() ([]string, error) {
	return []string{plaintextKey.key}, nil
}

// Stored in a variable so we can mock it out for the unit tests.
var getGithubKeys = func(username string) ([]string, error) {
	usersService := github.NewClient(nil).Users
	opt := &github.ListOptions{}
	keys, _, err := usersService.ListKeys(username, opt)

	if err != nil {
		return nil, err
	}

	var keyStrings []string
	for _, key := range keys {
		keyStrings = append(keyStrings, *key.Key)
	}

	return keyStrings, nil
}
