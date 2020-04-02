#BanzaiCloud feladat

Feladat lírás: https://docs.google.com/document/d/1LDMmUPPjV3NhrtGbXSfSdshPcZvv7Sft5m_-q_aVycM/edit#heading=h.gjc0w6xygrw7

TL;DR:
K8s operátor amely adott annotációval rendelkező service-ekhez automatikusan ingress-t készit valamint Let's encrypt és cert-manager használatával certificatet-t is előállít. 

Kis help a local teszteléshez:

kind delete cluster --name test

kind create cluster --name test

kubectl apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v0.14.0/cert-manager.yaml

kubectl apply -f test-service.yml
