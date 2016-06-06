package main

// A Label as defined in the DSL
type Label string

// A Path represents a series of labels that must be traversed to from a to b.
type Path []Label

func contains(ls Path, it Label) bool {
	for _, a := range ls {
		if a == it {
			return true
		}
	}

	return false
}
