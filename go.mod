module github.com/webdevops/kube-bootstrap-token-manager

go 1.15

require (
	github.com/Azure/azure-sdk-for-go v48.2.0+incompatible
	github.com/Azure/go-autorest/autorest v0.11.11
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.3
	github.com/Azure/go-autorest/autorest/date v0.3.0
	github.com/Azure/go-autorest/autorest/to v0.4.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.3.0 // indirect
	github.com/jessevdk/go-flags v1.4.0
	github.com/prometheus/client_golang v1.8.0
	github.com/sirupsen/logrus v1.7.0
	k8s.io/api v0.19.4
	k8s.io/apimachinery v0.19.4
	k8s.io/client-go v0.19.4
)
