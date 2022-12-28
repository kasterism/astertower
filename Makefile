PROJECT=github.com/kasterism/astertower
VERSION=v1alpha1
PROJECT_APIS=${PROJECT}/pkg/apis/${VERSION}
CLIENTSET=${PROJECT}/pkg/clients/clientset
INFORMER=${PROJECT}/pkg/clients/informer
LISTER=${PROJECT}/pkg/clients/lister
HEADER=hack/boilerplate.go.txt

ifndef $(GOPATH)
	GOPATH=$(shell go env GOPATH)
	export GOPATH
endif
GOPATH_SRC=${GOPATH}/src

all: register-gen deepcopy-gen defaulter-gen openapi-gen client-gen lister-gen informer-gen

install-tools:
	go install k8s.io/code-generator/cmd/client-gen@v0.25.3
	go install k8s.io/code-generator/cmd/informer-gen@v0.25.3
	go install k8s.io/code-generator/cmd/deepcopy-gen@v0.25.3
	go install k8s.io/code-generator/cmd/lister-gen@v0.25.3
	go install k8s.io/code-generator/cmd/register-gen@v0.25.3
	go install k8s.io/code-generator/cmd/openapi-gen@v0.25.3
	go install k8s.io/code-generator/cmd/defaulter-gen@v0.25.3
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.10.0

deepcopy-gen:
	@echo ">> generating pkg/apis/deepcopy_generated.go"
	deepcopy-gen --input-dirs ${PROJECT_APIS} \
		--output-package ${PROJECT_APIS} -h ${HEADER} \
	--alsologtostderr
	mv ${GOPATH_SRC}/${PROJECT_APIS}/deepcopy_generated.go pkg/apis/${VERSION}

register-gen:
	@echo ">> generating pkg/apis/zz_generated.register.go"
	register-gen --input-dirs ${PROJECT_APIS} \
		--output-package ${PROJECT_APIS} -h ${HEADER} \
	--alsologtostderr
	mv ${GOPATH_SRC}/${PROJECT_APIS}/zz_generated.register.go pkg/apis/${VERSION}

defaulter-gen:
	@echo ">> generating pkg/apis/zz_generated.defaults.go"
	defaulter-gen --input-dirs ${PROJECT_APIS} \
		--output-package ${PROJECT_APIS} -h ${HEADER} \
	--alsologtostderr
	mv ${GOPATH_SRC}/${PROJECT_APIS}/zz_generated.defaults.go pkg/apis/${VERSION}

openapi-gen:
	@echo ">> generating pkg/apis/openapi_generated.go"
	openapi-gen --input-dirs ${PROJECT_APIS} \
		--output-package ${PROJECT_APIS} -h ${HEADER} \
	--alsologtostderr
	mv ${GOPATH_SRC}/${PROJECT_APIS}/openapi_generated.go pkg/apis/${VERSION}

client-gen:
	@echo ">> generating pkg/clients/clientset..."
	rm -rf pkg/clients/clientset
	client-gen --input-dirs ${PROJECT_APIS} \
		--clientset-name='astertower' \
		--fake-clientset=false \
		--input-base=${PROJECT} \
		--input='pkg/apis/${VERSION}' \
		--output-package ${CLIENTSET} -h ${HEADER} \
	--alsologtostderr
	mv ${GOPATH_SRC}/${CLIENTSET} pkg/clients

lister-gen:
	@echo ">> generating pkg/clients/lister..."
	rm -rf pkg/clients/lister
	lister-gen --input-dirs ${PROJECT_APIS} \
		--output-package ${LISTER} -h ${HEADER} \
	--alsologtostderr
	mv ${GOPATH_SRC}/${LISTER} pkg/clients

informer-gen:
	@echo ">> generating pkg/clients/informer..."
	rm -rf pkg/clients/informer
	informer-gen --input-dirs ${PROJECT_APIS} --versioned-clientset-package ${CLIENTSET}/astertower \
		--output-package ${INFORMER} -h ${HEADER} \
		--listers-package ${LISTER} \
	--alsologtostderr
	mv ${GOPATH_SRC}/${INFORMER} pkg/clients

crd:
	controller-gen crd:crdVersions=v1,allowDangerousTypes=true paths="./pkg/apis/..." output:crd:artifacts:config=crds

goimports:
	go install golang.org/x/tools/cmd/goimports@latest

fmt: ## Run go fmt against code.
	go fmt ./...

vet: ## Run go vet against code.
	go vet ./...

build: fmt vet ## Build manager binary.
	go build -o bin/astertower main.go

run: fmt vet ## Run code from your host.
	go run ./main.go

test:
	go test ./... -coverprofile cover.out
