package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/GoCodeAlone/workflow-plugin-gcp/provider"
)

func main() {
	p := provider.New()

	info := map[string]any{
		"name":    p.Name(),
		"version": p.Version(),
		"caps":    p.Capabilities(),
	}

	if len(os.Args) > 1 && os.Args[1] == "info" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(info); err != nil {
			log.Fatal(err)
		}
		return
	}

	fmt.Printf("workflow-plugin-gcp %s\n", p.Version())
	fmt.Println("Usage: workflow-plugin-gcp info")
}
