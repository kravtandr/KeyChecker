package provider

// Detect возвращает первый провайдер, распознавший ключ по префиксу.
func Detect(providers []Provider, key string) Provider {
	for _, p := range providers {
		if p.Matches(key) {
			return p
		}
	}
	return nil
}
