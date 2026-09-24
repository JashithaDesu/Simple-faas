// The gateway is the front door of the whole platform. It answers one
// question on every request: "is there a warm pod for this function?"
// If yes, forward immediately (warm path). If no, trigger a scale-up,
// wait for the pod to become Ready, then forward (cold-start path).
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	faasv1alpha1 "mini-faas/api/v1alpha1"
)

var k8sClient client.Client

func main() {
	cfg := ctrl.GetConfigOrDie()
	scheme := runtimeScheme()
	c, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		log.Fatalf("failed to build k8s client: %v", err)
	}
	k8sClient = c

	http.HandleFunc("/invoke/", handleInvoke)
	log.Println("gateway listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleInvoke(w http.ResponseWriter, r *http.Request) {
	fnName := r.URL.Path[len("/invoke/"):]
	if fnName == "" {
		http.Error(w, "missing function name, expected /invoke/<name>", http.StatusBadRequest)
		return
	}
	ctx := r.Context()

	var fn faasv1alpha1.Function
	key := types.NamespacedName{Name: fnName, Namespace: "default"}
	if err := k8sClient.Get(ctx, key, &fn); err != nil {
		http.Error(w, fmt.Sprintf("function %q not found", fnName), http.StatusNotFound)
		return
	}

	// Mark this function as active NOW, before anything else, so the
	// controller's idle-timeout check never races against us scaling
	// it back down while we're mid-cold-start.
	now := metav1.Now()
	fn.Status.LastRequestTime = &now
	if err := k8sClient.Status().Update(ctx, &fn); err != nil {
		log.Printf("warning: failed to bump LastRequestTime for %s: %v", fnName, err)
	}

	if fn.Status.Phase != faasv1alpha1.PhaseWarm {
		// COLD PATH: not warm yet. Bump min replicas so the controller's
		// next reconcile scales it up, then poll until it's ready.
		if err := triggerColdStart(ctx, &fn); err != nil {
			http.Error(w, "failed to trigger cold start", http.StatusInternalServerError)
			return
		}
		if err := waitUntilWarm(ctx, fnName, 15*time.Second); err != nil {
			http.Error(w, "function did not become ready in time", http.StatusGatewayTimeout)
			return
		}
	}

	forwardToFunction(w, r, fnName)
}

// triggerColdStart nudges the Function's spec so the controller's next
// reconcile loop scales the backing Deployment up from zero.
func triggerColdStart(ctx context.Context, fn *faasv1alpha1.Function) error {
	if fn.Spec.MinReplicas < 1 {
		patch := client.MergeFrom(fn.DeepCopy())
		fn.Spec.MinReplicas = 1 // temporary nudge; controller can lower it again after idle
		return k8sClient.Patch(ctx, fn, patch)
	}
	return nil
}

// waitUntilWarm polls the Function's status until the controller reports
// it Warm, or the timeout expires. A production version would use a
// watch instead of polling — left as a documented next step, see README.
func waitUntilWarm(ctx context.Context, fnName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var fn faasv1alpha1.Function
		key := types.NamespacedName{Name: fnName, Namespace: "default"}
		if err := k8sClient.Get(ctx, key, &fn); err == nil && fn.Status.Phase == faasv1alpha1.PhaseWarm {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for %s to become warm", fnName)
}

// forwardToFunction reverse-proxies the request to the function's
// in-cluster Service.
func forwardToFunction(w http.ResponseWriter, r *http.Request, fnName string) {
	target, _ := url.Parse(fmt.Sprintf("http://%s.default.svc.cluster.local:8080", fnName))
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ServeHTTP(w, r)
}
