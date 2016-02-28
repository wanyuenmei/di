package dsl

import "github.com/google/go-github/github"

type key interface {
	keys() ([]string, error)
}

type githubKey struct {
	username string

	atomImpl
}

type plaintextKey struct {
	key string

	atomImpl
}

var githubCache = make(map[string][]string)

func (githubKey githubKey) keys() ([]string, error) {
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

func (plaintextKey plaintextKey) keys() ([]string, error) {
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
