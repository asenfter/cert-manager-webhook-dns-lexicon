GO ?= $(shell which go)
OS ?= $(shell $(GO) env GOOS)
ARCH ?= $(shell $(GO) env GOARCH)

IMAGE_NAME := "webhook"
IMAGE_TAG := "latest"

OUT := $(shell pwd)/_out

KUBEBUILDER_VERSION=1.28.0

HELM_FILES := $(shell find deploy/cert-manager-webhook-dns-lexicon)

# Load .env if present
ifneq (,$(wildcard .env))
  include .env
  export
endif

test: _test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/etcd _test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/kube-apiserver _test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/kubectl | $(OUT)
	@command -v envsubst >/dev/null 2>&1 || (echo "ERROR: envsubst not found (install gettext)"; exit 1)

	# Render testdata (inject tokens from env) into $(OUT)/testdata-rendered
	rm -rf $(OUT)/testdata-rendered
	mkdir -p $(OUT)/testdata-rendered/testdata
	cp -R testdata/lexicon-hetzner $(OUT)/testdata-rendered/testdata/
	cp -R testdata/lexicon-desec $(OUT)/testdata-rendered/testdata/

	# Render secrets with provider-specific env vars
	@( export TEST_DNS_TOKEN_HETZNER="$(TEST_DNS_TOKEN_HETZNER)"; \
	  envsubst < testdata/lexicon-hetzner/secret.yaml > $(OUT)/testdata-rendered/testdata/lexicon-hetzner/secret.yaml )
	@( export TEST_DNS_TOKEN_DESEC="$(TEST_DNS_TOKEN_DESEC)"; \
	  envsubst < testdata/lexicon-desec/secret.yaml > $(OUT)/testdata-rendered/testdata/lexicon-desec/secret.yaml )


	# Run tests
	TEST_ASSET_ETCD=_test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/etcd \
	TEST_ASSET_KUBE_APISERVER=_test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/kube-apiserver \
	TEST_ASSET_KUBECTL=_test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/kubectl \
	LEXICON_TESTDATA_ROOT="$(OUT)/testdata-rendered" \
	TEST_ZONE_NAME="$(TEST_ZONE_NAME)" \
	$(GO) test -v .

_test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH).tar.gz: | _test
	curl -fsSL https://go.kubebuilder.io/test-tools/$(KUBEBUILDER_VERSION)/$(OS)/$(ARCH) -o $@

_test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/etcd _test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/kube-apiserver _test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/kubectl: _test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH).tar.gz | _test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)
	tar xfO $< kubebuilder/bin/$(notdir $@) > $@ && chmod +x $@

.PHONY: clean
clean:
	rm -r _test $(OUT)

.PHONY: build
build:
	docker build -t "$(IMAGE_NAME):$(IMAGE_TAG)" .

.PHONY: rendered-manifest.yaml
rendered-manifest.yaml: $(OUT)/rendered-manifest.yaml

$(OUT)/rendered-manifest.yaml: $(HELM_FILES) | $(OUT)
	helm template \
	    --name cert-manager-webhook-dns-lexicon \
            --set image.repository=$(IMAGE_NAME) \
            --set image.tag=$(IMAGE_TAG) \
            deploy/cert-manager-webhook-dns-lexicon > $@

_test $(OUT) _test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH):
	mkdir -p $@
