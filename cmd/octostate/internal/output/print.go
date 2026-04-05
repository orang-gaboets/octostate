package output

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/spf13/cobra"
)

// PrintJSON writes a pretty-printed JSON value to the command stdout.
func PrintJSON(cmd *cobra.Command, v any) error {
	rv := reflect.ValueOf(v)
	if rv.IsValid() && rv.Kind() == reflect.Slice && rv.IsNil() {
		v = reflect.MakeSlice(rv.Type(), 0, 0).Interface()
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("write JSON output: %w", err)
	}
	return nil
}
