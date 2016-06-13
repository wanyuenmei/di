package stitch

import "github.com/google/go-github/github"

type key interface {
	keys() ([]string, error)

	ast
}

var githubCache = make(map[string][]string)

func (githubKey astGithubKey) keys() ([]string, error) {
	username := string(githubKey)
	if keys, ok := githubCache[username]; ok {
		return keys, nil
	}
	keys, err := getGithubKeys(username)
	if err != nil {
		return nil, err
	}
	githubCache[username] = keys
	return keys, nil
}

func (plaintextKey astPlaintextKey) keys() ([]string, error) {
	return []string{string(plaintextKey)}, nil
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
