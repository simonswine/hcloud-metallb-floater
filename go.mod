module github.com/simonswine/hcloud-metallb-floater

go 1.13

require (
	github.com/go-logr/logr v0.1.0
	github.com/hetznercloud/hcloud-go v1.17.0
	github.com/spf13/cobra v1.0.0
	go.uber.org/zap v1.10.0
	k8s.io/api v0.17.2
	k8s.io/apimachinery v0.17.2
	k8s.io/apiserver v0.17.2
	k8s.io/client-go v0.17.2
	sigs.k8s.io/controller-runtime v0.5.2
	sigs.k8s.io/controller-tools v0.2.9 // indirect
)
