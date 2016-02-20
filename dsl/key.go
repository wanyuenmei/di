package dsl

import (
	log "github.com/Sirupsen/logrus"
	"github.com/google/go-github/github"
)

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

func getGithubKeys(username string) ([]string, error) {
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

func ParseKeys(keys []Key) []string {
	var parsed []string
	for _, key := range keys {
		parsedKeys, err := key.keys()
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
				"key":   key,
			}).Warning("Failed to retrieve key.")
			continue
		}
		parsed = append(parsed, parsedKeys...)
	}
	return parsed
}
