package embed

import _ "embed"

//go:embed definitions.yaml
var Definitions []byte
