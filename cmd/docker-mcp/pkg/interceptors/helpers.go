package interceptors

import (
	"encoding/json"
	"fmt"
)

func argumentsToString(args any) string {
	buf, err := json.Marshal(args)
	if err != nil {
		return fmt.Sprintf("%v", args)
	}

	return string(buf)
}
