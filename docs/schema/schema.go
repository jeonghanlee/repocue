// Package schema validates the embedded public contracts without loading remote
// schemas or runtime files from the user's checkout.
package schema

import (
	"bytes"
	"embed"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed *.schema.json
var contracts embed.FS

// Contract returns an embedded contract for an external provider that requires
// a schema file. The embedded filesystem never reads the user's repository.
func Contract(name string) ([]byte, error) {
	return contracts.ReadFile(name)
}

func Validate(name string, serialized []byte) error {
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	compiler.UseLoader(nil)
	entries, err := contracts.ReadDir(".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		data, err := contracts.ReadFile(entry.Name())
		if err != nil {
			return err
		}
		document, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
		if err != nil {
			return err
		}
		if err := compiler.AddResource("https://repocue.local/schema/"+entry.Name(), document); err != nil {
			return err
		}
	}
	compiled, err := compiler.Compile("https://repocue.local/schema/" + name)
	if err != nil {
		return err
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(serialized))
	if err != nil {
		return err
	}
	if err := compiled.Validate(document); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}
