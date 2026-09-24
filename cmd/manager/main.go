package main

import (
	"log"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	faasv1alpha1 "mini-faas/api/v1alpha1"
	"mini-faas/internal/controller"
)

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = faasv1alpha1.AddToScheme(scheme)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:  scheme,
		Metrics: metricsserver.Options{BindAddress: ":8081"},
	})
	if err != nil {
		log.Fatalf("unable to start manager: %v", err)
	}

	reconciler := &controller.FunctionReconciler{Client: mgr.GetClient()}
	if err := reconciler.SetupWithManager(mgr); err != nil {
		log.Fatalf("unable to set up FunctionReconciler: %v", err)
	}

	log.Println("starting function controller manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Fatalf("manager exited with error: %v", err)
	}
}
