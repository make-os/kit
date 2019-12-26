package jsmodules

import (
	"fmt"
	"strings"
)

// type castPanick

func castPanic(field string) {
	if err := recover(); err != nil {
		if strings.HasPrefix(err.(error).Error(), "interface conversion") {
			msg := fmt.Sprintf("field '%s' has invalid value type: %s", field, err)
			msg = strings.ReplaceAll(msg, "interface conversion: interface {} is", "has")
			msg = strings.ReplaceAll(msg, "not", "want")
			panic(fmt.Errorf(msg))
		}
		panic(err)
	}
}
