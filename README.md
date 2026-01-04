![GitHub release](https://img.shields.io/github/v/release/asenfter/cert-manager-webhook-dns-lexicon)


# ACME webhook for dns-lexicon

This project implements an ACME DNS‑01 webhook for cert-manager based on
[dns-lexicon](https://github.com/dns-lexicon/dns-lexicon). It allows you to obtain
certificates using any DNS provider supported by dns-lexicon (e.g. Hetzner,
deSEC, …) without cert-manager needing a built‑in integration for that
provider.

The webhook is exposed as a Kubernetes APIService and is used by cert-manager through the *webhook solver*.

---

## dns-lexicon version

The dns-lexicon version used by this project is defined in the Dockerfile:

```Dockerfile
FROM ghcr.io/dns-lexicon/dns-lexicon:x.y.z
```

---

## Prerequisites for local development

- Kubernetes cluster with cert-manager installed
- `kubectl` configured for the cluster
- Docker/container runtime to build an image (optional if you use a published image)
- Helm (for installing the chart)

---

## Installation (Helm)

The Helm chart already sets the `groupName`, see
[deploy/cert-manager-webhook-dns-lexicon/values.yaml](deploy/cert-manager-webhook-dns-lexicon/values.yaml).

```bash
# Optional: build and push your own image
docker build -t docker.senfter.net/cert-manager-webhook-dns-lexicon:x.y.z .
docker push docker.senfter.net/cert-manager-webhook-dns-lexicon:x.y.z

# Install webhook in namespace cert-manager
helm install \
  --namespace cert-manager \
  cert-manager-webhook-dns-lexicon \
  deploy/cert-manager-webhook-dns-lexicon
```

Check status:

```bash
kubectl get pods -n cert-manager
kubectl -n cert-manager logs deploy/cert-manager-webhook-dns-lexicon
```

Uninstall:

```bash
helm uninstall --namespace cert-manager cert-manager-webhook-dns-lexicon
```

Open a shell in the pod (for debugging, etc.):

```bash
kubectl -n cert-manager exec -it deploy/cert-manager-webhook-dns-lexicon -- /bin/sh
```

---

## Configuration in cert-manager

The webhook is used via an `Issuer` or `ClusterIssuer` object with
`dns01.webhook`. Example for desec (adapt similarly for other providers):

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-staging
  labels:
    app.kubernetes.io/part-of: cert-manager
    app.kubernetes.io/managed-by: argocd
spec:
  acme:
    email: asenfter@gmail.com
    server: https://acme-staging-v02.api.letsencrypt.org/directory
    privateKeySecretRef:
      name: letsencrypt-staging-secret
    solvers:
      - dns01:
          webhook:
            groupName: dns-lexicon.cert-manager-webhook.io
            solverName: dns-lexicon
            config:
              provider: desec
              zoneName: example.com.
              ttl: 3600
              authTokenSecretRef:
                name: dns-lexicon-desec-token
                key: token
```

The referenced Secret may look like this:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name:  dns-lexicon-desec-token
  namespace: cert-manager
type: Opaque
data:
  token: <base64-ENCODED-TOKEN>
```

ArgoCD Application YAML may look like this:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: cert-manager-webhook-dns-lexicon
  namespace: argocd
  annotations:
    argocd.argoproj.io/sync-wave: "1"
  labels:
    app.kubernetes.io/name: cert-manager-webhook-dns-lexicon
    app.kubernetes.io/part-of: cert-manager
spec:
  project: default
  source:
    repoURL: https://github.com/asenfter/cert-manager-webhook-dns-lexicon.git
    targetRevision: "0.12.0"
    path: deploy/cert-manager-webhook-dns-lexicon
    helm:
      values: |
        groupName: dns-lexicon.cert-manager-webhook.io
        certManager:
          namespace: cert-manager
          serviceAccountName: cert-manager
        image:
          repository: ghcr.io/asenfter/cert-manager-webhook-dns-lexicon
          pullPolicy: IfNotPresent
        replicaCount: 1
  destination:
    server: https://kubernetes.default.svc
    namespace: cert-manager
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
      - ServerSideApply=true
```

---

## Local development

Tidy dependencies and build:

```bash
go mod tidy
go build ./...
```

The main implementation of the solver lives in `main.go`. The solver name is
`dns-lexicon` and is referenced via `solverName` in the Issuer.

---

## Tests

There are conformance‑style tests against real providers (e.g. deSEC). To run
them:

```bash
make test

source .env    # if you store env vars there

TEST_ASSET_ETCD=_test/kubebuilder-1.28.0-linux-arm64/etcd \
TEST_ASSET_KUBE_APISERVER=_test/kubebuilder-1.28.0-linux-arm64/kube-apiserver \
TEST_ASSET_KUBECTL=_test/kubebuilder-1.28.0-linux-arm64/kubectl \
go test -v -run TestRunsSuiteDeSEC ./...
```

---

## How it works (short overview)

- `GROUP_NAME` is taken from the environment and must match the `groupName`
  configured in the Helm values.
- The solver reads its configuration (`provider`, `zoneName`,
  `authTokenSecretRef`, `ttl`) from the `config` block in the Issuer.
- During `Present`, dns-lexicon is used to create or update TXT records under
  `_acme-challenge.<record_name>` at the configured DNS provider.
- `CleanUp` removes the corresponding TXT records after validation has
  finished.
