package pretty

import (
	"encoding/json"
	"fmt"
)

func Print(data interface{}) {
	buf, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		panic(err)
	}

	fmt.Println(string(buf))
}
