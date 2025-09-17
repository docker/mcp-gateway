package secretsscan

func ContainsSecrets(text string) bool {
	for _, rule := range rules() {
		if rule.matches(text) {
			return true
		}
	}

	return false
}
