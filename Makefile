IMAGE_TAG ?= latest
IMG ?= ai-platform-operator:$(IMAGE_TAG)

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: build
build: ## Build the operator binary
	go build -o bin/manager main.go

.PHONY: run
run: ## Run the operator locally
	go run main.go

.PHONY: docker-build
docker-build: ## Build docker image
	docker build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image
	docker push ${IMG}

.PHONY: install
install: ## Install CRDs into the cluster
	kubectl apply -f config/crd/

.PHONY: uninstall
uninstall: ## Uninstall CRDs from the cluster
	kubectl delete -f config/crd/

.PHONY: deploy
deploy: ## Deploy the operator to the cluster
	kubectl apply -f config/deployment/

.PHONY: undeploy
undeploy: ## Undeploy the operator from the cluster
	kubectl delete -f config/deployment/

.PHONY: test
test: ## Run tests
	go test ./... -coverprofile cover.out

.PHONY: fmt
fmt: ## Run go fmt
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: tidy
tidy: ## Run go mod tidy
	go mod tidy
