package diff

import (
	"encoding/json"
	"fmt"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// ComputeDiff generates a human-readable diff between two objects
func ComputeDiff(oldObj, newObj interface{}) (string, error) {
	oldJSON, err := json.MarshalIndent(oldObj, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal old object: %w", err)
	}

	newJSON, err := json.MarshalIndent(newObj, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal new object: %w", err)
	}

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(string(oldJSON), string(newJSON), false)

	// Create a more readable diff format
	return dmp.DiffPrettyText(diffs), nil
}

// ExtractImage extracts container image from a deployment spec
func ExtractImage(obj map[string]interface{}) string {
	// Navigate through the spec to find container image
	spec, ok := obj["spec"].(map[string]interface{})
	if !ok {
		return ""
	}

	template, ok := spec["template"].(map[string]interface{})
	if !ok {
		return ""
	}

	templateSpec, ok := template["spec"].(map[string]interface{})
	if !ok {
		return ""
	}

	containers, ok := templateSpec["containers"].([]interface{})
	if !ok || len(containers) == 0 {
		return ""
	}

	firstContainer, ok := containers[0].(map[string]interface{})
	if !ok {
		return ""
	}

	image, _ := firstContainer["image"].(string)
	return image
}
