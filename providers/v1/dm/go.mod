module github.com/external-secrets/external-secrets/providers/v1/dm

go 1.26.2

require (
	git.adsrv.wtf/clickadu/domain-manager v0.4.12
	github.com/external-secrets/external-secrets/apis v0.0.0
	github.com/external-secrets/external-secrets/runtime v0.0.0
	github.com/stretchr/testify v1.11.1
	k8s.io/api v0.35.2
	sigs.k8s.io/controller-runtime v0.23.3
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/external-secrets/external-secrets/apis => ../../../apis
	github.com/external-secrets/external-secrets/runtime => ../../../runtime
	github.com/cloudflare/cloudflare-go => git.adsrv.wtf/clickadu/cloudflare-go v0.106.1-0.20250129141752-9b0a92c0319a
)
