package main

import (
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	faasv1alpha1 "mini-faas/api/v1alpha1"
)

func runtimeScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = faasv1alpha1.AddToScheme(scheme)
	return scheme
}
