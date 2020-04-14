# BanzaiCloud task

Task description: https://docs.google.com/document/d/1LDMmUPPjV3NhrtGbXSfSdshPcZvv7Sft5m_-q_aVycM/edit#heading=h.gjc0w6xygrw7

TL;DR:
K8s operator which creates ingress and certificate for services with specified labels and annotations (Check test-service.yaml). Certificate is issued by Let's encrypt.

### Cert-manager setup:

kubectl apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v0.14.1/cert-manager.yaml

### Help for local test: 

kind delete cluster --name test

kind create cluster --name test

kubectl apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v0.14.1/cert-manager.yaml

kubectl apply -f test-service.yml

### Helm commands

helm install-f values.yaml customingressmanager .

### Checks

kubectl get svc

kubectl get ingress

kubectl get clusterissuer

kubectl get certificate

kubectl get secret default-secret -o=jsonpath='{.data.tls\.crt}'|base64 -d | openssl x509 -text
