package main

func contains(ls []string, it string) bool {
	for _, a := range ls {
		if a == it {
			return true
		}
	}

	return false
}
