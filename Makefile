APP := hcloud-metallb-floater
IMAGE := simonswine/$(APP)
TAG := canary

CONTROLLER_GEN := go run sigs.k8s.io/controller-tools/cmd/controller-gen

gobuild: ## Builds a static binary
	CGO_ENABLED=0 GOOS=linux go build -o $(APP) .

image: ## Build docker image
	docker build -t $(IMAGE):$(TAG) .

push: image ## Push docker image
	docker push $(IMAGE):$(TAG)

.PHONY: manifests
manifests: ## Update generated manifests
	$(CONTROLLER_GEN) rbac:roleName=controller paths=./cmd/...
