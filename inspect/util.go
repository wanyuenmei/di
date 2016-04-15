package main

type Label string

type Path []Label

func contains(ls Path, it Label) bool {
	for _, a := range ls {
		if a == it {
			return true
		}
	}

	return false
}
