module github.com/openshift/cluster-api-provider-alibaba

go 1.16

require (
	github.com/aliyun/alibaba-cloud-sdk-go v1.61.1153
	github.com/blang/semver v3.5.1+incompatible
	github.com/go-logr/logr v0.4.0
	github.com/golang/mock v1.6.0
	github.com/onsi/gomega v1.14.0
	github.com/openshift/api v0.0.0-20211108165917-be1be0e89115
	github.com/openshift/machine-api-operator v0.2.1-0.20211102083422-ee77ca7b9fd1
	github.com/stretchr/testify v1.7.0

	// kube 1.18
	k8s.io/api v0.22.1
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v0.22.1
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.9.0
	sigs.k8s.io/controller-runtime v0.9.6
	sigs.k8s.io/controller-tools v0.6.3-0.20210916130746-94401651a6c3
	sigs.k8s.io/yaml v1.2.0
)
