package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// PodMutator handles mutations for Pods
type PodMutator struct {
	decoder admission.Decoder
}

// Handle mutates incoming Pod creation requests
func (m *PodMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &metav1.PartialObjectMetadata{}
	err := m.decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	patch := `[{"op": "add", "path": "/metadata/labels/mutated", "value": "true"}]`
	patchType := admissionv1.PatchTypeJSONPatch

	return admission.Response{
		AdmissionResponse: admissionv1.AdmissionResponse{
			UID:       req.UID,
			Allowed:   true,
			Patch:     []byte(patch),
			PatchType: &patchType,
		},
	}
}

func main() {
	// Set up the logger for controller-runtime
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), manager.Options{
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    8443,
			CertDir: "/etc/webhook/certs",
		}),
	})
	if err != nil {
		log.Fatalf("Failed to create manager: %v", err)
	}

	decoder := admission.NewDecoder(mgr.GetScheme())
	mutator := &PodMutator{decoder: decoder}

	mgr.GetWebhookServer().Register("/mutate", &admission.Webhook{Handler: mutator})

	fmt.Println("Starting mutating webhook server on port 8443...")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Fatalf("Failed to start manager: %v", err)
	}
}
