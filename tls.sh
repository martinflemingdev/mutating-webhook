# Create a CertificateSigningRequest (CSR)
openssl req -new -newkey rsa:4096 -keyout tls.key -out tls.csr -nodes \
    -subj "/CN=system:node:mutating-webhook.default.svc/O=system:nodes" \
    -addext "subjectAltName = DNS:mutating-webhook.default.svc"


# Convert to Kubernetes CSR Format
cat <<EOF | kubectl apply -f -
apiVersion: certificates.k8s.io/v1
kind: CertificateSigningRequest
metadata:
  name: mutating-webhook.default
spec:
  signerName: kubernetes.io/kubelet-serving 
  request: $(cat tls.csr | base64 | tr -d '\n')
  usages:
  - digital signature
  - key encipherment
  - server auth  
EOF


# Approve the CSR
kubectl certificate approve mutating-webhook.default


# Retrieve the Signed Cert
kubectl get csr mutating-webhook.default -o jsonpath='{.status.certificate}' | base64 -d > tls.crt


# Recreate the TLS Secret
kubectl delete secret mutating-webhook-tls
kubectl create secret tls mutating-webhook-tls --cert=tls.crt --key=tls.key


# Update the Webhook CA Bundle
cat tls.crt | base64 | tr -d '\n'


# Copy the base64 output and update mutating-webhook.yaml:
caBundle: "PASTE_BASE64_CA_HERE"
kubectl apply -f mutating-webhook.yaml
