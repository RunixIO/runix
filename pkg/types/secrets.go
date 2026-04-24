package types

// SecretRef defines a reference to a secret value.
type SecretRef struct {
	Type  string `json:"type" yaml:"type"` // env, file, vault
	Value string `json:"value" yaml:"value"`
}
