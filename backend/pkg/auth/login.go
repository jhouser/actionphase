package auth

// fingerprintPtr converts an empty fingerprint to nil, so an absent value is
// stored as NULL rather than an empty string.
func fingerprintPtr(fp string) *string {
	if fp == "" {
		return nil
	}
	return &fp
}
