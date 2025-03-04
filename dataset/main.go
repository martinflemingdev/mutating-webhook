package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// DatasetMutator handles mutation
type DatasetMutator struct {
	decoder *admission.Decoder
}

// AccessEntry defines the structure for dataset access control
type AccessEntry struct {
	Role         string `json:"role"`
	GroupByEmail string `json:"groupByEmail,omitempty"`
	IAMMember    string `json:"iamMember,omitempty"`
	SpecialGroup string `json:"specialGroup,omitempty"`
	UserByEmail  string `json:"userByEmail,omitempty"`
}

// Handle intercepts and mutates incoming Dataset resources
func (m *DatasetMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	// Decode incoming dataset
	var dataset map[string]interface{}
	if err := json.Unmarshal(req.Object.Raw, &dataset); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Navigate to spec.forProvider.access
	spec, ok := dataset["spec"].(map[string]interface{})
	if !ok {
		return admission.Allowed("No mutation required")
	}
	forProvider, ok := spec["forProvider"].(map[string]interface{})
	if !ok {
		return admission.Allowed("No mutation required")
	}
	access, ok := forProvider["access"].([]interface{})
	if !ok {
		access = []interface{}{} // Initialize if missing
	}

	// Define required access entries
	requiredAccess := []AccessEntry{
		{Role: "OWNER", UserByEmail: "crossplane@axial-life-395119.iam.gserviceaccount.com"},
		{Role: "OWNER", SpecialGroup: "projectOwners"},
		{Role: "READER", SpecialGroup: "projectReaders"},
		{Role: "WRITER", SpecialGroup: "projectWriters"},
	}

	// Merge required entries
	access = mergeAccess(access, requiredAccess)
	forProvider["access"] = access

	// Marshal modified object
	mutated, err := json.Marshal(dataset)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// Return a patch-based response
	return admission.PatchResponseFromRaw(req.Object.Raw, mutated)
}

// mergeAccess ensures required entries exist without duplicates while keeping the correct field name
func mergeAccess(existing []interface{}, required []AccessEntry) []interface{} {
	existingSet := make(map[string]string) // Map: key -> field name

	// Convert existing entries to a lookup map
	for _, entry := range existing {
		entryMap := entry.(map[string]interface{})
		key, field := generateAccessKey(entryMap)
		existingSet[key] = field // Store the field name too
	}

	// Add required entries if they are not already present
	for _, r := range required {
		// Convert struct to map for comparison
		entryMap := map[string]interface{}{
			"role": r.Role,
		}

		// Find which identifier field is set
		var identifierField, identifierValue string
		if r.GroupByEmail != "" {
			identifierField, identifierValue = "groupByEmail", r.GroupByEmail
		} else if r.IAMMember != "" {
			identifierField, identifierValue = "iamMember", r.IAMMember
		} else if r.SpecialGroup != "" {
			identifierField, identifierValue = "specialGroup", r.SpecialGroup
		} else if r.UserByEmail != "" {
			identifierField, identifierValue = "userByEmail", r.UserByEmail
		}

		if identifierField != "" {
			entryMap[identifierField] = identifierValue
		}

		key, _ := generateAccessKey(entryMap) // Generate key again
		if _, exists := existingSet[key]; !exists {
			existing = append(existing, entryMap)
		}
	}

	return existing
}

// generateAccessKey creates a unique key based on role + one of the identifier fields
func generateAccessKey(entry map[string]interface{}) (string, string) {
	role, _ := entry["role"].(string)

	// Check for the first non-empty identifier field
	identifiers := []string{"groupByEmail", "iamMember", "specialGroup", "userByEmail"}
	for _, field := range identifiers {
		if value, exists := entry[field]; exists && value != "" {
			return role + ":" + value.(string), field // Return field name too
		}
	}

	// Fallback if none of the fields are set
	return role + ":unknown", ""
}

func main() {
	opts := ctrl.Options{
		// Other configurations...
		CertDir: "/path/to/certs", // This is typically specified in the controller options
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), opts)
	if err != nil {
		fmt.Println("Failed to create manager:", err)
		os.Exit(1)
	}

	hookServer := mgr.GetWebhookServer()
	hookServer.Register("/mutate-pod", &webhook.Admission{Handler: &PodAnnotator{}})

	fmt.Println("Starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		fmt.Println("Failed to start manager:", err)
		os.Exit(1)
	}
}
